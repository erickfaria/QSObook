package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type radioIDMetadata struct {
	Status   string
	Next     int64
	Fields   int
	Revision string
	IDs      []int64
}

func (s *Store) radioIDEnabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx, "SELECT radioid_enabled FROM app_settings WHERE id=1").Scan(&enabled)
	return enabled, err
}

func (s *Store) radioIDMetadata(ctx context.Context, callsign string) (radioIDMetadata, error) {
	var m radioIDMetadata
	var ids string
	err := s.db.QueryRowContext(ctx, "SELECT status,next_attempt,auto_fields,revision,radio_ids FROM radioid_lookups WHERE callsign=?", callsign).Scan(&m.Status, &m.Next, &m.Fields, &m.Revision, &ids)
	if errors.Is(err, sql.ErrNoRows) {
		return m, nil
	}
	if err == nil {
		err = json.Unmarshal([]byte(ids), &m.IDs)
	}
	return m, err
}

func incompleteStation(st Station) bool {
	return st.Name == "" || st.City == "" || st.State == "" || st.Country == ""
}

func clearRadioIDOrigin(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := tx.ExecContext(ctx, "UPDATE radioid_lookups SET auto_fields=0,revision='' WHERE callsign=(SELECT callsign FROM stations WHERE id=?)", id)
	return err
}

// Only a fresh form may replace fields that RadioID supplied. Consuming the
// revision prevents stale forms from undoing a subsequent manual correction.
func correctRadioIDFields(ctx context.Context, tx *sql.Tx, st Station) error {
	if len(st.radioIDRevision) != 32 {
		return nil
	}
	var fields int
	err := tx.QueryRowContext(ctx, "SELECT auto_fields FROM radioid_lookups WHERE callsign=? AND revision=?", st.Callsign, st.radioIDRevision).Scan(&fields)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var sets []string
	var args []any
	for i, field := range []struct{ column, value string }{{"name", st.Name}, {"city", st.City}, {"state", st.State}, {"country", st.Country}} {
		if fields&(1<<i) != 0 && field.value != "" {
			sets, args = append(sets, field.column+"=?"), append(args, field.value)
		}
	}
	if len(sets) > 0 {
		args = append(args, st.Callsign)
		if _, err = tx.ExecContext(ctx, "UPDATE stations SET "+strings.Join(sets, ",")+" WHERE callsign=? AND deleted_at IS NULL", args...); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE radioid_lookups SET auto_fields=0,revision='' WHERE callsign=?", st.Callsign)
	return err
}

func (s *Store) applyRadioID(ctx context.Context, result radioIDResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var enabled bool
	if err = tx.QueryRowContext(ctx, "SELECT radioid_enabled FROM app_settings WHERE id=1").Scan(&enabled); err != nil || !enabled {
		return err
	}
	if err = stationAvailable(ctx, tx, result.Station.Callsign); err != nil {
		return err
	}
	old, err := scanStation(tx.QueryRowContext(ctx, "SELECT "+stationColumns+" FROM stations WHERE callsign=?", result.Station.Callsign))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var fields int
	var revision string
	if err = tx.QueryRowContext(ctx, "SELECT auto_fields,revision FROM radioid_lookups WHERE callsign=?", result.Station.Callsign).Scan(&fields, &revision); err != nil {
		return err
	}
	added := 0
	for i, values := range [][2]string{{old.Name, result.Station.Name}, {old.City, result.Station.City}, {old.State, result.Station.State}, {old.Country, result.Station.Country}} {
		if values[0] == "" && values[1] != "" {
			added |= 1 << i
		}
	}
	if added != 0 {
		if old.ID > 0 {
			snapshot, e := json.Marshal(old)
			if e != nil {
				return e
			}
			if _, err = tx.ExecContext(ctx, `UPDATE participants SET radioid_snapshot=? WHERE station_id=? AND radioid_snapshot IS NULL
 AND (removed_at IS NOT NULL OR round_id IN (SELECT id FROM rounds WHERE ended_at IS NOT NULL))`, string(snapshot), old.ID); err != nil {
				return err
			}
		}
		if revision, err = newSubmissionKey(); err != nil {
			return err
		}
	}
	if _, err = mergeConversationStation(ctx, tx, result.Station); err != nil {
		return err
	}
	ids, err := json.Marshal(result.IDs)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE radioid_lookups SET status='found',next_attempt=?,radio_ids=?,auto_fields=?,revision=? WHERE callsign=?`, time.Now().Add(24*time.Hour).Unix(), string(ids), fields|added, revision, result.Station.Callsign)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RadioID metadata travels with consistent notebook backups. Validate it before
// trusting any uploaded retry schedule, provenance token or historical snapshot.
func validateRadioIDBackup(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "SELECT callsign,radio_ids,auto_fields,revision FROM radioid_lookups")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var callsign, data, revision string
		var fields int
		if rows.Scan(&callsign, &data, &fields, &revision) != nil {
			return errBackupInvalid
		}
		st := Station{Callsign: callsign}
		var ids []int64
		if st.validate() != nil || st.Callsign != callsign || len(data) > 4096 || json.Unmarshal([]byte(data), &ids) != nil || len(ids) > 200 {
			return errBackupInvalid
		}
		for i, id := range ids {
			if id <= 0 || id > 9999999999 || (i > 0 && id <= ids[i-1]) {
				return errBackupInvalid
			}
		}
		if decoded, e := hex.DecodeString(revision); (revision != "" && (e != nil || len(decoded) != 16)) || (fields > 0 && revision == "") {
			return errBackupInvalid
		}
	}
	return rows.Err()
}

func readRadioIDSnapshot(data string, id int64, callsign string) (Station, error) {
	var st Station
	if len(data) > 32<<10 || json.Unmarshal([]byte(data), &st) != nil || st.ID != id || st.Callsign != callsign {
		return Station{}, errBackupInvalid
	}
	validated := st
	if validated.validate() != nil {
		return Station{}, errBackupInvalid
	}
	return st, nil
}
