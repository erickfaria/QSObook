package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var initialSchema string

type Station struct {
	ID       int64  `json:"id"`
	Callsign string `json:"callsign"`
	Name     string `json:"name"`
	City     string `json:"city"`
	State    string `json:"state"`
	Country  string `json:"country"`
	Notes    string `json:"notes"`
}
type Contact struct {
	Station
	ID         int64
	StationID  int64
	Mode       string
	OccurredAt time.Time
}
type Participant struct {
	Station
	ID        int64
	StationID int64
	Mode      string
	Notes     string
	QSOID     sql.NullInt64
}
type Store struct{ db *sql.DB }

func openStore(ctx context.Context, path string) (*Store, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("caminho do banco: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("diretório do banco: %w", err)
	}
	// Refuse symlinks and non-regular files before opening a local database.
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("o banco deve ser um arquivo regular")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("abrir banco: %w", err)
	}
	if err = file.Chmod(0600); err != nil {
		file.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", u.String()+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("preparar banco: %w", err)
	}
	return s, nil
}
func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return errors.New("banco criado por versão mais recente")
	}
	if version == 0 {
		if _, err := tx.ExecContext(ctx, initialSchema); err != nil {
			return err
		}
	}
	return tx.Commit()
}

var callsignPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9/.-]{0,31}$`)

func (s *Station) validate() error {
	s.Callsign = strings.ToUpper(strings.TrimSpace(s.Callsign))
	if !callsignPattern.MatchString(s.Callsign) {
		return errors.New("informe um indicativo de até 32 caracteres: letras, números, /, ponto ou hífen")
	}
	fields := []*string{&s.Name, &s.City, &s.State, &s.Country, &s.Notes}
	for i, p := range fields {
		*p = strings.TrimSpace(*p)
		limit := 120
		if i == 4 {
			limit = 4000
		}
		if !utf8.ValidString(*p) || utf8.RuneCountInString(*p) > limit || strings.ContainsRune(*p, 0) {
			return fmt.Errorf("campo inválido: limite de %d caracteres", limit)
		}
	}
	return nil
}
func validateMode(mode string) error {
	if strings.TrimSpace(mode) == "" || !utf8.ValidString(mode) || utf8.RuneCountInString(mode) > 40 || strings.ContainsAny(mode, "\x00\r\n") {
		return errors.New("informe um modo de até 40 caracteres")
	}
	return nil
}
func validateNotes(notes string) error {
	if !utf8.ValidString(notes) || utf8.RuneCountInString(notes) > 4000 || strings.ContainsRune(notes, 0) {
		return errors.New("observações inválidas: limite de 4000 caracteres")
	}
	return nil
}
func (s *Store) saveStation(ctx context.Context, st Station) (int64, error) {
	if err := st.validate(); err != nil {
		return 0, err
	}
	if st.ID > 0 {
		result, err := s.db.ExecContext(ctx, `UPDATE stations SET name=?,city=?,state=?,country=?,notes=? WHERE id=?`, st.Name, st.City, st.State, st.Country, st.Notes, st.ID)
		if err != nil {
			return 0, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, sql.ErrNoRows
		}
		return st.ID, nil
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO stations(callsign,name,city,state,country,notes) VALUES(?,?,?,?,?,?) ON CONFLICT(callsign) DO UPDATE SET
		name=coalesce(nullif(excluded.name,''),stations.name), city=coalesce(nullif(excluded.city,''),stations.city),
		state=coalesce(nullif(excluded.state,''),stations.state), country=coalesce(nullif(excluded.country,''),stations.country),
		notes=coalesce(nullif(excluded.notes,''),stations.notes) RETURNING id`, st.Callsign, st.Name, st.City, st.State, st.Country, st.Notes).Scan(&id)
	return id, err
}

