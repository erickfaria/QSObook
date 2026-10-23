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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.html static/* LICENSE docs/THIRD_PARTY_NOTICES.txt
var assets embed.FS

type App struct {
	store     *Store
	templates *template.Template
	token     string
	mux       *http.ServeMux
}
type Page struct {
	Title, View, Token, Query, Message, Action, Back string
	Stations                                         []Station
	Contacts                                         []Contact
	Participants                                     []Participant
	Station                                          Station
	Contact                                          Contact
	Mode                                             string
	Notes                                            string
	IsContact                                        bool
	Round                                            Round
	Rounds                                           []Round
	PreviousRound                                    int64
	ParticipantID                                    int64
	RoundID                                          int64
	ContactCount                                     int64
}

func newApp(store *Store, loc *time.Location) (*App, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	t, err := template.New("layout.html").Funcs(template.FuncMap{"localTime": func(t time.Time) string { return t.In(loc).Format("02/01/2006 15:04:05 MST") }}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	a := &App{store: store, templates: t, token: hex.EncodeToString(secret), mux: http.NewServeMux()}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	a.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	a.mux.HandleFunc("GET /{$}", a.dashboard)
	a.mux.HandleFunc("GET /round", a.round)
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
			a.fail(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})
	a.mux.HandleFunc("GET /license", func(w http.ResponseWriter, r *http.Request) {
		data, err := assets.ReadFile("LICENSE")
		if err != nil {
			a.fail(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})
	return a, nil
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		http.Error(w, "Host não permitido", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || u.Scheme != "http" {
				http.Error(w, "Origem não permitida", 403)
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Origem não permitida", 403)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Formulário inválido ou muito grande", 400)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(a.token)) != 1 {
			http.Error(w, "Formulário expirado. Recarregue a página.", 403)
			return
		}
	}
	a.mux.ServeHTTP(w, r)
}
func (a *App) render(w http.ResponseWriter, p Page) {
	p.Token = a.token
	var b bytes.Buffer
	if err := a.templates.ExecuteTemplate(&b, "layout.html", p); err != nil {
		a.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b.Bytes())
}
func (a *App) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, errRoundChanged) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Registro não encontrado", 404)
		return
	}
	http.Error(w, "Não foi possível concluir a operação. Tente novamente.", 500)
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
	q := r.URL.Query().Get("q")
	if len(q) > 200 {
		http.Error(w, "Busca muito longa", 400)
		return
	}
	list, err := a.store.stations(r.Context(), q)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, Page{Title: "Estações", View: "stations", Stations: list, Query: q})
}
func (a *App) stationForm(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "Cadastrar estação", View: "form", Action: "/station", Back: "/stations"}
	if r.FormValue("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p.Station, err = a.store.station(r.Context(), id)
		if err != nil {
			a.fail(w, err)
			return
		}
		p.Title = "Editar estação"
	}
	a.render(w, p)
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
			http.Error(w, err.Error(), 400)
			return
		}
		existing, err := a.store.station(r.Context(), id)
		if err != nil {
			a.fail(w, err)
			return
		}
		st.ID = id
		st.Callsign = existing.Callsign
	}
	id, err := a.store.saveStation(r.Context(), st)
	if err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/station?id="+strconv.FormatInt(id, 10))
}
func (a *App) invalidForm(w http.ResponseWriter, r *http.Request, st Station, validationErr error) {
	p := Page{Title: "Revise os dados", View: "form", Action: r.URL.Path, Station: st, Mode: r.PostForm.Get("mode"), Notes: r.PostForm.Get("notes"), Message: validationErr.Error(), Back: "/", IsContact: r.URL.Path == "/contact"}
	if r.PostForm.Get("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			http.Error(w, err.Error(), 400)
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
			http.Error(w, err.Error(), 400)
			return
		}
		p.Round, err = a.store.round(r.Context(), id)
		if err != nil {
			a.fail(w, err)
			return
		}
		if p.Round.EndedAt.Valid {
			a.fail(w, errRoundChanged)
			return
		}
		p.Back = "/round"
		p.RoundID = id
		if p.ParticipantID == 0 {
			p.Participants, err = a.store.participantsInRound(r.Context(), id)
			if err != nil {
				a.fail(w, err)
				return
			}
			p.Title = "Modo QSO"
			p.View = "round"
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	a.render(w, p)
}

func (a *App) history(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > 200 {
		http.Error(w, "Busca muito longa", 400)
		return
	}
	list, err := a.store.contacts(r.Context(), q)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, Page{Title: "Caderno de contatos", View: "history", Contacts: list, Query: q})
}
func (a *App) contactForm(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "Registrar QSO", View: "form", IsContact: true, Mode: "FM", Action: "/contact", Back: "/history"}
	if r.FormValue("id") != "" {
		id, err := identifier(r, "id")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p.Contact, err = a.store.contact(r.Context(), id)
		if err != nil {
			a.fail(w, err)
			return
		}
		p.Station = p.Contact.Station
		p.Mode = p.Contact.Mode
		p.Notes = p.Contact.Notes
		p.Title = "Detalhes e edição do QSO"
	}
	a.render(w, p)
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
			http.Error(w, err.Error(), 400)
			return
		}
		old, err := a.store.contact(r.Context(), id)
		if err != nil {
			a.fail(w, err)
			return
		}
		st.Callsign = old.Callsign
		old.Station = st
		old.Mode = mode
		if err = a.store.updateContact(r.Context(), old); err != nil {
			a.fail(w, err)
			return
		}
		redirect(w, r, "/contact?id="+strconv.FormatInt(id, 10))
		return
	}
	qid, err := a.store.createContact(r.Context(), st, mode, st.Notes)
	if err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/contact?id="+strconv.FormatInt(qid, 10))
}
func (a *App) saveParticipant(w http.ResponseWriter, r *http.Request) {
	roundID, err := identifier(r, "round_id")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	st := readStation(r)
	var participantID int64
	if r.PostForm.Get("id") != "" {
		participantID, err = identifier(r, "id")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		err = a.store.db.QueryRowContext(r.Context(), `SELECT s.id,s.callsign FROM participants p JOIN stations s ON s.id=p.station_id JOIN rounds r ON r.id=p.round_id WHERE p.id=? AND p.round_id=? AND p.removed_at IS NULL AND r.ended_at IS NULL`, participantID, roundID).Scan(&st.ID, &st.Callsign)
		if err != nil {
			a.fail(w, err)
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
	if err := a.store.saveParticipant(r.Context(), roundID, participantID, st, mode, st.Notes); err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/round?message=Participante+salvo")
}
func (a *App) removeParticipant(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err = a.store.removeParticipant(r.Context(), id); err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/round?message=Participante+removido+da+lista")
}
func (a *App) recordParticipant(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err = a.store.recordParticipant(r.Context(), id); err != nil {
		a.fail(w, err)
		return
	}
	redirect(w, r, "/round?message=QSO+registrado+no+caderno")
}
func (a *App) lookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("callsign")
	if len(q) > 32 {
		http.Error(w, "Indicativo muito longo", 400)
		return
	}
	st, err := a.store.lookup(r.Context(), q)
	if err != nil {
		a.fail(w, err)
		return
	}
	var count int
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT count(*) FROM qsos WHERE station_id=?", st.ID).Scan(&count); err != nil {
		a.fail(w, err)
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
		http.Error(w, err.Error(), 400)
		return
	}
	list, err := a.store.participants(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	for _, p := range list {
		if p.ID == id {
			st := p.Station
			a.render(w, Page{Title: "Editar participante", View: "form", Action: "/participant", Back: "/round", Station: st, Mode: p.Mode, Notes: p.Notes, ParticipantID: p.ID, RoundID: p.RoundID, Round: Round{ID: p.RoundID}})
			return
		}
	}
	a.fail(w, sql.ErrNoRows)
}
