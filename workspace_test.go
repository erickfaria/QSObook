package main

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceListeningRoundMemoryAndEditionSnapshots(t *testing.T) {
	a, err := newApp(testStore(t), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	form := url.Values{"csrf": {a.token}, "previous_round": {"0"}, "workspace": {"1"}, "name": {"Saved net"}, "edition": {"42"}}
	// A remembered name must not implicitly opt a listening session into a roundtable.
	if w := request(a, "POST", "/round/start", form); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	listening, _ := a.store.currentRound(ctx)
	if listening.Name != "" || listening.Edition != "" {
		t.Fatal("listening became a roundtable", listening)
	}
	a.store.endRound(ctx, listening.ID)
	form.Set("previous_round", strconv.FormatInt(listening.ID, 10))
	form.Set("in_round", "1")
	if w := request(a, "POST", "/round/start", form); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	round, _ := a.store.currentRound(ctx)
	if round.Label() != "Saved net #42" {
		t.Fatal(round)
	}
	st := Station{Callsign: "ZZ0WORK", Name: "Example", City: "Example city", Notes: "Permanent"}
	qid, err := a.store.saveWorkspaceEntry(ctx, round.ID, st, "DMR", "70 cm", "Pending conversation", strings.Repeat("a", 32), false)
	if err != nil || qid != 0 {
		t.Fatal(qid, err)
	}
	contacts, _ := a.store.contacts(ctx, "")
	if len(contacts) != 0 {
		t.Fatal("listening confirmed contact")
	}
	participants, _ := a.store.participants(ctx)
	if len(participants) != 1 || participants[0].Notes != "Pending conversation" {
		t.Fatal(participants)
	}
	if err = a.store.recordParticipant(ctx, participants[0].ID); err != nil {
		t.Fatal(err)
	}
	contacts, _ = a.store.contacts(ctx, "")
	if len(contacts) != 1 || contacts[0].RoundName != "Saved net #42" || contacts[0].Notes != "Pending conversation" {
		t.Fatal("snapshot", contacts)
	}
	a.store.endRound(ctx, round.ID)
	w := request(a, "GET", "/round", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `value="Saved net"`) || !strings.Contains(w.Body.String(), `value="42"`) {
		t.Fatal("name/edition not remembered", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), `name="in_round" value="1" checked`) {
		t.Fatal("round opted in automatically")
	}
	if _, err = a.store.startRound(ctx, "Saved net", round.ID, "43"); err != nil {
		t.Fatal(err)
	}
	old, _ := a.store.contact(ctx, contacts[0].ID)
	if old.RoundName != "Saved net #42" {
		t.Fatal("old edition changed")
	}
}
func TestWorkspaceConcurrentConfirmationAndQRTOrder(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	st := Station{Callsign: "ZZ0A", Name: "First", Notes: "Permanent"}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.store.saveWorkspaceEntry(ctx, 1, st, "FM", "2 m", "Conversation", strings.Repeat("b", 32), true)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	qsos, _ := a.store.contacts(ctx, "")
	parts, _ := a.store.participants(ctx)
	if len(qsos) != 1 || len(parts) != 1 {
		t.Fatal("duplicate confirmation", len(qsos), len(parts))
	}
	a.store.setParticipantActive(ctx, parts[0].ID, false)
	// A fresh form for the same participation does not duplicate a QSO or undo QRT.
	if _, err := a.store.saveWorkspaceEntry(ctx, 1, st, "DMR", "70 cm", "Changed", strings.Repeat("c", 32), true); err != nil {
		t.Fatal(err)
	}
	a.store.saveWorkspaceEntry(ctx, 1, Station{Callsign: "ZZ0B"}, "FM", "", "", strings.Repeat("d", 32), false)
	parts, _ = a.store.participants(ctx)
	if len(parts) != 2 || parts[0].Callsign != "ZZ0B" || parts[1].Callsign != "ZZ0A" || parts[1].Active {
		t.Fatal("QRT not last", parts)
	}
	qsos, _ = a.store.contacts(ctx, "")
	if len(qsos) != 1 || qsos[0].Mode != "FM" || qsos[0].Notes != "Conversation" {
		t.Fatal("snapshot overwritten", qsos)
	}
	w := request(a, "GET", "/round", nil)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `class="qrt-row"`) || strings.Index(body, `>ZZ0B</a>`) > strings.Index(body, `>ZZ0A</a>`) {
		t.Fatal("QRT rendering/order", w.Code)
	}
	a.store.endRound(ctx, 1)
	if _, err := a.store.saveWorkspaceEntry(ctx, 1, Station{Callsign: "ZZ0STALE"}, "FM", "", "", strings.Repeat("e", 32), false); err != errRoundChanged {
		t.Fatal("closed session wrote", err)
	}
	if _, err := a.store.lookup(ctx, "ZZ0STALE"); err == nil {
		t.Fatal("stale session created registration")
	}
}
func TestWorkspaceHTTPDefaultsAndDrafts(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	home := request(a, "GET", "/", nil)
	if !strings.Contains(home.Body.String(), `href="/round"`) || strings.Contains(home.Body.String(), `href="/records"`) {
		t.Fatal("duplicated entry navigation")
	}
	form := url.Values{"csrf": {a.token}, "round_id": {"1"}, "submission_key": {recordKey(t, a)}, "callsign": {"ZZ0FLOW"}, "name": {"Example"}, "mode": {"FM"}, "operator_notes": {"Permanent"}, "notes": {"Listening notes"}}
	if w := request(a, "POST", "/qso/entry", form); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	qsos, _ := a.store.contacts(ctx, "")
	parts, _ := a.store.participants(ctx)
	if len(qsos) != 0 || len(parts) != 1 || parts[0].Notes != "Listening notes" {
		t.Fatal("unconfirmed contact", parts)
	}
	form.Set("submission_key", recordKey(t, a))
	form.Set("confirmed", "1")
	for i := 0; i < 2; i++ {
		if w := request(a, "POST", "/qso/entry", form); w.Code != 303 {
			t.Fatal(w.Code, w.Body)
		}
	}
	qsos, _ = a.store.contacts(ctx, "")
	if len(qsos) != 1 {
		t.Fatal("duplicate", len(qsos))
	}
	st, _ := a.store.lookup(ctx, "ZZ0FLOW")
	if st.Notes != "Permanent" || qsos[0].Notes != "Listening notes" {
		t.Fatal("notes mixed")
	}
	form.Set("callsign", "bad space")
	form.Set("name", "Draft name")
	form.Set("notes", "Draft note")
	if w := request(a, "POST", "/qso/entry", form); w.Code != 400 || !strings.Contains(w.Body.String(), "Draft name") || !strings.Contains(w.Body.String(), "Draft note") {
		t.Fatal("draft lost", w.Code)
	}
}