// Conversation forms may complete the current registration, but never erase
// fields with empty values or write permanent operator notes.
func mergeConversationStation(ctx context.Context, tx *sql.Tx, st Station) (int64, error) {
	var id int64
	if st.ID > 0 {
		err := tx.QueryRowContext(ctx, `UPDATE stations SET
			name=coalesce(nullif(?,''),name), city=coalesce(nullif(?,''),city),
			state=coalesce(nullif(?,''),state), country=coalesce(nullif(?,''),country)
			WHERE id=? RETURNING id`, st.Name, st.City, st.State, st.Country, st.ID).Scan(&id)
		return id, err
	}
	err := tx.QueryRowContext(ctx, `INSERT INTO stations(callsign,name,city,state,country) VALUES(?,?,?,?,?)
		ON CONFLICT(callsign) DO UPDATE SET
		name=coalesce(nullif(excluded.name,''),stations.name), city=coalesce(nullif(excluded.city,''),stations.city),
		state=coalesce(nullif(excluded.state,''),stations.state), country=coalesce(nullif(excluded.country,''),stations.country)
		RETURNING id`, st.Callsign, st.Name, st.City, st.State, st.Country).Scan(&id)
	return id, err
}

// Saving a participant does not confirm a QSO. Only an explicit edit changes
// an existing participation; resubmitting an addition preserves it.
func (s *Store) saveParticipant(ctx context.Context, st Station, mode, notes string) error {
	if err := st.validate(); err != nil {
		return err
	}
	if err := validateMode(mode); err != nil {
		return err
	}
	notes = strings.TrimSpace(notes)
	if err := validateNotes(notes); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id, err := mergeConversationStation(ctx, tx, st)
	if err != nil {
		return fmt.Errorf("salvar operador da rodada: %w", err)
	}
	if st.ID > 0 {
		result, err := tx.ExecContext(ctx, `UPDATE participants SET mode=?,notes=? WHERE station_id=?`, strings.TrimSpace(mode), notes, id)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return sql.ErrNoRows
		}
	} else if _, err := tx.ExecContext(ctx, `INSERT INTO participants(station_id,mode,notes) VALUES(?,?,?) ON CONFLICT(station_id) DO NOTHING`, id, strings.TrimSpace(mode), notes); err != nil {
		return fmt.Errorf("adicionar participante: %w", err)
	}
	return tx.Commit()
}

// A manual contact also keeps conversation notes away from the registration.
func (s *Store) createContact(ctx context.Context, st Station, mode, notes string) (int64, error) {
	if err := st.validate(); err != nil {
		return 0, err
	}
	if err := validateMode(mode); err != nil {
		return 0, err
	}
	notes = strings.TrimSpace(notes)
	if err := validateNotes(notes); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	id, err := mergeConversationStation(ctx, tx, st)
	if err != nil {
		return 0, err
	}
	var qid int64
	if err := tx.QueryRowContext(ctx, insertContactSQL, strings.TrimSpace(mode), notes, time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), id).Scan(&qid); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return qid, nil
}

const stationColumns = `id,callsign,name,city,state,country,notes`

func scanStation(row interface{ Scan(...any) error }) (Station, error) {
	var st Station
	err := row.Scan(&st.ID, &st.Callsign, &st.Name, &st.City, &st.State, &st.Country, &st.Notes)
	return st, err
}
func (s *Store) station(ctx context.Context, id int64) (Station, error) {
	return scanStation(s.db.QueryRowContext(ctx, "SELECT "+stationColumns+" FROM stations WHERE id=?", id))
}
func (s *Store) lookup(ctx context.Context, callsign string) (Station, error) {
	return scanStation(s.db.QueryRowContext(ctx, "SELECT "+stationColumns+" FROM stations WHERE callsign=?", strings.ToUpper(strings.TrimSpace(callsign))))
}
func (s *Store) stations(ctx context.Context, query string) ([]Station, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+stationColumns+" FROM stations WHERE instr(callsign,upper(?))>0 OR instr(lower(name),lower(?))>0 ORDER BY callsign LIMIT 200", query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Station{}
	for rows.Next() {
		st, err := scanStation(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, st)
	}
	return list, rows.Err()
}

