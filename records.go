package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var errSubmissionConflict = errors.New("Este formulário já foi utilizado. Abra um novo registro.")

func newSubmissionKey() (string, error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
func validateRecordDetails(band, round string) error {
	if !utf8.ValidString(band) || utf8.RuneCountInString(band) > 40 || strings.ContainsAny(band, "\x00\r\n") {
		return errors.New("Faixa inválida: até 40 caracteres")
	}
	return validateRoundName(round)
}

// Save registration and optional confirmation together. Replays are checked
// before any registration write, so they cannot overwrite a newer operator.
func (s *Store) saveRecord(ctx context.Context, st Station, mode, band, round, notes, key string, confirmed bool) (int64, error) {
	if err := st.validate(); err != nil {
		return 0, err
	}
	if err := validateNotes(notes); err != nil {
		return 0, err
	}
	if err := validateRecordDetails(band, round); err != nil {
		return 0, err
	}
	if confirmed {
		if err := validateMode(mode); err != nil {
			return 0, err
		}
	}
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) != 16 {
		return 0, errors.New("Formulário inválido. Recarregue a página.")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var previous int64
	var callsign string
	err = tx.QueryRowContext(ctx, "SELECT id,callsign FROM qsos WHERE submission_key=?", key).Scan(&previous, &callsign)
	if err == nil {
		if callsign != st.Callsign {
			return 0, errSubmissionConflict
		}
		return previous, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	id, err := mergeConversationStation(ctx, tx, st)
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE stations SET notes=coalesce(nullif(notes,''),?) WHERE id=?", st.Notes, id); err != nil {
		return 0, err
	}
	var qid int64
	if confirmed {
		err = tx.QueryRowContext(ctx, `INSERT INTO qsos(station_id,callsign,name,city,state,country,mode,notes,occurred_at,band,round_name,submission_key)
   SELECT id,callsign,name,city,state,country,?,?,?,?,?,? FROM stations WHERE id=? RETURNING id`, strings.TrimSpace(mode), strings.TrimSpace(notes), time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), strings.TrimSpace(band), strings.TrimSpace(round), key, id).Scan(&qid)
		if err != nil {
			return 0, err
		}
	}
	return qid, tx.Commit()
}
