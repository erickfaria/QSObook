package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mapTestContact(t *testing.T, a *App) url.Values {
	t.Helper()
	_, err := a.store.createContact(context.Background(), Station{Callsign: "ZZ0PRIVATE", Name: "Private name", City: "Exampletown", State: "EX", Country: "Exampleland", Notes: "Private permanent note"}, "FM", "Private conversation")
	if err != nil {
		t.Fatal(err)
	}
	return url.Values{"csrf": {a.token}, "city": {"Exampletown"}, "state": {"EX"}, "country": {"Exampleland"}}
}

func TestMapRequiresOptInAndCachesOnlyGeographicData(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	form := mapTestContact(t, a)
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if len(q) != 5 || q.Get("city") != "Exampletown" || q.Get("state") != "EX" || q.Get("country") != "Exampleland" || q.Get("format") != "jsonv2" || q.Get("limit") != "1" {
			t.Errorf("unexpected data sent: %s", r.URL.RawQuery)
		}
		if r.Header.Get("User-Agent") != "QSObook/1 (+https://github.com/erickfaria/QSObook)" {
			t.Error("missing provider identification")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"lat":"-20.5","lon":"-40.2"}]`))
	}))
	defer provider.Close()
	if err := a.configureMap(defaultMapTiles, provider.URL+"/search"); err != nil {
		t.Fatal(err)
	}
	if err := saveTestParticipant(t, a.store, ctx, Station{Callsign: "ZZ1UNCONFIRMED", City: "Other town"}, "DMR", ""); err != nil {
		t.Fatal(err)
	}
	w := request(a, "GET", "/", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "leaflet.js") || strings.Contains(w.Header().Get("Content-Security-Policy"), "https://tile.openstreetmap.org") {
		t.Fatal("map loads before opt-in")
	}
	if w = request(a, "POST", "/map/locate", form); w.Code != 403 || calls.Load() != 0 {
		t.Fatal("geocoding allowed before opt-in")
	}
	if w = request(a, "POST", "/map/preference", url.Values{"csrf": {a.token}, "enabled": {"1"}}); w.Code != 303 {
		t.Fatalf("enable: %d", w.Code)
	}
	w = request(a, "GET", "/", nil)
	if !strings.Contains(w.Body.String(), "leaflet.js") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "img-src 'self' https://tile.openstreetmap.org") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || w.Header().Get("X-Frame-Options") != "DENY" || w.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("incorrect map/security headers")
	}
	if calls.Load() != 0 {
		t.Fatal("opening the map triggered geocoding")
	}
	for i := 0; i < 3; i++ {
		if w = request(a, "POST", "/map/locate", form); w.Code != 303 {
			t.Fatalf("locate: %d %s", w.Code, w.Body)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("repeated geocoding: %d", calls.Load())
	}
	cities, err := a.store.contactCities(ctx)
	if err != nil || len(cities) != 1 || cities[0].City != "Exampletown" || cities[0].Count != 1 || !cities[0].Latitude.Valid || cities[0].Latitude.Float64 != -20.5 {
		t.Fatalf("map mixed participants or lost coordinates: %+v %v", cities, err)
	}
	if w = request(a, "POST", "/map/preference", url.Values{"csrf": {a.token}, "enabled": {"0"}}); w.Code != 303 {
		t.Fatal("disable failed")
	}
	if w = request(a, "POST", "/map/locate", form); w.Code != 403 || calls.Load() != 1 {
		t.Fatal("disabled map used provider")
	}
	cities, _ = a.store.contactCities(ctx)
	if !cities[0].Latitude.Valid {
		t.Fatal("disabling map discarded local coordinates")
	}
	w = request(a, "GET", "/history?city=Exampletown&state=EX&country=Exampleland&q=ZZ0PRIVATE", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Private name") {
		t.Fatal("city history missing")
	}
	w = request(a, "GET", "/history?city=Exampletown&state=EX&country=Exampleland&q=ZZ9MISSING", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "Private name") {
		t.Fatal("city history ignores search")
	}
	if w = request(a, "POST", "/map/preference", url.Values{"enabled": {"1"}}); w.Code != 403 {
		t.Fatal("map preference lacks CSRF")
	}
}

func TestGeocodingLeasePersistsAcrossClients(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if ok, err := s.claimGeocoding(ctx); err != nil || ok {
		t.Fatalf("disabled geocoding lease: %t %v", ok, err)
	}
	if _, err := s.db.Exec("UPDATE app_settings SET map_enabled=1"); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.claimGeocoding(ctx); err != nil || !ok {
		t.Fatalf("first lease: %t %v", ok, err)
	}
	if ok, err := s.claimGeocoding(ctx); err != nil || ok {
		t.Fatalf("overlapping lease: %t %v", ok, err)
	}
}

func TestMapProviderFailuresAndNegativeCache(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		cached     bool
	}{
		{"not found", "[]", 200, true},
		{"bad coordinates", `[{"lat":"NaN","lon":"-40"}]`, 200, false},
		{"out of range", `[{"lat":"91","lon":"-40"}]`, 200, false},
		{"malformed", "not json", 200, false},
		{"rate limited", "", 429, false},
		{"offline", "", 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			form := mapTestContact(t, a)
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "120")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer provider.Close()
			if err := a.configureMap(defaultMapTiles, provider.URL); err != nil {
				t.Fatal(err)
			}
			request(a, "POST", "/map/preference", url.Values{"csrf": {a.token}, "enabled": {"1"}})
			if w := request(a, "POST", "/map/locate", form); w.Code != 303 {
				t.Fatalf("failure response: %d %s", w.Code, w.Body)
			}
			cities, err := a.store.contactCities(context.Background())
			if err != nil || len(cities) != 1 || cities[0].Latitude.Valid || cities[0].CheckedAt.Valid != tc.cached {
				t.Fatalf("invalid cache: %+v %v", cities, err)
			}
			contacts, _ := a.store.contacts(context.Background(), "")
			if len(contacts) != 1 || contacts[0].Notes != "Private conversation" {
				t.Fatal("provider failure damaged contact")
			}
			if tc.cached {
				request(a, "POST", "/map/locate", form)
				if calls.Load() != 1 {
					t.Fatal("negative result not cached")
				}
			}
			if tc.status != 200 {
				var until int64
				if err := a.store.db.QueryRow("SELECT geocode_after FROM app_settings").Scan(&until); err != nil {
					t.Fatal(err)
				}
				if time.Until(time.UnixMilli(until)) < 119*time.Second {
					t.Fatal("Retry-After was ignored")
				}
			}
		})
	}
}

func TestMapConfigurationRejectsUnsafeURLs(t *testing.T) {
	a := testApp(t)
	for _, raw := range []string{"javascript:alert(1)", "http://external.example/search", "https://user:pass@example.test/search", "https://example.test/search#fragment"} {
		if err := a.configureMap(defaultMapTiles, raw); err == nil {
			t.Fatalf("accepted provider %q", raw)
		}
	}
}
