package main

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWriteRoutesRejectForgeryWithoutChangingData(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: "ZZ0SAFE"}, "FM", "Saved notes"); err != nil {
		t.Fatal(err)
	}
	if err := a.store.recordParticipant(ctx, 1); err != nil {
		t.Fatal(err)
	}
	before := totalChanges(t, a.store)
	for _, path := range []string{"/records", "/location", "/station", "/contact", "/participant", "/participant/remove", "/participant/record", "/participant/status", "/round/start", "/round/end", "/map/preference", "/map/locate", "/language"} {
		for _, attack := range []struct{ origin, token string }{
			{"http://external.example", a.token},
			{"null", a.token},
			{"http://127.0.0.1:8080", "invalid-token"},
			{"http://127.0.0.1:8080", ""},
		} {
			form := url.Values{"csrf": {attack.token}, "id": {"1"}, "round_id": {"1"}, "previous_round": {"1"}, "callsign": {"ZZ0SAFE"}, "name": {"Overwrite"}, "mode": {"FM"}, "notes": {"Overwrite"}, "active": {"0"}, "enabled": {"1"}, "language": {"fr"}, "return_to": {"/"}}
			r := httptest.NewRequest("POST", "http://127.0.0.1:8080"+path, strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", attack.origin)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != 403 || len(w.Result().Cookies()) != 0 {
				t.Fatalf("%s accepted forged form: %d %s", path, w.Code, w.Body)
			}
		}
	}
	for _, path := range []string{"/participant/remove", "/participant/record", "/participant/status", "/round/start", "/round/end", "/map/preference", "/map/locate", "/language"} {
		for _, method := range []string{"GET", "HEAD"} {
			w := request(a, method, path+"?id=1&enabled=1&active=0&language=fr&return_to=/", nil)
			if w.Code != 405 {
				t.Errorf("%s %s: got %d, want 405", method, path, w.Code)
			}
		}
	}
	if after := totalChanges(t, a.store); after != before {
		t.Fatal("rejected requests changed the notebook")
	}
}

func TestStoredInputRemainsDataInSQLAndHTML(t *testing.T) {
	a := testApp(t)
	const name = "O'Brien'); DROP TABLE qsos; --"
	const payload = `</textarea><script>alert('example')</script><img src=x onerror=alert(1)>`
	form := url.Values{"csrf": {a.token}, "round_id": {"1"}, "callsign": {"ZZ0ESCAPE"}, "name": {name}, "city": {payload}, "mode": {"FM"}, "notes": {payload}}
	for _, path := range []string{"/station", "/participant", "/contact"} {
		if w := request(a, "POST", path, form); w.Code != 303 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	before := totalChanges(t, a.store)
	for _, path := range []string{"/", "/round", "/stations", "/history", "/station?id=1", "/contact?id=1", "/participant?id=1"} {
		w := request(a, "GET", path, nil)
		if w.Code != 200 || strings.Contains(w.Body.String(), payload) || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
			t.Errorf("%s failed to escape stored input: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/stations", "/history", "/lookup"} {
		w := request(a, "GET", path+"?q="+url.QueryEscape(name)+"&callsign="+url.QueryEscape("' OR 1=1 --"), nil)
		if w.Code != 200 && w.Code != 404 {
			t.Errorf("%s SQL-like search failed: %d", path, w.Code)
		}
	}
	if after := totalChanges(t, a.store); after != before {
		t.Fatal("rendering/searching stored input changed the notebook")
	}
}

func TestPrivatePathsAreNeverStaticAssets(t *testing.T) {
	a := testApp(t)
	for _, path := range []string{"/data/qsobook.db", "/qsobook.db", "/.env", "/.git/config", "/secrets/access-password.txt", "/static/../data/qsobook.db", "/static/%2e%2e/%2e%2e/.env", "/static/%2e%2e/secrets/access-password.txt", "/static/../../store.go", "/static/.env"} {
		w := request(a, "GET", path, nil)
		// Follow ServeMux's path-cleaning redirect, as a browser would.
		for redirects := 0; redirects < 3 && w.Code >= 300 && w.Code < 400; redirects++ {
			w = request(a, "GET", w.Header().Get("Location"), nil)
		}
		if w.Code != 404 {
			t.Errorf("private path %s: got %d, want 404", path, w.Code)
		}
	}
}

// Store uses one SQLite connection. Count writes, including attempted changes
// later rolled back, so a rejected request cannot hide a partial mutation.
func totalChanges(t *testing.T, store *Store) int64 {
	t.Helper()
	var changes int64
	if err := store.db.QueryRow("SELECT total_changes()").Scan(&changes); err != nil {
		t.Fatal(err)
	}
	return changes
}
