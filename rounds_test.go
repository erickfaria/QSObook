package main

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRoundLifecycleAndStaleActions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.currentRound(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("fresh database has a round: %v", err)
	}
	if err := s.saveParticipant(ctx, 1, 0, Station{Callsign: "ZZ0EARLY"}, "FM", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("added before start: %v", err)
	}
	id, err := s.startRound(ctx, "Evening round", 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, e := s.startRound(ctx, "Repeated click", 0)
			if e != nil || got != id {
				t.Errorf("start replay: %d %v", got, e)
			}
		}()
	}
	wg.Wait()
	st := Station{Callsign: "ZZ0ROUND", Name: "Example operator", City: "Exampletown"}
	if err = s.saveParticipant(ctx, id, 0, st, "DMR", "Conversation"); err != nil {
		t.Fatal(err)
	}
	list, err := s.participants(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	p := list[0]
	for i := 0; i < 2; i++ {
		if err = s.setParticipantActive(ctx, p.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	list, _ = s.participants(ctx)
	contacts, _ := s.contacts(ctx, "")
	if len(list) != 1 || list[0].Active || list[0].Notes != "Conversation" || len(contacts) != 0 {
		t.Fatalf("QRT damaged records or confirmed a contact: %+v %+v", list, contacts)
	}
	if err = s.setParticipantActive(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = s.recordParticipant(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.endRound(ctx, id); err != nil {
		t.Fatal(err)
	}
	round, _ := s.round(ctx, id)
	ended := round.EndedAt
	if err = s.endRound(ctx, id); err != nil {
		t.Fatal(err)
	}
	round, _ = s.round(ctx, id)
	if round.EndedAt != ended {
		t.Fatal("repeated close changed the end time")
	}
	if _, err = s.startRound(ctx, "Stale start", 0); !errors.Is(err, errRoundChanged) {
		t.Fatalf("stale start created a round: %v", err)
	}
	next, err := s.startRound(ctx, "Next round", id)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.saveParticipant(ctx, next, 0, st, "CW", "New conversation"); err != nil {
		t.Fatal(err)
	}
	for _, action := range []func() error{
		func() error {
			return s.saveParticipant(ctx, id, p.ID, Station{Callsign: st.Callsign, Name: "Stale overwrite"}, "AM", "")
		},
		func() error { return s.saveParticipant(ctx, next, p.ID, st, "AM", "") },
		func() error { return s.recordParticipant(ctx, p.ID) },
		func() error { return s.setParticipantActive(ctx, p.ID, false) },
		func() error { return s.removeParticipant(ctx, p.ID) },
	} {
		if err := action(); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("closed-round action accepted: %v", err)
		}
	}
	if err = s.endRound(ctx, id); err != nil {
		t.Fatal(err)
	}
	current, _ := s.currentRound(ctx)
	if current.ID != next {
		t.Fatal("old close ended the new round")
	}
	old, _ := s.participantsInRound(ctx, id)
	currentList, _ := s.participants(ctx)
	contacts, _ = s.contacts(ctx, "")
	if len(old) != 1 || !old[0].QSOID.Valid || len(currentList) != 1 || currentList[0].ID == p.ID || currentList[0].QSOID.Valid || len(contacts) != 1 || contacts[0].Notes != "Conversation" {
		t.Fatalf("rounds/history mixed: old=%+v new=%+v contacts=%+v", old, currentList, contacts)
	}
	if err = s.recordParticipant(ctx, currentList[0].ID); err != nil {
		t.Fatal(err)
	}
	contacts, _ = s.contacts(ctx, "")
	stations, _ := s.stations(ctx, "")
	if len(contacts) != 2 || len(stations) != 1 {
		t.Fatal("new round did not reuse registration and create an independent contact")
	}
}

func TestRemovedParticipantEditCannotReachReaddedOperator(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := saveTestParticipant(t, s, ctx, Station{Callsign: "ZZ0SAME", Name: "Original"}, "FM", ""); err != nil {
		t.Fatal(err)
	}
	old, _ := s.participants(ctx)
	if err := s.removeParticipant(ctx, old[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.saveParticipant(ctx, old[0].RoundID, 0, Station{Callsign: "ZZ0SAME"}, "DMR", "New"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveParticipant(ctx, old[0].RoundID, old[0].ID, Station{Callsign: "ZZ0SAME", Name: "Stale"}, "CW", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale edit: %v", err)
	}
	st, _ := s.lookup(ctx, "ZZ0SAME")
	current, _ := s.participants(ctx)
	if st.Name != "Original" || len(current) != 1 || current[0].Mode != "DMR" || current[0].Notes != "New" {
		t.Fatal("old edit changed a new participation")
	}
}

func TestRoundMigrationRetainsSequenceWhenOldListIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-two.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{initialSchema, participantIdentitySchema, `INSERT INTO stations(id,callsign) VALUES(1,'ZZ0OLD'); INSERT INTO participants(id,station_id,mode,notes) VALUES(900,1,'FM','Old'); DELETE FROM participants;`} {
		if _, err = db.Exec(script); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	db.Close()
	s, err := openStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	if err = saveTestParticipant(t, s, context.Background(), Station{Callsign: "ZZ0NEW"}, "FM", ""); err != nil {
		t.Fatal(err)
	}
	list, err := s.participants(context.Background())
	if err != nil || len(list) != 1 || list[0].ID <= 900 {
		t.Fatalf("lost high-water mark: %+v %v", list, err)
	}
}

func TestHTTPRoundLifecycle(t *testing.T) {
	a, err := newApp(testStore(t), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	get := request(a, "GET", "/", nil)
	if get.Code != 200 || !strings.Contains(get.Body.String(), "Iniciar QSO") {
		t.Fatalf("dashboard: %d", get.Code)
	}
	csrf := url.Values{"csrf": {a.token}, "previous_round": {"0"}, "name": {"Example round"}}
	if w := request(a, "POST", "/round/start", csrf); w.Code != 303 {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	round, _ := a.store.currentRound(context.Background())
	form := url.Values{"csrf": {a.token}, "round_id": {strconv.FormatInt(round.ID, 10)}, "callsign": {"ZZ0HTTP"}, "mode": {"FM"}}
	if w := request(a, "POST", "/participant", form); w.Code != 303 {
		t.Fatalf("include: %d %s", w.Code, w.Body)
	}
	list, _ := a.store.participants(context.Background())
	action := url.Values{"csrf": {a.token}, "id": {strconv.FormatInt(list[0].ID, 10)}, "active": {"0"}}
	if w := request(a, "POST", "/participant/status", action); w.Code != 303 {
		t.Fatalf("QRT: %d %s", w.Code, w.Body)
	}
	if w := request(a, "GET", "/round", nil); !strings.Contains(w.Body.String(), "Voltou") {
		t.Fatal("QRT state absent")
	}
	if w := request(a, "POST", "/round/end", url.Values{"csrf": {a.token}, "id": {strconv.FormatInt(round.ID, 10)}}); w.Code != 303 {
		t.Fatalf("end: %d %s", w.Code, w.Body)
	}
	closed := request(a, "GET", "/round?id="+strconv.FormatInt(round.ID, 10), nil)
	if closed.Code != 200 || strings.Contains(closed.Body.String(), "Adicionar à rodada") || strings.Contains(closed.Body.String(), "/participant/status") {
		t.Fatal("closed round is editable")
	}
	if w := request(a, "POST", "/participant", form); w.Code != 404 {
		t.Fatalf("closed addition: %d", w.Code)
	}
	if w := request(a, "POST", "/round/start", csrf); w.Code != 409 {
		t.Fatalf("old start: %d", w.Code)
	}
	if w := request(a, "POST", "/participant/status", url.Values{"id": {"1"}, "active": {"0"}}); w.Code != 403 {
		t.Fatal("missing CSRF protection")
	}
}

func TestRoundMigrationFailurePreservesVersionTwo(t *testing.T) {
	path := legacyDatabase(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(participantIdentitySchema); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE participants SET station_id=9999 WHERE id=23"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if migrated, err := openStore(context.Background(), path); err == nil {
		migrated.db.Close()
		t.Fatal("accepted invalid legacy participation")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, rounds, participants, contacts int
	for query, target := range map[string]*int{
		"PRAGMA user_version": &version,
		"SELECT count(*) FROM sqlite_master WHERE name='rounds'":            &rounds,
		"SELECT count(*) FROM participants WHERE id=23 AND station_id=9999": &participants,
		"SELECT count(*) FROM qsos WHERE id=11":                             &contacts,
	} {
		if err := db.QueryRow(query).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if version != 2 || rounds != 0 || participants != 1 || contacts != 1 {
		t.Fatalf("migration failed to roll back: %d %d %d %d", version, rounds, participants, contacts)
	}
}
