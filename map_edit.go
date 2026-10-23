package main

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (a *App) editableCity(r *http.Request) (ContactCity, error) {
	c := ContactCity{City: r.FormValue("city"), State: r.FormValue("state"), Country: r.FormValue("country")}
	st := Station{Callsign: "MAP", City: c.City, State: c.State, Country: c.Country}
	if err := st.validate(); err != nil || strings.TrimSpace(c.City) == "" {
		return c, errors.New("Localização inválida")
	}
	var count int
	if err := a.store.db.QueryRowContext(r.Context(), `SELECT (SELECT count(*) FROM qsos WHERE deleted_at IS NULL AND city=? AND state=? AND country=?)+(SELECT count(*) FROM app_settings WHERE home_city=? AND home_state=? AND home_country=?)`, c.City, c.State, c.Country, c.City, c.State, c.Country).Scan(&count); err != nil {
		return c, err
	}
	if count == 0 {
		return c, sql.ErrNoRows
	}
	err := a.store.db.QueryRowContext(r.Context(), "SELECT latitude,longitude,checked_at FROM city_coordinates WHERE city=? AND state=? AND country=?", c.City, c.State, c.Country).Scan(&c.Latitude, &c.Longitude, &c.CheckedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return c, err
}
func (a *App) mapEdit(w http.ResponseWriter, r *http.Request) {
	c, err := a.editableCity(r)
	if err != nil {
		if err.Error() == "Localização inválida" {
			a.httpError(w, r, err.Error(), 400)
		} else {
			a.fail(w, r, err)
		}
		return
	}
	a.render(w, r, Page{Title: "Corrigir localização", View: "map-edit", CityFilter: c})
}
func (a *App) saveMapEdit(w http.ResponseWriter, r *http.Request) {
	c, err := a.editableCity(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	action := r.PostForm.Get("action")
	switch action {
	case "retry":
		enabled, e := a.store.mapEnabled(r.Context())
		if e != nil {
			a.fail(w, r, e)
			return
		}
		if !enabled {
			a.httpError(w, r, "Ative o mapa antes de localizar uma cidade", 403)
			return
		}
		err = a.geocodeCity(r.Context(), c, true)
		if errors.Is(err, errGeocodingBusy) {
			w.Header().Set("Retry-After", "2")
			a.httpError(w, r, "Uma consulta está em andamento. Aguarde antes de tentar outra cidade.", 429)
			return
		}
		if err != nil {
			a.httpError(w, r, "Não foi possível localizar a cidade. A posição anterior foi preservada.", 502)
			return
		}
	case "save":
		lat, e1 := strconv.ParseFloat(r.PostForm.Get("latitude"), 64)
		lon, e2 := strconv.ParseFloat(r.PostForm.Get("longitude"), 64)
		if e1 != nil || e2 != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			a.httpError(w, r, "Coordenadas inválidas. Use latitude de -90 a 90 e longitude de -180 a 180.", 400)
			return
		}
		_, err = a.store.db.ExecContext(r.Context(), `INSERT INTO city_coordinates(city,state,country,latitude,longitude,checked_at) VALUES(?,?,?,?,?,?) ON CONFLICT(city,state,country) DO UPDATE SET latitude=excluded.latitude,longitude=excluded.longitude,checked_at=excluded.checked_at`, c.City, c.State, c.Country, lat, lon, time.Now().UTC().Format(time.RFC3339Nano))
	default:
		a.httpError(w, r, "Ação inválida", 400)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/map/edit?city="+url.QueryEscape(c.City)+"&state="+url.QueryEscape(c.State)+"&country="+url.QueryEscape(c.Country))
}
