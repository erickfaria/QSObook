package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
)

type HomeLocation struct {
	City, State, Country    string
	Latitude, Longitude     sql.NullFloat64
	MapEnabled, AutoGeocode bool
}

func (s *Store) homeLocation(ctx context.Context) (HomeLocation, error) {
	var home HomeLocation
	err := s.db.QueryRowContext(ctx, `SELECT a.home_city,a.home_state,a.home_country,c.latitude,c.longitude,a.map_enabled,a.auto_geocode FROM app_settings a LEFT JOIN city_coordinates c ON c.city=a.home_city AND c.state=a.home_state AND c.country=a.home_country WHERE a.id=1`).Scan(&home.City, &home.State, &home.Country, &home.Latitude, &home.Longitude, &home.MapEnabled, &home.AutoGeocode)
	return home, err
}
func (a *App) locationForm(w http.ResponseWriter, r *http.Request) {
	home, err := a.store.homeLocation(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, Page{Title: "Meu local", View: "location", Action: "/location", Home: home, Station: Station{City: home.City, State: home.State, Country: home.Country}, Message: r.URL.Query().Get("message")})
}
func (a *App) saveLocation(w http.ResponseWriter, r *http.Request) {
	st := readStation(r)
	st.Callsign = "HOME"
	if err := st.validate(); err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	for _, field := range []string{"map_enabled", "auto_geocode"} {
		if value := r.PostForm.Get(field); value != "" && value != "1" {
			a.httpError(w, r, "Preferência inválida", 400)
			return
		}
	}
	enabled, automatic := r.PostForm.Get("map_enabled") == "1", r.PostForm.Get("auto_geocode") == "1"
	if _, err := a.store.db.ExecContext(r.Context(), "UPDATE app_settings SET home_city=?,home_state=?,home_country=?,map_enabled=?,auto_geocode=? WHERE id=1", st.City, st.State, st.Country, enabled, automatic && enabled); err != nil {
		a.fail(w, r, err)
		return
	}
	message := "Local salvo"
	if err := a.autoLocate(r.Context(), ContactCity{City: st.City, State: st.State, Country: st.Country}); err != nil {
		message = "Registro salvo. Cidade pendente de localização no mapa."
	}
	redirect(w, r, "/location?message="+url.QueryEscape(message))
}
func (a *App) autoLocate(ctx context.Context, city ContactCity) error {
	home, err := a.store.homeLocation(ctx)
	if err != nil {
		return err
	}
	if !home.MapEnabled || !home.AutoGeocode || strings.TrimSpace(city.City) == "" {
		return nil
	}
	return a.geocodeCity(ctx, city)
}
