package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func recordKey(t *testing.T, a *App) string {
	t.Helper()
	w := request(a, "GET", "/records", nil)
	match := regexp.MustCompile(`name="submission_key" value="([a-f0-9]{32})"`).FindStringSubmatch(w.Body.String())
	if w.Code != 200 || len(match) != 2 {
		t.Fatalf("record form: %d %s", w.Code, w.Body)
	}
	return match[1]
}
func TestRecordsHTTPConfirmationDefaultsAndValidation(t *testing.T) {
	a := testApp(t)
	w := request(a, "GET", "/records", nil)
	if strings.Contains(w.Body.String(), `name="confirmed" value="1" checked`) || strings.Contains(w.Body.String(), `value="Brasil" selected`) {
		t.Fatal("country or confirmation defaulted")
	}
	form := url.Values{"csrf": {a.token}, "submission_key": {recordKey(t, a)}, "callsign": {"ZZ0FORM"}, "name": {"Example"}, "country": {"Brasil"}, "city": {"Exampletown"}, "operator_notes": {"Permanent"}}
	if w = request(a, "POST", "/records", form); w.Code != 303 {
		t.Fatalf("registration: %d %s", w.Code, w.Body)
	}
	form.Set("submission_key", recordKey(t, a))
	form.Set("confirmed", "1")
	form.Set("notes", "Conversation")
	form.Set("band", "__other")
	form.Set("band_other", "Example band")
	if w = request(a, "POST", "/records", form); w.Code != 400 || !strings.Contains(w.Body.String(), "Conversation") {
		t.Fatal("missing mode lost draft")
	}
	form.Set("mode", "__other")
	form.Set("mode_other", "Example digital")
	form.Set("in_round", "1")
	form.Set("round_name", "Example net")
	for i := 0; i < 2; i++ {
		if w = request(a, "POST", "/records", form); w.Code != 303 {
			t.Fatalf("confirm: %d %s", w.Code, w.Body)
		}
	}
	contacts, err := a.store.contacts(context.Background(), "")
	if err != nil || len(contacts) != 1 || contacts[0].Mode != "Example digital" || contacts[0].Band != "Example band" || contacts[0].RoundName != "Example net" {
		t.Fatalf("contacts: %+v %v", contacts, err)
	}
	st, _ := a.store.lookup(context.Background(), "ZZ0FORM")
	if st.Notes != "Permanent" {
		t.Fatal("operator notes changed")
	}
	if _, err = a.store.db.Exec("UPDATE app_settings SET map_enabled=1"); err != nil {
		t.Fatal(err)
	}
	if home := request(a, "GET", "/", nil); home.Code != 200 || !strings.Contains(home.Body.String(), "<option>Example band</option>") {
		t.Fatal("custom band missing from map filters")
	}
	a.store.db.Close()
	if w = request(a, "POST", "/records", form); w.Code != 500 || strings.Contains(w.Body.String(), "sql:") {
		t.Fatal("database error exposed")
	}
}
func TestGeocodingOnSaveNeedsOptInAndNeverRunsOnDashboard(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if len(q) != 5 || q.Get("city") != "Exampletown" || q.Get("country") != "Brasil" {
			t.Errorf("non-geographic query %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"lat":"-10","lon":"-45"}]`))
	}))
	defer provider.Close()
	if err := a.configureMap("http://127.0.0.1:1/{z}/{x}/{y}.png", provider.URL); err != nil {
		t.Fatal(err)
	}
	location := url.Values{"csrf": {a.token}, "city": {"Exampletown"}, "state": {"EX"}, "country": {"Brasil"}}
	if w := request(a, "POST", "/location", location); w.Code != 303 {
		t.Fatal(w.Code)
	}
	if calls.Load() != 0 {
		t.Fatal("geocoding without consent")
	}
	location.Set("map_enabled", "1")
	location.Set("auto_geocode", "1")
	if w := request(a, "POST", "/location", location); w.Code != 303 {
		t.Fatalf("home: %d %s", w.Code, w.Body)
	}
	home, err := a.store.homeLocation(ctx)
	if err != nil || !home.Latitude.Valid || home.Latitude.Float64 != -10 {
		t.Fatalf("home not saved: %+v %v", home, err)
	}
	form := url.Values{"csrf": {a.token}, "submission_key": {recordKey(t, a)}, "callsign": {"ZZ0AUTO"}, "name": {"Private name"}, "city": {"Exampletown"}, "state": {"EX"}, "country": {"Brasil"}, "mode": {"DMR"}, "band": {"70 cm"}, "confirmed": {"1"}, "notes": {"Private conversation"}}
	if w := request(a, "POST", "/records", form); w.Code != 303 {
		t.Fatalf("contact: %d %s", w.Code, w.Body)
	}
	for i := 0; i < 3; i++ {
		w := request(a, "GET", "/", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `data-home-lat="-10"`) {
			t.Fatal("home marker missing")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("cache/render repeated geocoding: %d", calls.Load())
	}
	cities, err := a.store.contactCities(ctx)
	if err != nil || len(cities) != 1 || len(cities[0].Groups) != 1 || cities[0].Groups[0].Band != "70 cm" {
		t.Fatalf("map groups: %+v %v", cities, err)
	}
	form.Set("city", "Another city")
	form.Set("submission_key", recordKey(t, a))
	form.Set("callsign", "ZZ1AUTO")
	// The active cooldown must not prevent saving the next contact.
	if w := request(a, "POST", "/records", form); w.Code != 303 {
		t.Fatal("cooldown lost contact")
	}
	contacts, _ := a.store.contacts(ctx, "")
	if len(contacts) != 2 || calls.Load() != 1 {
		t.Fatal("cooldown duplicated lookup or lost data")
	}
}
func TestNewRoutesKeepForgeryProtection(t *testing.T) {
	a := testApp(t)
	before := totalChanges(t, a.store)
	for _, path := range []string{"/records", "/location"} {
		if w := request(a, "POST", path, url.Values{"city": {"Changed"}}); w.Code != 403 {
			t.Fatalf("%s bypassed CSRF", path)
		}
	}
	if totalChanges(t, a.store) != before {
		t.Fatal("forgery changed data")
	}
}

func TestMonthlyCountUsesConfiguredTimeZone(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	now := time.Now().In(a.location)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, a.location)
	for _, stamp := range []time.Time{start.Add(-time.Nanosecond), start, start.AddDate(0, 1, 0)} {
		id, err := a.store.createContact(ctx, Station{Callsign: "ZZ0MONTH"}, "FM", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.store.db.Exec("UPDATE qsos SET occurred_at=? WHERE id=?", stamp.UTC().Format("2006-01-02T15:04:05.000000000Z"), id); err != nil {
			t.Fatal(err)
		}
	}
	w := request(a, "GET", "/", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<strong>1</strong><span>Contatos neste mês") || !strings.Contains(w.Body.String(), "<strong>3</strong><span>No caderno") {
		t.Fatal("monthly boundary count incorrect")
	}
}
func TestCountryChoicesPreserveLegacyValuesWithoutDefaults(t *testing.T) {
	a := testApp(t)
	for _, language := range []string{"pt-BR", "en", "es", "fr"} {
		countries := a.countryChoices(language, "")
		if len(countries) < 240 {
			t.Fatal("incomplete countries")
		}
		for _, option := range countries {
			if option.Selected {
				t.Fatal("country selected by default")
			}
		}
		found := false
		for _, option := range a.countryChoices(language, "Legacy country") {
			found = found || (option.Selected && option.Value == "Legacy country")
		}
		if !found {
			t.Fatal("legacy country lost")
		}
	}
}