const insertContactSQL = `INSERT INTO qsos(station_id,callsign,name,city,state,country,mode,notes,occurred_at) SELECT id,callsign,name,city,state,country,?,?,? FROM stations WHERE id=? RETURNING id`

func (s *Store) record(ctx context.Context, stationID int64, mode, notes string) (int64, error) {
	if err := validateMode(mode); err != nil {
		return 0, err
	}
	if err := validateNotes(notes); err != nil {
		return 0, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, insertContactSQL, strings.TrimSpace(mode), notes, time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), stationID).Scan(&id)
	return id, err
}
func scanContact(row interface{ Scan(...any) error }) (Contact, error) {
	var c Contact
	var stamp string
	err := row.Scan(&c.ID, &c.StationID, &c.Callsign, &c.Name, &c.City, &c.State, &c.Country, &c.Mode, &c.Notes, &stamp)
	if err != nil {
		return c, err
	}
	c.OccurredAt, err = time.Parse(time.RFC3339Nano, stamp)
	return c, err
}

const contactColumns = `id,station_id,callsign,name,city,state,country,mode,notes,occurred_at`

func (s *Store) contacts(ctx context.Context, q string) ([]Contact, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+contactColumns+" FROM qsos WHERE instr(callsign,upper(?))>0 OR instr(lower(name),lower(?))>0 ORDER BY occurred_at DESC,id DESC LIMIT 200", q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Contact{}
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
func (s *Store) contact(ctx context.Context, id int64) (Contact, error) {
	return scanContact(s.db.QueryRowContext(ctx, "SELECT "+contactColumns+" FROM qsos WHERE id=?", id))
}
func (s *Store) updateContact(ctx context.Context, c Contact) error {
	if err := c.Station.validate(); err != nil {
		return err
	}
	if err := validateMode(c.Mode); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE qsos SET name=?,city=?,state=?,country=?,mode=?,notes=? WHERE id=?`, c.Name, c.City, c.State, c.Country, c.Mode, c.Notes, c.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) addParticipant(ctx context.Context, id int64, mode, notes string) error {
	if err := validateMode(mode); err != nil {
		return err
	}
	if err := validateNotes(notes); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO participants(station_id,mode,notes) VALUES(?,?,?) ON CONFLICT(station_id) DO UPDATE SET mode=excluded.mode,notes=excluded.notes`, id, strings.TrimSpace(mode), notes)
	return err
}
func (s *Store) participants(ctx context.Context) ([]Participant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,s.id,s.callsign,s.name,s.city,s.state,s.country,s.notes,p.mode,p.notes,p.qso_id FROM participants p JOIN stations s ON s.id=p.station_id ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Participant{}
	for rows.Next() {
		var p Participant
		err := rows.Scan(&p.ID, &p.StationID, &p.Callsign, &p.Name, &p.City, &p.State, &p.Country, &p.Station.Notes, &p.Mode, &p.Notes, &p.QSOID)
		if err != nil {
			return nil, err
		}
		p.Station.ID = p.StationID
		list = append(list, p)
	}
	return list, rows.Err()
}

// A transaction makes repeated clicks on a participant record at most one QSO.
func (s *Store) recordParticipant(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT qso_id FROM participants WHERE id=?`, id).Scan(&existing); err != nil {
		return err
	}
	if existing.Valid {
		return tx.Commit()
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO qsos(station_id,callsign,name,city,state,country,mode,notes,occurred_at) SELECT s.id,s.callsign,s.name,s.city,s.state,s.country,p.mode,p.notes,? FROM participants p JOIN stations s ON s.id=p.station_id WHERE p.id=?`, time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), id)
	if err != nil {
		return err
	}
	qid, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE participants SET qso_id=? WHERE id=?`, qid, id); err != nil {
		return err
	}
	return tx.Commit()
}
