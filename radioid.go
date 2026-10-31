package main

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

type radioIDService struct {
	mu     sync.Mutex
	client *radioIDClient
	wake   chan struct{}
}

func (s *Store) queueRadioID(ctx context.Context, callsign string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO radioid_lookups(callsign)
 SELECT ? WHERE (SELECT radioid_enabled FROM app_settings WHERE id=1)=1 ON CONFLICT DO NOTHING`, callsign)
	return err
}

func (a *App) enrichRadioID(ctx context.Context, callsign string) (string, error) {
	enabled, err := a.store.radioIDEnabled(ctx)
	if err != nil || !enabled {
		return "disabled", err
	}
	if err = stationAvailable(ctx, a.store.db, callsign); err != nil {
		if errors.Is(err, errStationInTrash) {
			return "unavailable", nil
		}
		return "unavailable", err
	}
	local, err := a.store.lookup(ctx, callsign)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "unavailable", err
	}
	if err == nil && !incompleteStation(local) {
		return "local", nil
	}
	if err := a.store.queueRadioID(ctx, callsign); err != nil {
		return "unavailable", err
	}
	if !a.radioID.mu.TryLock() {
		return "pending", nil
	}
	defer a.radioID.mu.Unlock()
	m, err := a.store.radioIDMetadata(ctx, callsign)
	if err != nil {
		return "unavailable", err
	}
	if m.Next > time.Now().Unix() {
		return m.Status, nil
	}
	var after int64
	if err = a.store.db.QueryRowContext(ctx, "SELECT radioid_after FROM app_settings WHERE id=1").Scan(&after); err != nil {
		return "unavailable", err
	}
	if delay := time.Until(time.Unix(after, 0)); delay > 0 {
		if delay > 3*time.Second {
			return "pending", nil
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return "unavailable", ctx.Err()
		case <-timer.C:
		}
	}
	// A persisted lease also limits requests across restarts/processes. No SQL
	// connection/transaction is held while contacting the external provider.
	lease, err := a.store.db.ExecContext(ctx, `UPDATE app_settings SET radioid_after=?
 WHERE id=1 AND radioid_enabled=1 AND radioid_after<=?`, time.Now().Unix()+10, time.Now().Unix())
	if err != nil {
		return "unavailable", err
	}
	claimed, err := lease.RowsAffected()
	if err != nil || claimed == 0 {
		return "pending", err
	}
	if _, err = a.store.db.ExecContext(ctx, "UPDATE radioid_lookups SET status='pending',next_attempt=? WHERE callsign=?", time.Now().Add(24*time.Hour).Unix(), callsign); err != nil {
		return "unavailable", err
	}
	result, lookupErr := a.radioID.client.lookup(ctx, callsign)
	cooldown := time.Now().Add(3 * time.Second)
	status, next := "not_found", time.Now().Add(7*24*time.Hour)
	if lookupErr != nil {
		status, next = "unavailable", time.Now().Add(24*time.Hour)
		cooldown = time.Now().Add(15 * time.Minute)
		if result.RetryAt.After(cooldown) {
			cooldown = result.RetryAt
		}
	}
	if _, err = a.store.db.ExecContext(ctx, "UPDATE app_settings SET radioid_after=? WHERE id=1", cooldown.Unix()); err != nil {
		return "unavailable", err
	}
	if lookupErr == nil && result.Found {
		if err = a.store.applyRadioID(ctx, result); err != nil {
			return "unavailable", err
		}
		return "found", nil
	}
	_, err = a.store.db.ExecContext(ctx, "UPDATE radioid_lookups SET status=?,next_attempt=? WHERE callsign=?", status, next.Unix(), callsign)
	return status, err
}

func (s *Store) nextRadioID(ctx context.Context) (string, error) {
	var callsign string
	err := s.db.QueryRowContext(ctx, `SELECT s.callsign FROM stations s LEFT JOIN radioid_lookups l ON l.callsign=s.callsign
 WHERE s.deleted_at IS NULL AND (s.name='' OR s.city='' OR s.state='' OR s.country='')
 AND coalesce(l.next_attempt,0)<=? AND (SELECT radioid_enabled FROM app_settings WHERE id=1)=1
 ORDER BY coalesce(l.next_attempt,0),s.id LIMIT 1`, time.Now().Unix()).Scan(&callsign)
	return callsign, err
}

func (a *App) syncRadioID(ctx context.Context) {
	a.requests.RLock()
	_, err := a.store.nextRadioID(ctx)
	var claimed int64
	if err == nil {
		var result sql.Result
		result, err = a.store.db.ExecContext(ctx, `UPDATE app_settings SET radioid_sync_after=?
 WHERE id=1 AND radioid_enabled=1 AND radioid_sync_after<=?`, time.Now().Add(24*time.Hour).Unix(), time.Now().Unix())
		if err == nil {
			claimed, err = result.RowsAffected()
		}
	}
	a.requests.RUnlock()
	if err != nil || claimed == 0 {
		return
	}
	// A small daily batch, never a directory download. Each request rechecks
	// consent and current rows, so restoration cannot receive an old response.
	for i := 0; i < 10; i++ {
		a.requests.RLock()
		callsign, err := a.store.nextRadioID(ctx)
		status := ""
		if err == nil {
			status, err = a.enrichRadioID(ctx, callsign)
		}
		a.requests.RUnlock()
		if err != nil || status == "unavailable" || status == "pending" || status == "disabled" {
			return
		}
		timer := time.NewTimer(3 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (a *App) startRadioID(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		// This minute timer only checks local eligibility; the persisted daily
		// schedule and per-callsign cache govern actual external requests.
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-a.radioID.wake:
			}
			a.syncRadioID(ctx)
		}
	}()
	return func() { cancel(); <-done }
}
