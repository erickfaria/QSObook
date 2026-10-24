package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"strconv"
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
	if _, err := a.store.startRound(context.Background(), "", 0); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestHTTPParticipantPreservesRegistrationWithoutLookup(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	id, err := a.store.saveStation(ctx, Station{Callsign: "ZZ0LOCAL", Name: "Nome salvo", City: "Cidade salva", State: "UF", Country: "Brasil", Notes: "Nota permanente"})
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {" zz0local "}, "mode": {"DMR"}, "notes": {"Nota da conversa"}}
	for i := 0; i < 2; i++ {
		if w := request(a, "POST", "/participant", form); w.Code != 303 {
			t.Fatalf("addition: %d %s", w.Code, w.Body)
		}
	}
	st, err := a.store.station(ctx, id)
	if err != nil || st.Name != "Nome salvo" || st.City != "Cidade salva" || st.State != "UF" || st.Country != "Brasil" || st.Notes != "Nota permanente" {
		t.Fatalf("registration overwritten: %+v %v", st, err)
	}
	participants, err := a.store.participants(ctx)
	if err != nil || len(participants) != 1 || participants[0].Notes != "Nota da conversa" {
		t.Fatalf("participants: %+v %v", participants, err)
	}
	contacts, err := a.store.contacts(ctx, "")
	if err != nil || len(contacts) != 0 {
		t.Fatalf("unconfirmed QSO: %+v %v", contacts, err)
	}
	record := url.Values{"csrf": {a.token}, "id": {strconv.FormatInt(participants[0].ID, 10)}}
	for i := 0; i < 2; i++ {
		if w := request(a, "POST", "/participant/record", record); w.Code != 303 {
			t.Fatalf("confirmation: %d %s", w.Code, w.Body)
		}
	}
	contacts, err = a.store.contacts(ctx, "")
	if err != nil || len(contacts) != 1 || contacts[0].Notes != "Nota da conversa" || contacts[0].City != "Cidade salva" {
		t.Fatalf("confirmed QSO: %+v %v", contacts, err)
	}
	// The manual QSO form follows the same separation of notes.
	form.Set("notes", "Outra conversa")
	if w := request(a, "POST", "/contact", form); w.Code != 303 {
		t.Fatalf("manual QSO: %d %s", w.Code, w.Body)
	}
	st, err = a.store.station(ctx, id)
	if err != nil || st.Notes != "Nota permanente" {
		t.Fatalf("manual QSO changed permanent notes: %+v %v", st, err)
	}
}

