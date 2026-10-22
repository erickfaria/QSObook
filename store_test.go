package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := openStore(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}
func TestPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "local.db")
	s, err := openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.saveStation(ctx, Station{Callsign: " py0test ", Name: "Estação fictícia", City: "Cidade exemplo"})
	if err != nil {
		t.Fatal(err)
	}
	qid, err := s.record(ctx, id, "FM", "Primeiro contato")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.addParticipant(ctx, id, "CW", "Rodada"); err != nil {
		t.Fatal(err)
	}
	if err = s.db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	st, err := s.lookup(ctx, "py0test")
	if err != nil || st.ID != id {
		t.Fatalf("lookup: %+v %v", st, err)
	}
	c, err := s.contact(ctx, qid)
	if err != nil || c.Mode != "FM" || c.OccurredAt.Location().String() != "UTC" {
		t.Fatalf("contact: %+v %v", c, err)
	}
	st.Name = "Nome corrigido"
	if _, err = s.saveStation(ctx, st); err != nil {
		t.Fatal(err)
	}
	old, _ := s.contact(ctx, qid)
	if old.Name == st.Name {
		t.Fatal("history changed with station")
	}
	c.Notes = "Correção"
	c.Name = "Nome no contato"
	if err = s.updateContact(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.contacts(ctx, "nome no contato")
	if err != nil || len(got) != 1 || got[0].Notes != "Correção" {
		t.Fatalf("search: %v %v", got, err)
	}
	participants, err := s.participants(ctx)
	if err != nil || len(participants) != 1 {
		t.Fatalf("participants: %v %v", participants, err)
	}
	for i := 0; i < 2; i++ {
		if err = s.recordParticipant(ctx, participants[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := s.contacts(ctx, "")
	if len(all) != 2 {
		t.Fatalf("duplicate registration: %d", len(all))
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
}
func TestValidationAndIntegrity(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, v := range []string{"", "bad space", "<script>", "ábc"} {
		if _, err := s.saveStation(ctx, Station{Callsign: v}); err == nil {
			t.Errorf("accepted %q", v)
		}
	}
	id, err := s.saveStation(ctx, Station{Callsign: "ZZ0TEST"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.record(ctx, id, "", ""); err == nil {
		t.Error("accepted empty mode")
	}
	if _, err = s.record(ctx, 9999, "FM", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing station: %v", err)
	}
	if err = s.addParticipant(ctx, 9999, "FM", ""); err == nil {
		t.Error("foreign key not enforced")
	}
	if _, err = s.saveStation(ctx, Station{ID: 9999, Callsign: "ZZ0TEST"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing update: %v", err)
	}
	for _, q := range []string{"zz0", "' OR 1=1 --"} {
		list, err := s.stations(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if q[0] == '\'' {
			want = 0
		}
		if len(list) != want {
			t.Errorf("search %q: %d", q, len(list))
		}
	}
}

func TestMigrationVersionAndMissingRecords(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.migrate(ctx); err != nil {
		t.Fatal("migration is not repeatable", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(ctx); err == nil {
		t.Fatal("accepted future schema")
	}
	if err := s.recordParticipant(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing participant: %v", err)
	}
	if err := s.updateContact(ctx, Contact{ID: 999, Station: Station{Callsign: "ZZ0TEST"}, Mode: "FM"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing contact: %v", err)
	}
}
