package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultMapTiles = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"
const defaultGeocoder = "https://nominatim.openstreetmap.org/search"

type ContactCity struct {
	City, State, Country string
	Count                int64
	Latitude, Longitude  sql.NullFloat64
	CheckedAt            sql.NullString
	Groups               []ModeGroup
}

type ModeGroup struct {
	Mode  string `json:"mode"`
	Band  string `json:"band"`
	Count int64  `json:"count"`
}

func (c ContactCity) GroupsJSON() string { data, _ := json.Marshal(c.Groups); return string(data) }
func (s *Store) contactCities(ctx context.Context) ([]ContactCity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT q.city,q.state,q.country,count(*),c.latitude,c.longitude,c.checked_at,q.mode,q.band FROM qsos q LEFT JOIN city_coordinates c ON c.city=q.city AND c.state=q.state AND c.country=q.country WHERE q.city<>'' GROUP BY q.city,q.state,q.country,q.mode,q.band ORDER BY q.city,q.state,q.country,q.mode,q.band`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ContactCity{}
	indices := map[[3]string]int{}
	for rows.Next() {
		var c ContactCity
		var g ModeGroup
		if err := rows.Scan(&c.City, &c.State, &c.Country, &g.Count, &c.Latitude, &c.Longitude, &c.CheckedAt, &g.Mode, &g.Band); err != nil {
			return nil, err
		}
		key := [3]string{c.City, c.State, c.Country}
		index, found := indices[key]
		if !found {
			index = len(list)
			indices[key] = index
			list = append(list, c)
		}
		list[index].Count += g.Count
		list[index].Groups = append(list[index].Groups, g)
	}
	return list, rows.Err()
}

func (s *Store) mapEnabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx, "SELECT map_enabled FROM app_settings WHERE id=1").Scan(&enabled)
	return enabled, err
}

// Public provider requests are user-triggered, serialized across processes
// sharing this database, and separated by at least one second. A short lease
// outlives the HTTP timeout and expires if the process stops unexpectedly.
func (s *Store) claimGeocoding(ctx context.Context) (bool, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, "UPDATE app_settings SET geocode_after=? WHERE id=1 AND map_enabled=1 AND geocode_after<=?", now.Add(15*time.Second).UnixMilli(), now.UnixMilli())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (a *App) configureMap(tiles, geocoder string) error {
	for _, raw := range []string{tiles, geocoder} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
			return errors.New("URL de mapa inválida")
		}
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())) {
			return errors.New("provedores de mapa devem usar HTTPS ou loopback local")
		}
	}
	if !strings.Contains(tiles, "{z}") || !strings.Contains(tiles, "{x}") || !strings.Contains(tiles, "{y}") {
		return errors.New("URL de tiles requer {z}, {x} e {y}")
	}
	a.mapTilesURL = tiles
	a.geocoderURL = geocoder
	return nil
}

func (a *App) mapPreference(w http.ResponseWriter, r *http.Request) {
	value := r.PostForm.Get("enabled")
	if value != "0" && value != "1" {
		a.httpError(w, r, "Preferência inválida", 400)
		return
	}
	if _, err := a.store.db.ExecContext(r.Context(), "UPDATE app_settings SET map_enabled=? WHERE id=1", value == "1"); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/#map-title")
}

func (a *App) locateCity(w http.ResponseWriter, r *http.Request) {
	enabled, err := a.store.mapEnabled(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if !enabled {
		a.httpError(w, r, "Ative o mapa antes de localizar uma cidade", 403)
		return
	}
	c := ContactCity{City: r.PostForm.Get("city"), State: r.PostForm.Get("state"), Country: r.PostForm.Get("country")}
	st := Station{Callsign: "MAP", City: c.City, State: c.State, Country: c.Country}
	if err := st.validate(); err != nil || strings.TrimSpace(c.City) == "" {
		a.httpError(w, r, "Localização inválida", 400)
		return
	}
	var count int
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT count(*) FROM qsos WHERE city=? AND state=? AND country=?", c.City, c.State, c.Country).Scan(&count); err != nil {
		a.fail(w, r, err)
		return
	}
	if count == 0 {
		a.fail(w, r, sql.ErrNoRows)
		return
	}
	err = a.geocodeCity(r.Context(), c)
	if errors.Is(err, errGeocodingBusy) {
		w.Header().Set("Retry-After", "2")
		a.httpError(w, r, "Uma consulta está em andamento. Aguarde antes de tentar outra cidade.", 429)
		return
	}
	if err != nil {
		redirect(w, r, "/?message=Não+foi+possível+localizar+a+cidade.#map-title")
		return
	}
	redirect(w, r, "/#map-title")
}

var errGeocodingBusy = errors.New("geocoding busy")
var errGeocodingUnavailable = errors.New("geocoding unavailable")

func (a *App) geocodeCity(ctx context.Context, c ContactCity) error {
	// The user has saved a city or explicitly requested its location. Never run
	// this during rendering, autocomplete, startup or a background batch.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var cached int
	if err := a.store.db.QueryRowContext(ctx, "SELECT count(*) FROM city_coordinates WHERE city=? AND state=? AND country=?", c.City, c.State, c.Country).Scan(&cached); err != nil {
		return err
	}
	if cached > 0 {
		return nil
	}
	claimed, err := a.store.claimGeocoding(ctx)
	if err != nil {
		return err
	}
	if !claimed {
		return errGeocodingBusy
	}
	cooldown := 1100 * time.Millisecond
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// A failed cleanup leaves only a short lease, never an unbounded lock.
		_, err := a.store.db.ExecContext(ctx, "UPDATE app_settings SET geocode_after=? WHERE id=1", time.Now().Add(cooldown).UnixMilli())
		if err != nil {
			log.Print("QSObook: map request lease cleanup failed; the short lease will expire automatically")
		}
	}()
	u, err := url.Parse(a.geocoderURL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("format", "jsonv2")
	q.Set("limit", "1")
	q.Set("city", c.City)
	q.Set("state", c.State)
	q.Set("country", c.Country)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "QSObook/1 (+https://github.com/erickfaria/QSObook)")
	req.Header.Set("Accept", "application/json")
	response, err := a.geoClient.Do(req)
	if err != nil {
		return errGeocodingUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		cooldown = time.Minute
		if seconds, e := strconv.ParseInt(response.Header.Get("Retry-After"), 10, 32); e == nil && seconds > 0 && seconds <= 86400 {
			cooldown = max(cooldown, time.Duration(seconds)*time.Second)
		}
		if when, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil && time.Until(when) > cooldown {
			cooldown = min(time.Until(when), 24*time.Hour)
		}
		return errGeocodingUnavailable
	}
	var places []struct{ Lat, Lon string }
	if err = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&places); err != nil {
		return errGeocodingUnavailable
	}
	if len(places) > 0 {
		lat, e1 := strconv.ParseFloat(places[0].Lat, 64)
		lon, e2 := strconv.ParseFloat(places[0].Lon, 64)
		if e1 != nil || e2 != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return errGeocodingUnavailable
		}
		c.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
		c.Longitude = sql.NullFloat64{Float64: lon, Valid: true}
	}
	if _, err = a.store.db.ExecContext(ctx, `INSERT INTO city_coordinates(city,state,country,latitude,longitude,checked_at) VALUES(?,?,?,?,?,?) ON CONFLICT(city,state,country) DO NOTHING`, c.City, c.State, c.Country, c.Latitude, c.Longitude, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return nil
}

func (s *Store) contactsInCity(ctx context.Context, c ContactCity, query string, filters ...string) ([]Contact, error) {
	mode, band := "", ""
	if len(filters) > 0 {
		mode = filters[0]
	}
	if len(filters) > 1 {
		band = filters[1]
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+contactColumns+` FROM qsos WHERE city=? AND state=? AND country=? AND (instr(callsign,upper(?))>0 OR instr(lower(name),lower(?))>0)
 AND (?='' OR upper(mode)=upper(?) OR (?='digital' AND upper(mode) IN ('DIGITAL','DMR','ECHOLINK','D-STAR','C4FM')) OR (?='other' AND upper(mode) NOT IN ('FM','AM','SSB','CW','DIGITAL','DMR','ECHOLINK','D-STAR','C4FM')))
 AND (?='' OR band=?) ORDER BY occurred_at DESC,id DESC LIMIT 200`, c.City, c.State, c.Country, query, query, mode, mode, mode, mode, band, band)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Contact{}
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
