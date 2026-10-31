package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRadioIDClient(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
		found, bad bool
	}{
		{"multiple exact IDs", `{"results":[{"callsign":"ZZ0OTHER","id":3},{"callsign":"zz0radio","id":2,"name":"Other profile"},{"callsign":"ZZ0RADIO","radio_id":"1","fname":"Example","surname":"Operator","city":"Example city"}]}`, 200, true, false},
		{"not found", `{"results":[]}`, 200, false, false},
		{"no fuzzy matches", `{"results":[{"callsign":"ZZ0RADIO/P","id":1}]}`, 200, false, false},
		{"unavailable", `{}`, 503, false, true},
		{"invalid response", `{"message":"not a result list"}`, 200, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if q := r.URL.Query(); len(q) != 3 || q.Get("callsign") != "ZZ0RADIO" || q.Get("callsign_sel") != "=" || !strings.Contains(r.UserAgent(), "QSObook") {
					t.Error("unexpected RadioID query or missing client identification")
				}
				w.Header().Set("Retry-After", "7200")
				w.WriteHeader(tc.code)
				w.Write([]byte(tc.body))
			}))
			defer provider.Close()
			client := newRadioIDClient()
			client.endpoint = provider.URL
			result, err := client.lookup(context.Background(), " zz0radio ")
			if (err != nil) != tc.bad || result.Found != tc.found {
				t.Fatalf("found=%v error=%v", result.Found, err)
			}
			if tc.found && (result.Station.Name != "Example Operator" || len(result.IDs) != 2 || result.IDs[0] != 1 || result.IDs[1] != 2) {
				t.Fatal("wrong profile, non-exact match or lost IDs", result)
			}
			if tc.code == 503 && !result.RetryAt.After(time.Now().Add(time.Hour)) {
				t.Fatal("Retry-After ignored")
			}
		})
	}
}

