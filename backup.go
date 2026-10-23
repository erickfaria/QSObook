package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var notebookTables = []string{"stations", "rounds", "qsos", "participants", "app_settings", "city_coordinates", "sqlite_sequence"}
var errBackupInvalid = errors.New("Backup inválido ou incompatível. O banco atual não foi alterado.")

type NotebookSummary struct{ Stations, Contacts, Trashed, Rounds int64 }

func (s *Store) summary(ctx context.Context) (NotebookSummary, error) {
	var n NotebookSummary
	err := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM stations WHERE deleted_at IS NULL),(SELECT count(*) FROM qsos WHERE deleted_at IS NULL),(SELECT count(*) FROM stations WHERE deleted_at IS NOT NULL)+(SELECT count(*) FROM qsos WHERE deleted_at IS NOT NULL),(SELECT count(*) FROM rounds)`).Scan(&n.Stations, &n.Contacts, &n.Trashed, &n.Rounds)
	return n, err
}

// VACUUM INTO produces a consistent SQLite snapshot, including with a live WAL.
// The destination must be a new file inside a private, application-owned folder.
func (s *Store) backup(ctx context.Context, destination string) error {
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("backup destination must not exist")
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return err
	}
	if err := os.Chmod(destination, 0600); err != nil {
		return err
	}
	f, err := os.OpenFile(destination, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func schemaSignature(ctx context.Context, db *sql.DB) (string, error) {
	rows, err := db.QueryContext(ctx, `SELECT type,name,tbl_name,sql FROM sqlite_schema WHERE substr(name,1,7)<>'sqlite_' ORDER BY type,name`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var typ, name, table, statement string
		if err := rows.Scan(&typ, &name, &table, &statement); err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "%s:%s:%s:%s\n", typ, name, table, strings.Join(strings.Fields(statement), " "))
	}
	return out.String(), rows.Err()
}
func expectedSignature(ctx context.Context, version int) (string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return "", err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schemas := []string{initialSchema, participantIdentitySchema, roundsSchema, mapSchema, recordsSchema, dataToolsSchema}
	for _, schema := range schemas[:version] {
		if _, err = db.ExecContext(ctx, schema); err != nil {
			return "", err
		}
	}
	return schemaSignature(ctx, db)
}

// Inspect without executing uploaded SQL, before allowing known migrations.
// Only exact application schemas 5 and 6 are supported by the restore UI.
func validateBackup(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?mode=ro&_pragma=query_only(1)&_pragma=trusted_schema(0)")
	if err != nil {
		return errBackupInvalid
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var version int
	if err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version < 5 || version > schemaVersion {
		return errBackupInvalid
	}
	expected, err := expectedSignature(ctx, version)
	if err != nil {
		return err
	}
	actual, err := schemaSignature(ctx, db)
	if err != nil || actual != expected {
		return errBackupInvalid
	}
	var integrity string
	if err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return errBackupInvalid
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return errBackupInvalid
	}
	bad := rows.Next() || rows.Err() != nil
	rows.Close()
	if bad {
		return errBackupInvalid
	}
	// Validate application fields too: a valid SQLite file can contain invalid QSOs.
	rows, err = db.QueryContext(ctx, "SELECT "+stationColumns+" FROM stations")
	if err != nil {
		return errBackupInvalid
	}
	for rows.Next() {
		st, e := scanStation(rows)
		original := st.Callsign
		if e != nil || st.validate() != nil || st.Callsign != original {
			rows.Close()
			return errBackupInvalid
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errBackupInvalid
	}
	rows, err = db.QueryContext(ctx, "SELECT "+contactColumns+" FROM qsos")
	if err != nil {
		return errBackupInvalid
	}
	for rows.Next() {
		c, e := scanContact(rows)
		if e != nil || c.Station.validate() != nil || validateMode(c.Mode) != nil || validateRecordDetails(c.Band, c.RoundName) != nil {
			rows.Close()
			return errBackupInvalid
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errBackupInvalid
	}
	var settings int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM app_settings WHERE id=1").Scan(&settings); err != nil || settings != 1 {
		return errBackupInvalid
	}
	return nil
}

// Replace rows in the existing database, never swap an open database file.
// Deferred foreign keys permit an atomic replacement of all related tables.
func (s *Store) restore(ctx context.Context, path string) error {
	if err := validateBackup(ctx, path); err != nil {
		return err
	}
	source, err := openStore(ctx, path)
	if err != nil {
		return err
	}
	source.db.Close()
	if err := validateBackup(ctx, path); err != nil {
		return err
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "ATTACH DATABASE ? AS incoming", sqliteFileURI(path)+"?mode=ro"); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "DETACH DATABASE incoming")
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); err != nil {
		return err
	}
	for _, table := range notebookTables {
		if _, err = tx.ExecContext(ctx, "DELETE FROM main."+table); err != nil {
			return err
		}
	}
	for _, table := range notebookTables {
		if table == "sqlite_sequence" {
			if _, err = tx.ExecContext(ctx, "DELETE FROM main.sqlite_sequence"); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO main."+table+" SELECT * FROM incoming."+table); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, "PRAGMA main.foreign_key_check")
	if err != nil {
		return err
	}
	bad := rows.Next() || rows.Err() != nil
	rows.Close()
	if bad {
		return errBackupInvalid
	}
	return tx.Commit()
}

func (s *Store) dataDirectory() string {
	if s.path != "" {
		return filepath.Dir(s.path)
	}
	return os.TempDir()
}
