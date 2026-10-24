package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// A session is simply a live listening list. A named roundtable is optional.
// Registration, participation and optional confirmation are committed together.
func (s *Store) saveWorkspaceEntry(ctx context.Context, roundID int64, st Station, mode, band, notes, key string, confirmed bool) (int64, error) {
	if err := validateRecordInput(&st, mode, band, "", notes, key, true); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var qid int64
	var callsign string
	err = tx.QueryRowContext(ctx, "SELECT id,callsign FROM qsos WHERE submission_key=?", key).Scan(&qid, &callsign)
	if err == nil {
		if callsign != st.Callsign {
			return 0, errSubmissionConflict
		}
		return qid, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	round, err := scanRound(tx.QueryRowContext(ctx, "SELECT id,name,started_at,ended_at,edition FROM rounds WHERE id=? AND ended_at IS NULL", roundID))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errRoundChanged
	}
	if err != nil {
		return 0, err
	}
	st.ID = 0
	stationID, err := mergeConversationStation(ctx, tx, st)
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE stations SET notes=coalesce(nullif(notes,''),?) WHERE id=?", st.Notes, stationID); err != nil {
		return 0, err
	}
	var participantID int64
	var existing sql.NullInt64
	err = tx.QueryRowContext(ctx, "SELECT id,qso_id FROM participants WHERE round_id=? AND station_id=? AND removed_at IS NULL", roundID, stationID).Scan(&participantID, &existing)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, "INSERT INTO participants(round_id,station_id,mode,band,notes) VALUES(?,?,?,?,?) RETURNING id", roundID, stationID, strings.TrimSpace(mode), strings.TrimSpace(band), strings.TrimSpace(notes)).Scan(&participantID)
	}
	if err != nil {
		return 0, err
	}
	if confirmed {
		if existing.Valid {
			return existing.Int64, tx.Commit()
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO qsos(station_id,callsign,name,city,state,country,mode,band,notes,occurred_at,round_id,round_name,submission_key)
   SELECT s.id,s.callsign,s.name,s.city,s.state,s.country,p.mode,p.band,p.notes,?,?,?,? FROM stations s JOIN participants p ON p.station_id=s.id WHERE p.id=? RETURNING id`, time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), roundID, round.Label(), key, participantID).Scan(&qid)
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE participants SET qso_id=? WHERE id=?", qid, participantID); err != nil {
			return 0, err
		}
	}
	return qid, tx.Commit()
}
func (a *App) saveWorkspaceEntry(w http.ResponseWriter, r *http.Request) {
	roundID, err := identifier(r, "round_id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	st := readStation(r)
	notes := st.Notes
	st.Notes = r.PostForm.Get("operator_notes")
	mode, band := recordChoice(r, "mode"), recordChoice(r, "band")
	key := r.PostForm.Get("submission_key")
	confirmed := r.PostForm.Get("confirmed") == "1"
	validationErr := validateRecordInput(&st, mode, band, "", notes, key, true)
	if validationErr != nil {
		a.workspaceDraft(w, r, roundID, st, mode, band, notes, key, confirmed, validationErr, 400)
		return
	}
	qid, err := a.store.saveWorkspaceEntry(r.Context(), roundID, st, mode, band, notes, key, confirmed)
	if errors.Is(err, errSubmissionConflict) || errors.Is(err, errStationInTrash) {
		a.workspaceDraft(w, r, roundID, st, mode, band, notes, key, confirmed, err, 409)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	message := "Estação salva na lista. Nenhum QSO foi criado."
	if qid > 0 {
		message = "QSO registrado no caderno"
		c, e := a.store.contact(r.Context(), qid)
		if e != nil || a.autoLocate(r.Context(), ContactCity{City: c.City, State: c.State, Country: c.Country}) != nil {
			message = "Registro salvo. Cidade pendente de localização no mapa."
		}
	}
	redirect(w, r, "/round?message="+url.QueryEscape(message))
}
func (a *App) workspaceDraft(w http.ResponseWriter, r *http.Request, id int64, st Station, mode, band, notes, key string, confirmed bool, validationErr error, status int) {
	round, err := a.store.round(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if round.EndedAt.Valid {
		a.fail(w, r, errRoundChanged)
		return
	}
	list, err := a.store.participantsInRound(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	a.render(w, r, Page{Title: "Modo QSO", View: "round", Action: "/qso/entry", Round: round, RoundID: id, Participants: list, Station: st, Mode: mode, Band: band, Notes: notes, SubmissionKey: key, Confirmed: confirmed, Message: validationErr.Error()})
}
