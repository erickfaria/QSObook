package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
)

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "Seu caderno", View: "dashboard", Message: r.URL.Query().Get("message")}
	var err error
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT count(*) FROM qsos").Scan(&p.ContactCount); err != nil {
		a.fail(w, r, err)
		return
	}
	p.Round, err = a.store.currentRound(r.Context())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.fail(w, r, err)
		return
	}
	p.Rounds, err = a.store.rounds(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if len(p.Rounds) > 0 {
		p.PreviousRound = p.Rounds[0].ID
	}
	p.MapEnabled, err = a.store.mapEnabled(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p.MapCities, err = a.store.contactCities(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, p)
}

func (a *App) round(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "Modo QSO", View: "round", Mode: "FM", Action: "/participant", Message: r.URL.Query().Get("message")}
	var err error
	if r.URL.Query().Get("id") != "" {
		id, e := identifier(r, "id")
		if e != nil {
			a.httpError(w, r, e.Error(), 400)
			return
		}
		p.Round, err = a.store.round(r.Context(), id)
	} else {
		p.Round, err = a.store.currentRound(r.Context())
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p.RoundID = p.Round.ID
	if p.Round.ID > 0 {
		p.Participants, err = a.store.participantsInRound(r.Context(), p.Round.ID)
		if err != nil {
			a.fail(w, r, err)
			return
		}
	} else {
		list, e := a.store.rounds(r.Context())
		if e != nil {
			a.fail(w, r, e)
			return
		}
		if len(list) > 0 {
			p.PreviousRound = list[0].ID
		}
	}
	a.render(w, r, p)
}

func (a *App) startRound(w http.ResponseWriter, r *http.Request) {
	previous, err := strconv.ParseInt(r.PostForm.Get("previous_round"), 10, 64)
	if err != nil || previous < 0 {
		a.httpError(w, r, "Rodada anterior inválida", 400)
		return
	}
	name := r.PostForm.Get("name")
	if err := validateRoundName(name); err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	id, err := a.store.startRound(r.Context(), name, previous)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/round?id="+strconv.FormatInt(id, 10))
}

func (a *App) endRound(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	if err = a.store.endRound(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/?message=Rodada+encerrada.+Seus+registros+foram+preservados.")
}

func (a *App) participantStatus(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	value := r.PostForm.Get("active")
	if value != "0" && value != "1" {
		a.httpError(w, r, "Estado inválido", 400)
		return
	}
	if err = a.store.setParticipantActive(r.Context(), id, value == "1"); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/round")
}
