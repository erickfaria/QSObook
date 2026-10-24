package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func legacyDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "version-one.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(initialSchema); err != nil {
		t.Fatal(err)
	}
	const fixture = `
		INSERT INTO stations(id,callsign,name,city,state,country,notes)
		 VALUES(7,'ZZ0MIG','Example operator','Exampletown','XX','Exampleland','Permanent note');
		INSERT INTO qsos(id,station_id,callsign,name,city,state,country,mode,notes,occurred_at)
		 SELECT 11,id,callsign,name,city,state,country,'DMR','Contact note','2026-10-23T12:00:00.000000000Z'
		 FROM stations WHERE id=7;
		INSERT INTO participants(id,station_id,mode,notes,qso_id)
		 VALUES(23,7,'DMR','Round note',11);`
	if _, err := db.Exec(fixture); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParticipantIdentityMigrationPreservesData(t *testing.T) {
	ctx := context.Background()
	path := legacyDatabase(t)
	s, err := openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s != nil {
			s.db.Close()
		}
	}()
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema version: %d %v", version, err)
	}
	st, err := s.station(ctx, 7)
	want := Station{ID: 7, Callsign: "ZZ0MIG", Name: "Example operator", City: "Exampletown", State: "XX", Country: "Exampleland", Notes: "Permanent note"}
	if err != nil || st != want {
		t.Fatalf("operator changed during migration: %+v %v", st, err)
	}
	c, err := s.contact(ctx, 11)
	if err != nil || c.StationID != 7 || c.Callsign != want.Callsign || c.Name != want.Name || c.City != want.City || c.State != want.State || c.Country != want.Country || c.Mode != "DMR" || c.Notes != "Contact note" || c.OccurredAt.Format("2006-01-02T15:04:05Z") != "2026-10-23T12:00:00Z" {
		t.Fatalf("contact changed during migration: %+v %v", c, err)
	}
	participants, err := s.participants(ctx)
	if err != nil || len(participants) != 1 || participants[0].ID != 23 || participants[0].Station != want || participants[0].Mode != "DMR" || participants[0].Notes != "Round note" || !participants[0].QSOID.Valid || participants[0].QSOID.Int64 != 11 {
		t.Fatalf("participation changed during migration: %+v %v", participants, err)
	}
	var integrity string
	if err := s.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity check: %s %v", integrity, err)
	}
	rows, err := s.db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() || rows.Err() != nil {
		rows.Close()
		t.Fatal("foreign key check failed")
	}
	rows.Close()
	// The sequence must remain above deleted IDs after reopening the database.
	lastID := int64(23)
	for _, callsign := range []string{"ZZ0NEW", "ZZ1NEW"} {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM participants WHERE id=?", lastID); err != nil {
			t.Fatal(err)
		}
		if err := s.db.Close(); err != nil {
			t.Fatal(err)
		}
		s, err = openStore(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		if err := saveTestParticipant(t, s, ctx, Station{Callsign: callsign}, "FM", "New participation"); err != nil {
			t.Fatal(err)
		}
		participants, err = s.participants(ctx)
		if err != nil || len(participants) != 1 || participants[0].ID <= lastID {
			t.Fatalf("reused participation ID after restart: %+v %v", participants, err)
		}
		lastID = participants[0].ID
	}
	contacts, err := s.contacts(ctx, "")
	if err != nil || len(contacts) != 1 || contacts[0].ID != 11 {
		t.Fatalf("history damaged: %+v %v", contacts, err)
	}
}

func TestParticipantIdentityMigrationRollsBackOnFailure(t *testing.T) {
	path := legacyDatabase(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an invalid legacy reference. The migration must fail without
	// discarding the old table, records or schema version.
	if _, err := db.Exec("UPDATE participants SET station_id=9999 WHERE id=23"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if s, err := openStore(context.Background(), path); err == nil {
		s.db.Close()
		t.Fatal("accepted invalid legacy reference")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, stationID, temporaryTables, stationCount, contactCount int
	for query, target := range map[string]*int{
		"PRAGMA user_version":                                               &version,
		"SELECT station_id FROM participants WHERE id=23":                   &stationID,
		"SELECT count(*) FROM sqlite_master WHERE name='participants_next'": &temporaryTables,
		"SELECT count(*) FROM stations":                                     &stationCount,
		"SELECT count(*) FROM qsos":                                         &contactCount,
	} {
		if err := db.QueryRow(query).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if version != 1 || stationID != 9999 || temporaryTables != 0 || stationCount != 1 || contactCount != 1 {
		t.Fatalf("migration did not roll back: version=%d reference=%d temporary=%d stations=%d contacts=%d", version, stationID, temporaryTables, stationCount, contactCount)
	}
}

func TestWorkspaceUpgradePreservesVersionSixNotebook(t *testing.T) {
	ctx := context.Background()
	path := legacyDatabase(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{participantIdentitySchema, roundsSchema, mapSchema, recordsSchema, dataToolsSchema} {
		if _, err = db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec("UPDATE participants SET active=0; UPDATE qsos SET deleted_at='2026-10-23T13:00:00Z'"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	var name, edition, remembered string
	if err = s.db.QueryRow("SELECT name,edition FROM rounds WHERE id=1").Scan(&name, &edition); err != nil || name != "Rodada anterior" || edition != "" {
		t.Fatal(name, edition, err)
	}
	if err = s.db.QueryRow("SELECT last_round_name||last_round_edition FROM app_settings").Scan(&remembered); err != nil || remembered != "" {
		t.Fatal(remembered, err)
	}
	var participant, qso, active int
	if err = s.db.QueryRow("SELECT id,qso_id,active FROM participants").Scan(&participant, &qso, &active); err != nil || participant != 23 || qso != 11 || active != 0 {
		t.Fatal(participant, qso, active, err)
	}
	if err = s.moveTrash(ctx, "contact", 11, true); err != nil {
		t.Fatal(err)
	}
	contact, err := s.contact(ctx, 11)
	if err != nil || contact.Notes != "Contact note" || contact.Name != "Example operator" {
		t.Fatal(contact, err)
	}
	if err = s.endRound(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.startRound(ctx, "Example net", 1, "12"); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "workspace.db")
	if err = s.backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if err = validateBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("UPDATE app_settings SET last_round_edition='99'"); err != nil {
		t.Fatal(err)
	}
	if err = s.restore(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT last_round_name,last_round_edition FROM app_settings").Scan(&remembered, &edition); err != nil || remembered != "Example net" || edition != "12" {
		t.Fatal(remembered, edition, err)
	}
}
