package main

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html static/* locales/*.json LICENSE docs/THIRD_PARTY_NOTICES.txt
var assets embed.FS

type App struct {
	requests                 sync.RWMutex
	transfers                transferState
	allowedHosts             map[string]bool
	passwordHash             *[32]byte
	location                 *time.Location
	store                    *Store
	templates                map[string]*template.Template
	translations             map[string]map[string]string
	countries                map[string]map[string]string
	token                    string
	mux                      *http.ServeMux
	mapTilesURL, geocoderURL string
	geoClient                *http.Client
}
type Page struct {
	Version, Revision                                string
	Schema                                           int
	Language, ReturnTo, JSMessages                   string
	Title, View, Token, Query, Message, Action, Back string
	Stations                                         []Station
	Contacts                                         []Contact
	Participants                                     []Participant
	Station                                          Station
	Contact                                          Contact
	Mode                                             string
	Band, RoundName, SubmissionKey                   string
	Confirmed                                        bool
	Home                                             HomeLocation
	MonthCount                                       int64
	Notes                                            string
	IsContact                                        bool
	Round                                            Round
	Rounds                                           []Round
	PreviousRound                                    int64
	ParticipantID                                    int64
	RoundID                                          int64
	ContactCount                                     int64
	MapEnabled                                       bool
	MapTilesURL                                      string
	MapCities                                        []ContactCity
	CityFilter                                       ContactCity
	Filter                                           ListFilter
	Pagination                                       Pagination
	Trash                                            []TrashItem
	Summary                                          NotebookSummary
	RecoveryFiles                                    []string
	Transfer                                         *transferPreview
}

func newApp(store *Store, loc *time.Location) (*App, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	translations, err := readTranslations()
	if err != nil {
		return nil, err
	}
	a := &App{store: store, location: loc, translations: translations, token: hex.EncodeToString(secret), mux: http.NewServeMux(), mapTilesURL: defaultMapTiles, geocoderURL: defaultGeocoder,
		geoClient: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	countryData, err := assets.ReadFile("locales/countries.json")
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(countryData, &a.countries); err != nil {
		return nil, err
	}
	a.templates, err = a.localizedTemplates(loc)
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	a.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	a.mux.HandleFunc("GET /{$}", a.dashboard)
	a.mux.HandleFunc("POST /language", a.languagePreference)
	a.mux.HandleFunc("GET /records", a.records)
	a.mux.HandleFunc("POST /records", a.saveRecord)
	a.mux.HandleFunc("GET /about", a.about)
	a.mux.HandleFunc("GET /map/edit", a.mapEdit)
	a.mux.HandleFunc("POST /map/edit", a.saveMapEdit)
	a.mux.HandleFunc("GET /data", a.dataPage)
	a.mux.HandleFunc("POST /data/export", a.downloadADIF)
	a.mux.HandleFunc("POST /data/backup", a.downloadBackup)
	a.mux.HandleFunc("POST /data/recovery", a.downloadRecovery)
	a.mux.HandleFunc("POST /data/restore", a.previewRestore)
	a.mux.HandleFunc("POST /data/import", a.previewImport)
	a.mux.HandleFunc("POST /data/confirm", a.confirmTransfer)
	a.mux.HandleFunc("POST /data/cancel", a.cancelTransfer)
	a.mux.HandleFunc("GET /trash", a.trash)
	a.mux.HandleFunc("GET /trash/confirm", a.trashConfirm)
	a.mux.HandleFunc("POST /trash", a.trashAction)
	a.mux.HandleFunc("GET /location", a.locationForm)
	a.mux.HandleFunc("POST /location", a.saveLocation)
	a.mux.HandleFunc("GET /round", a.round)
	a.mux.HandleFunc("POST /map/preference", a.mapPreference)
	a.mux.HandleFunc("POST /map/locate", a.locateCity)
	a.mux.HandleFunc("POST /round/start", a.startRound)
	a.mux.HandleFunc("POST /round/end", a.endRound)
	a.mux.HandleFunc("POST /participant/status", a.participantStatus)
	a.mux.HandleFunc("GET /stations", a.stations)
	a.mux.HandleFunc("GET /station", a.stationForm)
	a.mux.HandleFunc("POST /station", a.saveStation)
	a.mux.HandleFunc("GET /history", a.history)
	a.mux.HandleFunc("GET /contact", a.contactForm)
	a.mux.HandleFunc("POST /contact", a.saveContact)
	a.mux.HandleFunc("GET /participant", a.participantForm)
	a.mux.HandleFunc("POST /participant", a.saveParticipant)
	a.mux.HandleFunc("POST /participant/remove", a.removeParticipant)
	a.mux.HandleFunc("POST /participant/record", a.recordParticipant)
	a.mux.HandleFunc("GET /lookup", a.lookup)
	a.mux.HandleFunc("GET /third-party-licenses", func(w http.ResponseWriter, r *http.Request) {
		data, err := assets.ReadFile("docs/THIRD_PARTY_NOTICES.txt")
		if err != nil {
			a.fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})
	a.mux.HandleFunc("GET /license", func(w http.ResponseWriter, r *http.Request) {
		data, err := assets.ReadFile("LICENSE")
		if err != nil {
			a.fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})
	return a, nil
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serialize replacement against all in-flight HTTP operations and token reads.
	if r.Method == "POST" && (r.URL.Path == "/data/confirm" || r.URL.Path == "/data/restore" || r.URL.Path == "/data/import") {
		a.requests.Lock()
		defer a.requests.Unlock()
	} else {
		a.requests.RLock()
		defer a.requests.RUnlock()
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Keep browser form Origins intact; cross-origin requests still omit Referer.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	if !a.acceptsHost(r.Host) {
		a.httpError(w, r, "Host não permitido", http.StatusForbidden)
		return
	}
	if !a.authenticate(w, r) {
		return
	}
	if r.Method == http.MethodPost {
		limit := int64(32 << 10)
		upload := r.URL.Path == "/data/restore" || r.URL.Path == "/data/import"
		if upload {
			limit = maxUpload + (1 << 20)
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || u.Scheme != "http" {
				a.httpError(w, r, "Origem não permitida", 403)
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			a.httpError(w, r, "Origem não permitida", 403)
			return
		}
		var parseErr error
		if upload {
			parseErr = r.ParseMultipartForm(128 << 10)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			parseErr = r.ParseForm()
		}
		if parseErr != nil {
			a.httpError(w, r, "Formulário inválido ou muito grande", 400)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(a.token)) != 1 {
			a.httpError(w, r, "Formulário expirado. Recarregue a página.", 403)
			return
		}
	}
	a.mux.ServeHTTP(w, r)
}
func (a *App) render(w http.ResponseWriter, r *http.Request, p Page) {
	p.Token = a.token
	p.Language = requestLanguage(r)
	p.ReturnTo = r.URL.RequestURI()
	if r.Method == http.MethodPost {
		p.ReturnTo = p.Action
		id := p.Station.ID
		if p.IsContact {
			id = p.Contact.ID
		}
		if p.Action == "/participant" {
			id = p.ParticipantID
			if id == 0 {
				p.ReturnTo = "/round"
			}
		}
		if id > 0 {
			p.ReturnTo += "?id=" + strconv.FormatInt(id, 10)
		}
	}
	p.Title = a.translate(p.Language, p.Title)
	p.Message = a.translate(p.Language, p.Message)
	messages := make(map[string]string, len(browserMessages()))
	for _, key := range browserMessages() {
		messages[key] = a.translate(p.Language, key)
	}
	encoded, _ := json.Marshal(messages) // A map of strings cannot fail JSON encoding.
	p.JSMessages = string(encoded)
	w.Header().Set("Content-Language", p.Language)
	if p.MapEnabled {
		u, err := url.Parse(a.mapTilesURL)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		p.MapTilesURL = a.mapTilesURL
		w.Header().Set("Content-Security-Policy", strings.Replace(w.Header().Get("Content-Security-Policy"), "img-src 'self'", "img-src 'self' "+u.Scheme+"://"+u.Host, 1))
	}
	var b bytes.Buffer
	if err := a.templates[p.Language].ExecuteTemplate(&b, "layout.html", p); err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b.Bytes())
}
func (a *App) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errStationInTrash) {
		a.httpError(w, r, err.Error(), 409)
		return
	}
	if errors.Is(err, errRoundChanged) {
		a.httpError(w, r, err.Error(), http.StatusConflict)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		a.httpError(w, r, "Registro não encontrado", 404)
		return
	}
	a.httpError(w, r, "Não foi possível concluir a operação. Tente novamente.", 500)
}
func identifier(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(r.FormValue(key), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("identificador inválido")
	}
	return id, nil
}
func readStation(r *http.Request) Station {
	return Station{Callsign: r.PostForm.Get("callsign"), Name: r.PostForm.Get("name"), City: r.PostForm.Get("city"), State: r.PostForm.Get("state"), Country: r.PostForm.Get("country"), Notes: r.PostForm.Get("notes")}
}
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}
func (a *App) stations(w http.ResponseWriter, r *http.Request) {
	f, err := a.listFilter(r)
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	list, total, err := a.store.stationPage(r.Context(), f)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, Page{Title: "Estações", View: "stations", Stations: list, Query: f.Query, Filter: f, Pagination: pagination(r, f.Page, total)})
}
func (a *App) stationForm(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "Cadastrar estação", View: "form", Action: "/station", Back: "/stations"}
	if r.FormValue("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		p.Station, err = a.store.station(r.Context(), id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		p.Title = "Editar estação"
	}
	a.render(w, r, p)
}
func (a *App) saveStation(w http.ResponseWriter, r *http.Request) {
	st := readStation(r)
	if err := st.validate(); err != nil {
		a.invalidForm(w, r, st, err)
		return
	}
	if r.PostForm.Get("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		existing, err := a.store.station(r.Context(), id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		st.ID = id
		st.Callsign = existing.Callsign
	}
	id, err := a.store.saveStation(r.Context(), st)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/station?id="+strconv.FormatInt(id, 10))
}
func (a *App) invalidForm(w http.ResponseWriter, r *http.Request, st Station, validationErr error) {
	p := Page{Title: "Revise os dados", View: "form", Action: r.URL.Path, Station: st, Mode: r.PostForm.Get("mode"), Band: recordChoice(r, "band"), RoundName: r.PostForm.Get("round_name"), Notes: r.PostForm.Get("notes"), Message: validationErr.Error(), Back: "/", IsContact: r.URL.Path == "/contact"}
	if r.PostForm.Get("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		if p.IsContact {
			p.Contact.ID = id
		} else if r.URL.Path == "/participant" {
			p.ParticipantID = id
		} else {
			p.Station.ID = id
		}
	}
	if r.URL.Path == "/participant" {
		id, err := identifier(r, "round_id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		p.Round, err = a.store.round(r.Context(), id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		if p.Round.EndedAt.Valid {
			a.fail(w, r, errRoundChanged)
			return
		}
		p.Back = "/round"
		p.RoundID = id
		if p.ParticipantID == 0 {
			p.Participants, err = a.store.participantsInRound(r.Context(), id)
			if err != nil {
				a.fail(w, r, err)
				return
			}
			p.Title = "Modo QSO"
			p.View = "round"
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Language", requestLanguage(r))
	w.WriteHeader(http.StatusBadRequest)
	a.render(w, r, p)
}

func (a *App) history(w http.ResponseWriter, r *http.Request) {
	f, err := a.listFilter(r)
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	list, total, err := a.store.contactPage(r.Context(), f)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, Page{Title: "Caderno de contatos", View: "history", Contacts: list, Query: f.Query, Filter: f, CityFilter: ContactCity{City: f.City, State: f.State, Country: f.Country}, Pagination: pagination(r, f.Page, total)})
}
func (a *App) contactForm(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("id") == "" {
		a.records(w, r)
		return
	}
	p := Page{Title: "Registrar QSO", View: "form", IsContact: true, Mode: "FM", Action: "/contact", Back: "/history"}
	if r.FormValue("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		p.Contact, err = a.store.contact(r.Context(), id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		p.Station = p.Contact.Station
		p.Mode = p.Contact.Mode
		p.Band = p.Contact.Band
		p.RoundName = p.Contact.RoundName
		p.Notes = p.Contact.Notes
		p.Title = "Detalhes e edição do QSO"
	}
	a.render(w, r, p)
}
func (a *App) saveContact(w http.ResponseWriter, r *http.Request) {
	st := readStation(r)
	mode := strings.TrimSpace(r.PostForm.Get("mode"))
	if err := st.validate(); err != nil {
		a.invalidForm(w, r, st, err)
		return
	}
	if err := validateMode(mode); err != nil {
		a.invalidForm(w, r, st, err)
		return
	}
	if r.PostForm.Get("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		old, err := a.store.contact(r.Context(), id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		st.Callsign = old.Callsign
		old.Station = st
		old.Mode = mode
		if r.PostForm.Has("band") {
			old.Band = recordChoice(r, "band")
		}
		if r.PostForm.Has("round_name") {
			old.RoundName = r.PostForm.Get("round_name")
		}
		if err = a.store.updateContact(r.Context(), old); err != nil {
			a.fail(w, r, err)
			return
		}
		redirect(w, r, "/contact?id="+strconv.FormatInt(id, 10))
		return
	}
	qid, err := a.store.createContact(r.Context(), st, mode, st.Notes)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/contact?id="+strconv.FormatInt(qid, 10))
}
func (a *App) saveParticipant(w http.ResponseWriter, r *http.Request) {
	roundID, err := identifier(r, "round_id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	st := readStation(r)
	var participantID int64
	if r.PostForm.Get("id") != "" {
		participantID, err = identifier(r, "id")
		if err != nil {
			a.httpError(w, r, err.Error(), 400)
			return
		}
		err = a.store.db.QueryRowContext(r.Context(), `SELECT s.id,s.callsign FROM participants p JOIN stations s ON s.id=p.station_id JOIN rounds r ON r.id=p.round_id WHERE p.id=? AND p.round_id=? AND p.removed_at IS NULL AND r.ended_at IS NULL`, participantID, roundID).Scan(&st.ID, &st.Callsign)
		if err != nil {
			a.fail(w, r, err)
			return
		}
	}
	mode := strings.TrimSpace(r.PostForm.Get("mode"))
	if err := st.validate(); err != nil {
		a.invalidForm(w, r, st, err)
		return
	}
	if err := validateMode(mode); err != nil {
		a.invalidForm(w, r, st, err)
		return
	}
	if err := validateNotes(st.Notes); err != nil {
		a.invalidForm(w, r, st, err)
		return
	}
	if err := a.store.saveParticipant(r.Context(), roundID, participantID, st, mode, st.Notes, recordChoice(r, "band")); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/round?message=Participante+salvo")
}
func (a *App) removeParticipant(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	if err = a.store.removeParticipant(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/round?message=Participante+removido+da+lista")
}
func (a *App) recordParticipant(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	if err = a.store.recordParticipant(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/round?message=QSO+registrado+no+caderno")
}
func (a *App) lookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("callsign")
	if len(q) > 32 {
		a.httpError(w, r, "Indicativo muito longo", 400)
		return
	}
	st, err := a.store.lookup(r.Context(), q)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var count int
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT count(*) FROM qsos WHERE deleted_at IS NULL AND station_id=?", st.ID).Scan(&count); err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Station
		Contacts int `json:"contacts"`
	}{st, count})
}

func (a *App) participantForm(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	list, err := a.store.participants(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	for _, p := range list {
		if p.ID == id {
			st := p.Station
			a.render(w, r, Page{Title: "Editar participante", View: "form", Action: "/participant", Back: "/round", Station: st, Mode: p.Mode, Band: p.Band, Notes: p.Notes, ParticipantID: p.ID, RoundID: p.RoundID, Round: Round{ID: p.RoundID}})
			return
		}
	}
	a.fail(w, r, sql.ErrNoRows)
}
