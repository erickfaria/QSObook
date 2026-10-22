package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, err := newApp(testStore(t), time.FixedZone("BRT", -3*3600))
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func request(a *App, method, path string, form url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestHTTPWorkflow(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	for _, path := range []string{"/", "/history", "/stations", "/station", "/contact", "/static/style.css", "/static/app.js", "/license", "/third-party-licenses"} {
		w := request(a, "GET", path, nil)
		if w.Code != 200 {
			t.Fatalf("GET %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	form := url.Values{"csrf": {a.token}, "callsign": {"zz0test"}, "name": {"Operador fictício"}, "city": {"Cidade exemplo"}, "country": {"Brasil"}, "mode": {"FM"}, "notes": {"<script>alert(1)</script>"}}
	w := request(a, "POST", "/station", form)
	if w.Code != 303 {
		t.Fatalf("station: %d %s", w.Code, w.Body.String())
	}
	w = request(a, "POST", "/participant", form)
	if w.Code != 303 {
		t.Fatalf("participant: %d %s", w.Code, w.Body.String())
	}
	w = request(a, "GET", "/", nil)
	if strings.Contains(w.Body.String(), "<script>alert") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatal("unsafe or missing escaped notes")
	}
	w = request(a, "GET", "/lookup?callsign=zz0test", nil)
	var data struct {
		Station
		Contacts int `json:"contacts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.Callsign != "ZZ0TEST" || data.Contacts != 0 {
		t.Fatalf("lookup: %s %v", w.Body, err)
	}
	w = request(a, "GET", "/participant?id=1", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Editar participante") {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	edit := url.Values{"csrf": {a.token}, "id": {"1"}, "callsign": {"ZZ0TEST"}, "name": {"Nome alterado"}, "mode": {"SSB"}, "notes": {"Observação de rodada"}}
	if w = request(a, "POST", "/participant", edit); w.Code != 303 {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	for i := 0; i < 2; i++ {
		w = request(a, "POST", "/participant/record", url.Values{"csrf": {a.token}, "id": {"1"}})
		if w.Code != 303 {
			t.Fatalf("record: %d %s", w.Code, w.Body)
		}
	}
	contacts, err := a.store.contacts(ctx, "zz0test")
	if err != nil || len(contacts) != 1 || contacts[0].Mode != "SSB" {
		t.Fatalf("contacts: %+v %v", contacts, err)
	}
	w = request(a, "GET", "/history?q=Nome", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Nome alterado") {
		t.Fatalf("history: %s", w.Body)
	}
	w = request(a, "GET", "/contact?id=1", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "BRT") {
		t.Fatalf("details: %d %s", w.Code, w.Body)
	}
	edit.Set("notes", "Correção do contato")
	if w = request(a, "POST", "/contact", edit); w.Code != 303 {
		t.Fatalf("contact update: %d %s", w.Code, w.Body)
	}
	c, _ := a.store.contact(ctx, 1)
	if c.Notes != "Correção do contato" {
		t.Fatal("contact not updated")
	}
	if w = request(a, "POST", "/participant/remove", url.Values{"csrf": {a.token}, "id": {"1"}}); w.Code != 303 {
		t.Fatalf("remove: %d", w.Code)
	}
	participants, _ := a.store.participants(ctx)
	contacts, _ = a.store.contacts(ctx, "")
	if len(participants) != 0 || len(contacts) != 1 {
		t.Fatal("remove damaged history")
	}
	if _, err := a.store.lookup(ctx, "ZZ0TEST"); err != nil {
		t.Fatal("station removed", err)
	}
	form.Set("callsign", "ZZ1TEST")
	if w = request(a, "POST", "/contact", form); w.Code != 303 {
		t.Fatalf("manual contact: %d %s", w.Code, w.Body)
	}
}
func TestHTTPSecurityAndErrors(t *testing.T) {
	a := testApp(t)
	for _, tc := range []struct {
		method, path string
		form         url.Values
		want         int
	}{
		{"POST", "/station", url.Values{"callsign": {"ZZ0TEST"}}, 403},
		{"POST", "/station", url.Values{"csrf": {a.token}, "callsign": {"bad space"}}, 400},
		{"POST", "/participant", url.Values{"csrf": {a.token}, "callsign": {"ZZ0TEST"}, "mode": {""}}, 400},
		{"POST", "/contact", url.Values{"csrf": {a.token}, "callsign": {"ZZ0TEST"}, "mode": {""}}, 400},
		{"GET", "/contact?id=999", nil, 404}, {"GET", "/station?id=bad", nil, 400},
		{"GET", "/participant?id=999", nil, 404}, {"GET", "/lookup?callsign=ZZ9TEST", nil, 404},
		{"GET", "/participant/remove", nil, 405}, {"GET", "/data/test.db", nil, 404},
		{"POST", "/participant/remove", url.Values{"csrf": {a.token}, "id": {"999"}}, 404},
		{"POST", "/station", url.Values{"csrf": {a.token}, "notes": {strings.Repeat("x", 40000)}}, 400},
	} {
		w := request(a, tc.method, tc.path, tc.form)
		if w.Code != tc.want {
			t.Errorf("%s %s: %d want %d (%s)", tc.method, tc.path, w.Code, tc.want, w.Body)
		}
	}
	for _, host := range []string{"evil.example", "127.0.0.1.evil.example"} {
		r := httptest.NewRequest("GET", "http://"+host+"/", nil)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("host: %d", w.Code)
		}
	}
	for _, origin := range []string{"http://evil.example", "http://127.0.0.1:9090", "null"} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/station", strings.NewReader(url.Values{"csrf": {a.token}}.Encode()))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("origin %q: %d", origin, w.Code)
		}
	}
	w := request(a, "GET", "/", nil)
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing security header")
	}
	a.store.db.Close()
	w = request(a, "GET", "/history", nil)
	if w.Code != 500 || strings.Contains(w.Body.String(), "sql:") {
		t.Fatalf("error leak: %d %s", w.Code, w.Body)
	}
}

func TestInvalidEditPreservesRecord(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	id, err := a.store.saveStation(ctx, Station{Callsign: "ZZ0EDIT", Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	qid, err := a.store.record(ctx, id, "FM", "")
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 || qid != 1 {
		t.Fatal("unexpected test IDs")
	}
	form := url.Values{"csrf": {a.token}, "id": {"1"}, "callsign": {"ZZ0EDIT"}, "name": {"Corrigido"}, "mode": {""}}
	w := request(a, "POST", "/contact", form)
	if w.Code != 400 || !strings.Contains(w.Body.String(), `name="id" value="1"`) || !strings.Contains(w.Body.String(), `value="Corrigido"`) {
		t.Fatalf("edit lost: %d %s", w.Code, w.Body)
	}
	form.Set("mode", "Modo personalizado")
	w = request(a, "POST", "/contact", form)
	if w.Code != 303 {
		t.Fatalf("correction: %d %s", w.Code, w.Body)
	}
	list, err := a.store.contacts(ctx, "")
	if err != nil || len(list) != 1 || list[0].Name != "Corrigido" {
		t.Fatalf("edit duplicated record: %+v %v", list, err)
	}
}