func TestRadioIDCorrectionsHistoryAndBackup(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"callsign":"ZZ0RADIO","id":1,"name":"External name","city":"External city","state":"EX","country":"Exampleland"}]}`))
	}))
	defer provider.Close()
	a.radioID.client.endpoint = provider.URL
	if _, err := a.store.saveStation(ctx, Station{Callsign: "ZZ0RADIO", Name: "Manual name", Notes: "Permanent"}); err != nil {
		t.Fatal(err)
	}
	// Compatible older notebooks can retain surrounding whitespace.
	if _, err := a.store.db.Exec("UPDATE stations SET notes=' Permanent ' WHERE callsign='ZZ0RADIO'"); err != nil {
		t.Fatal(err)
	}
	if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: "ZZ0RADIO"}, "FM", "Conversation"); err != nil {
		t.Fatal(err)
	}
	if err := a.store.recordParticipant(ctx, 1); err != nil {
		t.Fatal(err)
	}
	before, err := a.store.contact(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.store.endRound(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if w := request(a, "POST", "/radioid/preference", url.Values{"csrf": {a.token}, "enabled": {"1"}}); w.Code != 303 {
		t.Fatal("enable", w.Code)
	}
	w := request(a, "POST", "/lookup/radioid", url.Values{"csrf": {a.token}, "callsign": {"zz0radio"}})
	var result stationLookup
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || w.Code != 200 || result.RadioIDStatus != "found" || result.RadioIDRevision == "" || result.Name != "Manual name" || result.City != "External city" || result.Contacts != 1 {
		t.Fatal("enrichment or local priority", w.Code, w.Body)
	}
	old, err := a.store.participantsInRound(ctx, 1)
	if err != nil || len(old) != 1 || old[0].City != "" || old[0].Station.Notes != " Permanent " {
		t.Fatal("historical participant changed", old, err)
	}
	round, err := a.store.startRound(ctx, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	st := result.Station
	st.City, st.radioIDRevision = "Manual correction", result.RadioIDRevision
	key, err := newSubmissionKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.store.saveWorkspaceEntry(ctx, round, st, "FM", "2 m", "", key, false); err != nil {
		t.Fatal(err)
	}
	st.City = "Stale form"
	if _, err = a.store.saveWorkspaceEntry(ctx, round, st, "FM", "2 m", "", key, false); err != nil {
		t.Fatal(err)
	}
	current, err := a.store.lookup(ctx, st.Callsign)
	if err != nil || current.City != "Manual correction" || current.Notes != " Permanent " {
		t.Fatal("correction lost", current, err)
	}
	current.Country = ""
	if _, err = a.store.saveStation(ctx, current); err != nil {
		t.Fatal(err)
	}
	if _, err = a.store.db.Exec("UPDATE radioid_lookups SET next_attempt=0; UPDATE app_settings SET radioid_after=0"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.enrichRadioID(ctx, st.Callsign); err != nil {
		t.Fatal(err)
	}
	current, err = a.store.lookup(ctx, st.Callsign)
	if err != nil || current.City != "Manual correction" || current.Country != "Exampleland" {
		t.Fatal("retry overwrote a correction or lost an empty field", current, err)
	}
	if after, err := a.store.contact(ctx, 1); err != nil || after != before {
		t.Fatal("historical QSO changed", after, err)
	}
	backup := filepath.Join(t.TempDir(), "radioid.db")
	if err := a.store.backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restored := testStore(t)
	if err := restored.restore(ctx, backup); err != nil {
		t.Fatal(err)
	}
	m, err := restored.radioIDMetadata(ctx, st.Callsign)
	if err != nil || m.Status != "found" || len(m.IDs) != 1 || m.Revision == "" {
		t.Fatal("metadata did not survive restore", m, err)
	}
	stations, err := restored.stations(ctx, "ZZ0RADIO")
	if err != nil || len(stations) != 1 || stations[0] != current {
		t.Fatal("duplicate or lost registration", stations, err)
	}
}

func TestRadioIDOfflineSingleFlightAndRestart(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	var calls atomic.Int32
	var online atomic.Bool
	started, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !online.Load() {
			started <- struct{}{}
			<-release
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"results":[{"callsign":"ZZ0OFF","id":2,"name":"External","city":"Example","state":"EX","country":"Exampleland"}]}`))
	}))
	t.Cleanup(func() { once.Do(func() { close(release) }); provider.Close() })
	a.radioID.client.endpoint = provider.URL
	if status, err := a.enrichRadioID(ctx, "ZZ0OFF"); err != nil || status != "disabled" || calls.Load() != 0 {
		t.Fatal("lookup before consent", status, err)
	}
	if w := request(a, "POST", "/radioid/preference", url.Values{"csrf": {a.token}, "enabled": {"1"}}); w.Code != 303 {
		t.Fatal("enable", w.Code)
	}
	done := make(chan string, 1)
	go func() { status, _ := a.enrichRadioID(ctx, "ZZ0OFF"); done <- status }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("lookup did not start")
	}
	if status, err := a.enrichRadioID(ctx, "ZZ0OFF"); err != nil || status != "pending" || calls.Load() != 1 {
		t.Fatal("concurrent duplicate", status, err)
	}
	key, err := newSubmissionKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.store.saveWorkspaceEntry(ctx, 1, Station{Callsign: "ZZ0OFF", Name: "Typed name"}, "FM", "2 m", "Offline QSO", key, true); err != nil {
		t.Fatal("offline QSO blocked by lookup", err)
	}
	once.Do(func() { close(release) })
	if status := <-done; status != "unavailable" {
		t.Fatal("offline result", status)
	}
	m, err := a.store.radioIDMetadata(ctx, "ZZ0OFF")
	if err != nil || m.Status != "unavailable" || m.Next <= time.Now().Unix() {
		t.Fatal("retry was not persisted", m, err)
	}
	if err := a.store.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openStore(ctx, a.store.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.db.Close() })
	resumed, err := newApp(reopened, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	resumed.radioID.client.endpoint = provider.URL
	if saved, err := reopened.radioIDMetadata(ctx, "ZZ0OFF"); err != nil || saved.Next != m.Next || saved.Status != m.Status {
		t.Fatal("restart lost pending state", saved, err)
	}
	online.Store(true)
	if _, err := reopened.db.Exec("UPDATE radioid_lookups SET next_attempt=0; UPDATE app_settings SET radioid_after=0,radioid_sync_after=0"); err != nil {
		t.Fatal(err)
	}
	resumed.syncRadioID(ctx)
	current, err := reopened.lookup(ctx, "ZZ0OFF")
	if err != nil || current.Name != "Typed name" || current.City != "Example" || calls.Load() != 2 {
		t.Fatal("resume failed or overwrote manual data", current, err, calls.Load())
	}
	contact, err := reopened.contact(ctx, 1)
	if err != nil || contact.City != "" || contact.Notes != "Offline QSO" {
		t.Fatal("retry rewrote history", contact, err)
	}
	if w := request(resumed, "POST", "/radioid/preference", url.Values{"csrf": {resumed.token}}); w.Code != 303 {
		t.Fatal("disable", w.Code)
	}
	resumed.syncRadioID(ctx)
	if status, err := resumed.enrichRadioID(ctx, "ZZ0OFF"); err != nil || status != "disabled" || calls.Load() != 2 {
		t.Fatal("lookup after opt-out", status, err)
	}
}
