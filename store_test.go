package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestParticipantConfirmationAndIndependentNotes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	original := Station{Callsign: "ZZ0ROUND", Name: "Operador exemplo", City: "Cidade exemplo", State: "UF", Country: "Brasil", Notes: "Observação permanente"}
	id, err := s.saveStation(ctx, original)
	if err != nil {
		t.Fatal(err)
	}
	original.ID = id
	oldID, err := s.record(ctx, id, "CW", "Contato anterior")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.saveParticipant(ctx, Station{Callsign: " zz0round "}, "DMR", "Conversa atual"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.station(ctx, id)
	if err != nil || got != original {
		t.Fatalf("registration changed: %+v %v", got, err)
	}
	list, err := s.participants(ctx)
	if err != nil || len(list) != 1 || list[0].QSOID.Valid {
		t.Fatalf("participants: %+v %v", list, err)
	}
	all, err := s.contacts(ctx, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("addition recorded a QSO: %+v %v", all, err)
	}
	// Another addition must not erase the current conversation or reset its mode.
	if err := s.saveParticipant(ctx, Station{Callsign: original.Callsign}, "FM", ""); err != nil {
		t.Fatal(err)
	}
	list, err = s.participants(ctx)
	if err != nil || list[0].Mode != "DMR" || list[0].Notes != "Conversa atual" {
		t.Fatalf("repeated addition changed participation: %+v %v", list, err)
	}
	// Explicit editing can clear conversation notes, while empty registration
	// fields still preserve the operator's current data.
	if err := s.saveParticipant(ctx, Station{ID: id, Callsign: original.Callsign, Name: "Nome corrigido"}, "SSB", ""); err != nil {
		t.Fatal(err)
	}
	got, err = s.station(ctx, id)
	if err != nil || got.Name != "Nome corrigido" || got.City != original.City || got.Notes != original.Notes {
		t.Fatalf("edit damaged registration: %+v %v", got, err)
	}
	list, err = s.participants(ctx)
	if err != nil || list[0].Notes != "" {
		t.Fatalf("cannot clear conversation notes: %+v %v", list, err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- s.recordParticipant(ctx, list[0].ID)
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	all, err = s.contacts(ctx, "")
	if err != nil || len(all) != 2 || all[0].Name != "Nome corrigido" || all[0].Mode != "SSB" || all[0].Notes != "" {
		t.Fatalf("confirmation: %+v %v", all, err)
	}
	old, err := s.contact(ctx, oldID)
	if err != nil || old.Name != original.Name || old.Notes != "Contato anterior" {
		t.Fatalf("history changed: %+v %v", old, err)
	}
	if err := s.saveParticipant(ctx, Station{Callsign: original.Callsign}, "FM", "Reenvio"); err != nil {
		t.Fatal(err)
	}
	if err := s.recordParticipant(ctx, list[0].ID); err != nil {
		t.Fatal(err)
	}
	all, err = s.contacts(ctx, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("reopened confirmed QSO: %+v %v", all, err)
	}
}

func TestConversationTransactionsRollback(t *testing.T) {
	for _, operation := range []string{"participant", "contact"} {
		t.Run(operation, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			id, err := s.saveStation(ctx, Station{Callsign: "ZZ0KEEP", Name: "Original", Notes: "Permanente"})
			if err != nil {
				t.Fatal(err)
			}
			table := "participants"
			if operation == "contact" {
				table = "qsos"
			}
			if _, err := s.db.ExecContext(ctx, "CREATE TRIGGER fixture_failure BEFORE INSERT ON "+table+" BEGIN SELECT RAISE(ABORT,'synthetic failure'); END"); err != nil {
				t.Fatal(err)
			}
			for _, callsign := range []string{"ZZ0KEEP", "ZZ0NEW"} {
				st := Station{Callsign: callsign, Name: "Changed"}
				if operation == "participant" {
					err = s.saveParticipant(ctx, st, "FM", "Conversa")
				} else {
					_, err = s.createContact(ctx, st, "FM", "Conversa")
				}
				if err == nil {
					t.Fatal("expected transaction failure")
				}
			}
			got, err := s.station(ctx, id)
			if err != nil || got.Name != "Original" || got.Notes != "Permanente" {
				t.Fatalf("partial registration update: %+v %v", got, err)
			}
			if _, err := s.lookup(ctx, "ZZ0NEW"); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("partial registration insert: %v", err)
			}
		})
	}
}

func TestManualContactPreservesRegistration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	id, err := s.saveStation(ctx, Station{Callsign: "ZZ0MANUAL", Name: "Operador", City: "Cidade", Notes: "Permanente"})
	if err != nil {
		t.Fatal(err)
	}
	qid, err := s.createContact(ctx, Station{Callsign: "zz0manual", Notes: "Nunca copiar ao cadastro"}, "DMR", "Somente neste contato")
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.station(ctx, id)
	if err != nil || st.Name != "Operador" || st.City != "Cidade" || st.Notes != "Permanente" {
		t.Fatalf("registration: %+v %v", st, err)
	}
	c, err := s.contact(ctx, qid)
	if err != nil || c.Notes != "Somente neste contato" || c.Name != st.Name {
		t.Fatalf("contact: %+v %v", c, err)
	}
	for _, notes := range []string{strings.Repeat("á", 4001), "invalid\x00notes", string([]byte{0xff})} {
		if err := s.saveParticipant(ctx, Station{Callsign: "ZZ0INVALID"}, "FM", notes); err == nil {
			t.Error("accepted invalid notes")
		}
	}
	if _, err := s.lookup(ctx, "ZZ0INVALID"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("invalid submission persisted: %v", err)
	}
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