func TestStaleLookupFormDoesNotOverwriteOperator(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	st := Station{Callsign: "ZZ0SAFE", Name: "Nome anterior", City: "Cidade anterior", Notes: "Nota permanente"}
	id, err := a.store.saveStation(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {" zz0safe "}, "name": {st.Name}, "city": {st.City}, "mode": {"DMR"}, "notes": {"Nota da conversa"}}
	st.ID = id
	st.Name = "Nome atual"
	st.City = "Cidade atual"
	if _, err := a.store.saveStation(ctx, st); err != nil {
		t.Fatal(err)
	}
	if w := request(a, "POST", "/participant", form); w.Code != 303 {
		t.Fatalf("addition: %d %s", w.Code, w.Body)
	}
	w := request(a, "GET", "/lookup?callsign="+url.QueryEscape(" zz0safe "), nil)
	var data struct {
		Station
		Contacts int `json:"contacts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || w.Code != 200 || data.Station != st || data.Contacts != 0 {
		t.Fatalf("lookup returned overwritten or duplicate registration: %+v, status %d, error %v", data, w.Code, err)
	}
}

func TestRoundInlineEditAndValidation(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	if _, err := a.store.saveStation(ctx, Station{Callsign: "ZZ0OTHER"}); err != nil {
		t.Fatal(err)
	}
	if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: "ZZ0EDIT", Name: "Nome atual", Notes: "Não copiar"}, "DMR", "Nota da conversa"); err != nil {
		t.Fatal(err)
	}
	st, err := a.store.lookup(ctx, "ZZ0EDIT")
	if err != nil {
		t.Fatal(err)
	}
	st.Notes = "Nota permanente"
	if _, err := a.store.saveStation(ctx, st); err != nil {
		t.Fatal(err)
	}
	w := request(a, "GET", "/round", nil)
	body := w.Body.String()
	for _, want := range []string{`data-edit-participant`, `id="participant-edit-1"`, `name="id" value="1"`, `Nota permanente`, `Nota da conversa`, `Registrar QSO`, `Cidade / UF`} {
		if w.Code != 200 || !strings.Contains(body, want) {
			t.Fatalf("round missing %q: %d %s", want, w.Code, body)
		}
	}
	if strings.Count(body, `id="lookup-status"`) != 1 || strings.Count(body, `id="modes"`) != 1 || strings.Count(body, `name="notes"`) != 2 {
		t.Fatalf("field counts: status=%d modes=%d notes=%d", strings.Count(body, `id="lookup-status"`), strings.Count(body, `id="modes"`), strings.Count(body, `name="notes"`))
	}
	form := url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {"bad space"}, "name": {"Digitado"}, "mode": {"DMR"}, "notes": {"Rascunho"}}
	form.Set("band", "70 cm")
	w = request(a, "POST", "/participant", form)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Estações nesta sessão") || !strings.Contains(w.Body.String(), `value="Digitado"`) || !strings.Contains(w.Body.String(), "Rascunho") || !strings.Contains(w.Body.String(), `value="70 cm" selected`) {
		t.Fatalf("validation lost round or draft: %d %s", w.Code, w.Body)
	}
	form.Set("id", "1")
	form.Set("callsign", st.Callsign)
	form.Set("name", "Nome corrigido")
	form.Set("notes", "")
	w = request(a, "POST", "/participant", form)
	if w.Code != 303 {
		t.Fatalf("inline edit: %d %s", w.Code, w.Body)
	}
	got, err := a.store.station(ctx, st.ID)
	if err != nil || got.Name != "Nome corrigido" || got.Notes != "Nota permanente" {
		t.Fatalf("wrong registration changed: %+v %v", got, err)
	}
	list, err := a.store.participants(ctx)
	if err != nil || len(list) != 1 || list[0].Notes != "" {
		t.Fatalf("conversation notes not cleared: %+v %v", list, err)
	}
}

func TestStaleParticipantEditDoesNotRecreateParticipation(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: "ZZ0REMOVED", Name: "Original"}, "FM", ""); err != nil {
		t.Fatal(err)
	}
	list, err := a.store.participants(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("participants: %+v %v", list, err)
	}
	form := url.Values{"csrf": {a.token}, "id": {strconv.FormatInt(list[0].ID, 10)}}
	if w := request(a, "POST", "/participant/remove", form); w.Code != 303 {
		t.Fatalf("remove: %d %s", w.Code, w.Body)
	}
	form.Set("round_id", "1")
	form.Set("callsign", "ZZ0REMOVED")
	form.Set("name", "Alteração de formulário antigo")
	form.Set("mode", "DMR")
	if w := request(a, "POST", "/participant", form); w.Code != 404 {
		t.Fatalf("stale edit: %d %s", w.Code, w.Body)
	}
	st, err := a.store.lookup(ctx, "ZZ0REMOVED")
	if err != nil || st.Name != "Original" {
		t.Fatalf("stale edit changed registration: %+v %v", st, err)
	}
	list, err = a.store.participants(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("stale edit recreated participation: %+v %v", list, err)
	}
}

func TestOldParticipantActionsDoNotReachNewParticipation(t *testing.T) {
	for _, callsign := range []string{"ZZ0OTHER", "ZZ0OLD"} {
		t.Run(callsign, func(t *testing.T) {
			a := testApp(t)
			ctx := context.Background()
			if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: "ZZ0OLD"}, "FM", "Old round"); err != nil {
				t.Fatal(err)
			}
			old, err := a.store.participants(ctx)
			if err != nil || len(old) != 1 {
				t.Fatalf("old participation: %+v %v", old, err)
			}
			form := url.Values{"csrf": {a.token}, "id": {strconv.FormatInt(old[0].ID, 10)}}
			if w := request(a, "POST", "/participant/remove", form); w.Code != 303 {
				t.Fatalf("remove: %d %s", w.Code, w.Body)
			}
			if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: callsign}, "DMR", "New round"); err != nil {
				t.Fatal(err)
			}
			current, err := a.store.participants(ctx)
			if err != nil || len(current) != 1 || current[0].ID == old[0].ID {
				t.Fatalf("participant ID reused: old=%+v current=%+v err=%v", old, current, err)
			}
			for _, path := range []string{"/participant/record", "/participant/remove"} {
				if w := request(a, "POST", path, form); w.Code != 404 {
					t.Fatalf("old action %s: %d %s", path, w.Code, w.Body)
				}
			}
			current, err = a.store.participants(ctx)
			if err != nil || len(current) != 1 || current[0].Callsign != callsign || current[0].QSOID.Valid {
				t.Fatalf("old form affected new participation: %+v %v", current, err)
			}
			contacts, err := a.store.contacts(ctx, "")
			if err != nil || len(contacts) != 0 {
				t.Fatalf("old form created a QSO: %+v %v", contacts, err)
			}
		})
	}
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
	form := url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {"zz0test"}, "name": {"Operador fictício"}, "city": {"Cidade exemplo"}, "country": {"Brasil"}, "mode": {"FM"}, "notes": {"<script>alert(1)</script>"}}
	w := request(a, "POST", "/station", form)
	if w.Code != 303 {
		t.Fatalf("station: %d %s", w.Code, w.Body.String())
	}
	w = request(a, "POST", "/participant", form)
	if w.Code != 303 {
		t.Fatalf("participant: %d %s", w.Code, w.Body.String())
	}
	w = request(a, "GET", "/round", nil)
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
	edit := url.Values{"round_id": {"1"}, "csrf": {a.token}, "id": {"1"}, "callsign": {"ZZ0TEST"}, "name": {"Nome alterado"}, "mode": {"SSB"}, "notes": {"Observação de rodada"}}
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
		{"POST", "/station", url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {"bad space"}}, 400},
		{"POST", "/participant", url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {"ZZ0TEST"}, "mode": {""}}, 400},
		{"POST", "/contact", url.Values{"round_id": {"1"}, "csrf": {a.token}, "callsign": {"ZZ0TEST"}, "mode": {""}}, 400},
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
	w := request(a, "GET", "/round", nil)
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

// Ordinary browser form submissions must retain their Origin. no-referrer
// changes it to null, even for same-origin forms, so every POST is rejected.
func TestBrowserFormsKeepOriginAndPersist(t *testing.T) {
	a, err := newApp(testStore(t), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	const host = "192.168.1.93:18088"
	if err := a.configureAccess("", host); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/round", "/contact"} {
		r := httptest.NewRequest("GET", "http://"+host+path, nil)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Referrer-Policy") != "same-origin" {
			t.Fatalf("browser policy %s: %d %v", path, w.Code, w.Header())
		}
	}
	for _, tc := range []struct {
		path   string
		fields url.Values
	}{
		{"/round/start", url.Values{"previous_round": {"0"}, "name": {"Browser round"}}},
		{"/map/preference", url.Values{"enabled": {"1"}}},
		{"/contact", url.Values{"callsign": {"ZZ0BROWSER"}, "name": {"Example operator"}, "city": {"Example city"}, "mode": {"FM"}, "notes": {"Example conversation"}}},
	} {
		tc.fields.Set("csrf", a.token)
		r := httptest.NewRequest("POST", "http://"+host+tc.path, strings.NewReader(tc.fields.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "http://"+host)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 303 {
			t.Fatalf("browser POST %s: %d %s", tc.path, w.Code, w.Body)
		}
	}
	ctx := context.Background()
	enabled, err := a.store.mapEnabled(ctx)
	if err != nil || !enabled {
		t.Fatalf("map preference not saved: %v %v", enabled, err)
	}
	contacts, err := a.store.contacts(ctx, "ZZ0BROWSER")
	if err != nil || len(contacts) != 1 || contacts[0].Notes != "Example conversation" {
		t.Fatalf("contact not saved: %+v %v", contacts, err)
	}
	w := request(a, "GET", "/round", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Browser round") {
		t.Fatalf("round not saved: %d %s", w.Code, w.Body)
	}
}

func TestBrowserFormsStillRejectUntrustedRequests(t *testing.T) {
	a, err := newApp(testStore(t), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	const host = "192.168.1.93:18088"
	if err := a.configureAccess("", host); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/round/start", "/map/preference", "/contact"} {
		for _, tc := range []struct{ origin, fetchSite, token string }{
			{"null", "same-origin", a.token},
			{"http://evil.example:18088", "cross-site", a.token},
			{"http://192.168.1.93:18089", "same-site", a.token},
			{"https://192.168.1.93:18088", "same-origin", a.token},
			{"http://" + host, "cross-site", a.token},
			{"http://" + host, "same-origin", "wrong"},
		} {
			form := url.Values{"csrf": {tc.token}, "previous_round": {"0"}, "enabled": {"1"}, "callsign": {"ZZ0UNTRUSTED"}, "mode": {"FM"}}
			r := httptest.NewRequest("POST", "http://"+host+path, strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Errorf("%s origin=%q site=%q: %d", path, tc.origin, tc.fetchSite, w.Code)
			}
		}
	}
	var rounds, contacts, enabled int
	for query, target := range map[string]*int{"SELECT count(*) FROM rounds": &rounds, "SELECT count(*) FROM qsos": &contacts, "SELECT map_enabled FROM app_settings WHERE id=1": &enabled} {
		if err := a.store.db.QueryRow(query).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if rounds != 0 || contacts != 0 || enabled != 0 {
		t.Fatalf("rejected request changed data: rounds=%d contacts=%d map=%d", rounds, contacts, enabled)
	}
}
