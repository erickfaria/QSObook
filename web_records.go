package main

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// Compatibility URL: render the same workspace, not a second entry panel.
func (a *App) records(w http.ResponseWriter, r *http.Request) { a.round(w, r) }
func recordChoice(r *http.Request, name string) string {
	value := strings.TrimSpace(r.PostForm.Get(name))
	if value == "__other" {
		return strings.TrimSpace(r.PostForm.Get(name + "_other"))
	}
	return value
}
func (a *App) saveRecord(w http.ResponseWriter, r *http.Request) {
	st := readStation(r)
	notes := st.Notes
	st.Notes = r.PostForm.Get("operator_notes")
	mode, band := recordChoice(r, "mode"), recordChoice(r, "band")
	round := ""
	if r.PostForm.Get("in_round") == "1" {
		round = strings.TrimSpace(r.PostForm.Get("round_name"))
	}
	confirmed := r.PostForm.Get("confirmed") == "1"
	key := r.PostForm.Get("submission_key")
	validationErr := validateRecordInput(&st, mode, band, round, notes, key, confirmed)
	var qid int64
	var err error
	if validationErr == nil {
		qid, err = a.store.saveRecord(r.Context(), st, mode, band, round, notes, key, confirmed)
	} else {
		err = validationErr
	}
	if err != nil {
		if validationErr == nil && !errors.Is(err, errSubmissionConflict) && !errors.Is(err, errStationInTrash) {
			a.fail(w, r, err)
			return
		}
		status := http.StatusBadRequest
		if errors.Is(err, errSubmissionConflict) || errors.Is(err, errStationInTrash) {
			status = http.StatusConflict
		}
		// Preserve drafts on validation errors, including the original replay key.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		a.render(w, r, Page{Title: "Registros", View: "records", Action: "/records", SubmissionKey: key, Station: st, Mode: mode, Band: band, RoundName: round, Notes: notes, Confirmed: confirmed, Message: err.Error()})
		return
	}
	message := "Cadastro salvo"
	if qid > 0 {
		message = "QSO registrado no caderno"
	}
	if confirmed {
		c, e := a.store.contact(r.Context(), qid)
		if e != nil || a.autoLocate(r.Context(), ContactCity{City: c.City, State: c.State, Country: c.Country}) != nil {
			message = "Registro salvo. Cidade pendente de localização no mapa."
		}
	}
	redirect(w, r, "/records?message="+url.QueryEscape(message))
}
