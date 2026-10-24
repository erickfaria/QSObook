package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestManualCoordinatesOfflineAndExplicitRetry(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	a.store.createContact(ctx, Station{Callsign: "ZZ0MAP", City: "Example", State: "XX", Country: "Exampleland"}, "FM", "")
	var calls atomic.Int32
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if q.Get("city") != "Example" || q.Get("state") != "XX" || q.Get("country") != "Exampleland" || len(q) != 5 {
			t.Error("geographic-only query", q)
		}
		w.Write([]byte(`[{"lat":"10","lon":"20"}]`))
	}))
	defer fake.Close()
	a.geocoderURL = fake.URL
	form := url.Values{"csrf": {a.token}, "city": {"Example"}, "state": {"XX"}, "country": {"Exampleland"}, "action": {"save"}, "latitude": {"1.5"}, "longitude": {"-2.5"}}
	if w := request(a, "GET", "/map/edit?city=Example&state=XX&country=Exampleland", nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(a, "POST", "/map/edit", form); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	if calls.Load() != 0 {
		t.Fatal("manual edit contacted network")
	}
	cities, _ := a.store.contactCities(ctx)
	if cities[0].Latitude.Float64 != 1.5 || cities[0].Longitude.Float64 != -2.5 {
		t.Fatal(cities)
	}
	for _, value := range []string{"NaN", "Inf", "91", "-91"} {
		form.Set("latitude", value)
		if w := request(a, "POST", "/map/edit", form); w.Code != 400 {
			t.Fatal("invalid latitude accepted", value, w.Code)
		}
	}
	form.Set("latitude", "1.5")
	form.Set("action", "retry")
	if w := request(a, "POST", "/map/edit", form); w.Code != 403 {
		t.Fatal("retry without opt-in", w.Code)
	}
	a.store.db.Exec("UPDATE app_settings SET map_enabled=1")
	if w := request(a, "POST", "/map/edit", form); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	cities, _ = a.store.contactCities(ctx)
	if cities[0].Latitude.Float64 != 10 || calls.Load() != 1 {
		t.Fatal("cache not refreshed", cities, calls.Load())
	}
	if w := request(a, "POST", "/map/edit", form); w.Code != 429 || calls.Load() != 1 {
		t.Fatal("cooldown bypassed", w.Code, calls.Load())
	}
	request(a, "GET", "/", nil)
	if calls.Load() != 1 {
		t.Fatal("render geocoded")
	}
	a.store.db.Exec("UPDATE app_settings SET geocode_after=0")
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) }))
	defer empty.Close()
	a.geocoderURL = empty.URL
	if w := request(a, "POST", "/map/edit", form); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	cities, _ = a.store.contactCities(ctx)
	if cities[0].Latitude.Float64 != 10 {
		t.Fatal("no-match erased corrected point")
	}
	a.store.db.Exec("UPDATE app_settings SET geocode_after=0")
	a.geocoderURL = "http://127.0.0.1:1"
	if w := request(a, "POST", "/map/edit", form); w.Code != 502 {
		t.Fatal("provider failure", w.Code, w.Body)
	}
	cities, _ = a.store.contactCities(ctx)
	if cities[0].Latitude.Float64 != 10 {
		t.Fatal("failure erased point")
	}
	form.Set("city", "Arbitrary")
	if w := request(a, "POST", "/map/edit", form); w.Code != 404 {
		t.Fatal("arbitrary city", w.Code)
	}
}
