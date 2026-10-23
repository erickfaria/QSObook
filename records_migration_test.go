package main

import (
	"context"
	"database/sql"
	"testing"
)

func recordsVersionFour(t *testing.T) string {
	t.Helper()
	path := legacyDatabase(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, schema := range []string{participantIdentitySchema, roundsSchema, mapSchema} {
		if _, err = db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec("UPDATE app_settings SET map_enabled=1;INSERT INTO city_coordinates VALUES('Exampletown','XX','Exampleland',1,2,'2026-10-23T12:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestRecordsMigrationPreservesBetaThreeNotebook(t *testing.T) {
	ctx := context.Background()
	s, err := openStore(ctx, recordsVersionFour(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	c, err := s.contact(ctx, 11)
	if err != nil || c.StationID != 7 || c.Notes != "Contact note" || c.Band != "" || c.RoundName != "Rodada anterior" {
		t.Fatalf("contact changed: %+v %v", c, err)
	}
	participants, err := s.participants(ctx)
	if err != nil || len(participants) != 1 || participants[0].ID != 23 || participants[0].QSOID.Int64 != 11 {
		t.Fatal("participant links lost", err)
	}
	home, err := s.homeLocation(ctx)
	if err != nil || home.City != "" || home.Country != "" || home.AutoGeocode || !home.MapEnabled {
		t.Fatalf("privacy defaults changed: %+v %v", home, err)
	}
	var latitude float64
	if err = s.db.QueryRow("SELECT latitude FROM city_coordinates").Scan(&latitude); err != nil || latitude != 1 {
		t.Fatal("geographic cache changed", err)
	}
	var integrity string
	if err = s.db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("integrity", err)
	}
	rows, err := s.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() || rows.Err() != nil {
		t.Fatal("foreign keys")
	}
}
func TestRecordsMigrationFailureRollsBackAllNewColumns(t *testing.T) {
	path := recordsVersionFour(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE INDEX qsos_submission ON qsos(mode)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := openStore(context.Background(), path); err == nil {
		s.db.Close()
		t.Fatal("conflicting schema accepted")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, count int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 4 {
		t.Fatal("schema version changed", err)
	}
	if err = db.QueryRow("SELECT count(*) FROM pragma_table_info('qsos') WHERE name='band'").Scan(&count); err != nil || count != 0 {
		t.Fatal("partial migration committed", err)
	}
	if err = db.QueryRow("SELECT count(*) FROM qsos WHERE id=11 AND notes='Contact note'").Scan(&count); err != nil || count != 1 {
		t.Fatal("history changed", err)
	}
}
