package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type Round struct {
	ID        int64
	Name      string
	StartedAt time.Time
	EndedAt   sql.NullString
}

var errRoundChanged = errors.New("a rodada mudou; volte ao Modo QSO e recarregue o formulário")

func scanRound(row interface{ Scan(...any) error }) (Round, error) {
	var round Round
	var started string
	err := row.Scan(&round.ID, &round.Name, &started, &round.EndedAt)
	if err == nil {
		round.StartedAt, err = time.Parse(time.RFC3339Nano, started)
	}
	return round, err
}

func (s *Store) currentRound(ctx context.Context) (Round, error) {
	return scanRound(s.db.QueryRowContext(ctx, "SELECT id,name,started_at,ended_at FROM rounds WHERE ended_at IS NULL"))
}

func (s *Store) round(ctx context.Context, id int64) (Round, error) {
	return scanRound(s.db.QueryRowContext(ctx, "SELECT id,name,started_at,ended_at FROM rounds WHERE id=?", id))
}

func (s *Store) rounds(ctx context.Context) ([]Round, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,started_at,ended_at FROM rounds ORDER BY id DESC LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Round{}
	for rows.Next() {
		round, err := scanRound(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, round)
	}
	return list, rows.Err()
}

// A form carries the latest known round ID. Replaying an old start after the
// round has ended must not create a new, unintended round.
func (s *Store) startRound(ctx context.Context, name string, previous int64) (int64, error) {
	name = strings.TrimSpace(name)
	if err := validateRoundName(name); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM rounds WHERE ended_at IS NULL").Scan(&id)
	if err == nil {
		return id, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var latest int64
	if err = tx.QueryRowContext(ctx, "SELECT coalesce(max(id),0) FROM rounds").Scan(&latest); err != nil {
		return 0, err
	}
	if latest != previous {
		return 0, errRoundChanged
	}
	err = tx.QueryRowContext(ctx, "INSERT INTO rounds(name,started_at) VALUES(?,?) RETURNING id", name, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) endRound(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE rounds SET ended_at=coalesce(ended_at,?) WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) setParticipantActive(ctx context.Context, id int64, active bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE participants SET active=? WHERE id=? AND removed_at IS NULL
	 AND round_id IN (SELECT id FROM rounds WHERE ended_at IS NULL)`, active, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) removeParticipant(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE participants SET removed_at=? WHERE id=? AND removed_at IS NULL
	 AND round_id IN (SELECT id FROM rounds WHERE ended_at IS NULL)`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

func validateRoundName(name string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(strings.TrimSpace(name)) > 120 || strings.ContainsRune(name, 0) {
		return errors.New("nome da rodada inválido: limite de 120 caracteres")
	}
	return nil
}
