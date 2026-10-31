package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

type stationLookup struct {
	Station
	Contacts        int     `json:"contacts"`
	RadioIDRevision string  `json:"radioid_revision,omitempty"`
	RadioIDStatus   string  `json:"radioid_status,omitempty"`
	RadioIDs        []int64 `json:"radio_ids,omitempty"`
}

func (a *App) stationLookup(ctx context.Context, callsign string) (stationLookup, error) {
	var result stationLookup
	st, err := a.store.lookup(ctx, callsign)
	if err != nil {
		return result, err
	}
	result.Station = st
	if err = a.store.db.QueryRowContext(ctx, "SELECT count(*) FROM qsos WHERE deleted_at IS NULL AND station_id=?", st.ID).Scan(&result.Contacts); err != nil {
		return result, err
	}
	m, err := a.store.radioIDMetadata(ctx, st.Callsign)
	result.RadioIDRevision, result.RadioIDs = m.Revision, m.IDs
	return result, err
}

func (a *App) radioIDLookup(w http.ResponseWriter, r *http.Request) {
	st := Station{Callsign: r.PostForm.Get("callsign")}
	if err := st.validate(); err != nil {
		a.httpError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	status, err := a.enrichRadioID(r.Context(), st.Callsign)
	if err != nil {
		status = "unavailable"
	}
	result, err := a.stationLookup(r.Context(), st.Callsign)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		status = "unavailable"
	}
	result.Callsign = st.Callsign
	result.RadioIDStatus = status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (a *App) radioIDPreference(w http.ResponseWriter, r *http.Request) {
	enabled := r.PostForm.Get("enabled") == "1"
	// Wait for an existing bounded request before acknowledging opt-out.
	a.radioID.mu.Lock()
	_, err := a.store.db.ExecContext(r.Context(), "UPDATE app_settings SET radioid_enabled=? WHERE id=1", enabled)
	a.radioID.mu.Unlock()
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if enabled {
		select {
		case a.radioID.wake <- struct{}{}:
		default:
		}
	}
	redirect(w, r, "/round")
}
