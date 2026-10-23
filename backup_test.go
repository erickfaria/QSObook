package main

import (
	"bytes"
	"context"
	"database/sql"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func upload(a *App, path string, data []byte, token string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	m.WriteField("csrf", token)
	part, _ := m.CreateFormFile("file", "notebook.db")
	part.Write(data)
	m.Close()
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080"+path, &body)
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestLiveBackupAndAtomicRestore(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if _, err := s.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	qid, err := s.saveRecord(ctx, Station{Callsign: "ZZ0BACKUP", Name: "José", City: "São Paulo", Notes: "Permanent"}, "DMR", "70 cm", "Example round", "Conversation", strings.Repeat("a", 32), true)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.contact(ctx, qid)
	if err = s.saveParticipant(ctx, 1, 0, Station{Callsign: "ZZ0BACKUP"}, "FM", ""); err == nil {
		t.Fatal("expected missing round")
	}
	s.startRound(ctx, "Example", 0)
	if err = s.saveParticipant(ctx, 1, 0, Station{Callsign: "ZZ0BACKUP"}, "FM", ""); err != nil {
		t.Fatal(err)
	}
	s.db.Exec("UPDATE participants SET qso_id=?", qid)
	s.db.Exec("UPDATE app_settings SET map_enabled=1,home_city='Example';INSERT INTO city_coordinates VALUES('Example','','',1,2,'2026-10-23T00:00:00Z')")
	s.moveTrash(ctx, "contact", qid, false)
	path := filepath.Join(t.TempDir(), "backup.db")
	if err = s.backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err = validateBackup(ctx, path); err != nil {
		t.Fatal("valid backup rejected", err)
	}
	s.moveTrash(ctx, "contact", qid, true)
	s.saveStation(ctx, Station{Callsign: "ZZ0NEW"})
	if err = s.restore(ctx, path); err != nil {
		t.Fatal("restore failed", err)
	}
	if _, err = s.lookup(ctx, "ZZ0NEW"); err != sql.ErrNoRows {
		t.Fatal("replacement failed", err)
	}
	if _, err = s.contact(ctx, qid); err != sql.ErrNoRows {
		t.Fatal("trash not restored", err)
	}
	s.moveTrash(ctx, "contact", qid, true)
	after, _ := s.contact(ctx, qid)
	if after != c {
		t.Fatal("historical data changed", after, c)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM participants WHERE qso_id=?", qid).Scan(&count)
	if count != 1 {
		t.Fatal("links lost")
	}
	s.db.QueryRow("SELECT count(*) FROM city_coordinates").Scan(&count)
	if count != 1 {
		t.Fatal("cache lost")
	}
	s.db.QueryRow("SELECT count(*) FROM sqlite_sequence WHERE name='participants'").Scan(&count)
	if count != 1 {
		t.Fatal("sequence duplicated", count)
	}
	// A cancelled restore does not erase the current notebook.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = s.restore(cancelled, path); err == nil {
		t.Fatal("cancelled restore succeeded")
	}
	if _, err = s.contact(ctx, qid); err != nil {
		t.Fatal("cancelled restore changed data", err)
	}
}
func TestRestoreRejectsForeignSchemaAndInvalidRows(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	s.saveStation(ctx, Station{Callsign: "ZZ0KEEP"})
	for _, mutation := range []string{"CREATE TRIGGER malicious AFTER INSERT ON stations BEGIN DELETE FROM qsos;END", "CREATE VIEW malicious AS SELECT * FROM stations", "CREATE VIEW sqliteXmalicious AS SELECT * FROM stations", "INSERT INTO rounds(name,started_at) VALUES('Example','invalid')", "PRAGMA user_version=999", "UPDATE qsos SET occurred_at='invalid'", "UPDATE qsos SET id=-1", "INSERT INTO participants(round_id,station_id,mode,notes) VALUES(999,999,'FM','')"} {
		t.Run(mutation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.db")
			if err := s.backup(ctx, path); err != nil {
				t.Fatal(err)
			}
			db, _ := sql.Open("sqlite", path)
			db.Exec("INSERT INTO qsos(station_id,callsign,name,city,state,country,mode,notes,occurred_at) VALUES(1,'ZZ0KEEP','','','','','FM','','2026-10-23T00:00:00Z')")
			if _, err := db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			db.Close()
			if err := s.restore(ctx, path); err == nil {
				t.Fatal("unsafe upload accepted")
			}
			if _, err := s.lookup(ctx, "ZZ0KEEP"); err != nil {
				t.Fatal("current data changed")
			}
		})
	}
}
func TestRestorePreviewGuardRecoveryAndOldForms(t *testing.T) {
	a := testApp(t)
	defer a.closeTransfers()
	ctx := context.Background()
	a.store.saveStation(ctx, Station{Callsign: "ZZ0OLD"})
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := a.store.backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if w := upload(a, "/data/restore", data, ""); w.Code != 403 {
		t.Fatal("upload bypasses CSRF", w.Code)
	}
	if w := upload(a, "/data/restore", []byte("not sqlite"), a.token); w.Code != 400 {
		t.Fatal("bad upload accepted", w.Code, w.Body)
	}
	if w := upload(a, "/data/restore", data, a.token); w.Code != 200 {
		t.Fatal("preview failed", w.Code, w.Body)
	}
	key := a.transfers.Pending.Key
	a.store.saveStation(ctx, Station{Callsign: "ZZ0RECENT"})
	form := url.Values{"csrf": {a.token}, "key": {key}, "confirm": {"yes"}}
	if w := request(a, "POST", "/data/confirm", form); w.Code != 409 {
		t.Fatal("stale preview accepted", w.Code, w.Body)
	}
	if w := upload(a, "/data/restore", data, a.token); w.Code != 200 {
		t.Fatal("second preview", w.Code, w.Body)
	}
	form.Set("key", a.transfers.Pending.Key)
	oldToken := a.token
	if w := request(a, "POST", "/data/confirm", form); w.Code != 303 {
		t.Fatal("restore", w.Code, w.Body)
	}
	if a.token == oldToken {
		t.Fatal("old forms still authorized")
	}
	if _, err := a.store.lookup(ctx, "ZZ0RECENT"); err != sql.ErrNoRows {
		t.Fatal("restore did not replace data")
	}
	backups, err := a.recoveryFiles()
	if err != nil || len(backups) != 1 {
		t.Fatal("no recovery backup", backups, err)
	}
	recovery := filepath.Join(a.store.dataDirectory(), "backups", backups[0])
	if err = validateBackup(ctx, recovery); err != nil {
		t.Fatal(err)
	}
	saved, _ := openStore(ctx, recovery)
	defer saved.db.Close()
	if _, err = saved.lookup(ctx, "ZZ0RECENT"); err != nil {
		t.Fatal("pre-restore data not preserved")
	}
	if w := request(a, "POST", "/station", url.Values{"csrf": {oldToken}, "callsign": {"ZZ0STALE"}}); w.Code != 403 {
		t.Fatal("old form accepted")
	}
	if w := request(a, "GET", "/static/backups/"+backups[0], nil); w.Code != 404 {
		t.Fatal("backup exposed as static")
	}
	if w := request(a, "POST", "/data/recovery", url.Values{"csrf": {a.token}, "name": {"../../notebook.db"}}); w.Code != 400 {
		t.Fatal("path traversal")
	}
	if w := request(a, "POST", "/data/recovery", url.Values{"csrf": {a.token}, "name": {backups[0]}}); w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("recovery download", w.Code, w.Body)
	}
}

func TestRestoreFailureRollsBackAndVersionFiveUpgrade(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	s.saveStation(ctx, Station{Callsign: "ZZ0KEEP", Notes: "Keep"})
	source := testStore(t)
	source.saveStation(ctx, Station{Callsign: "ZZ0OTHER"})
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := source.backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("CREATE TRIGGER simulated_write_error BEFORE INSERT ON stations BEGIN SELECT RAISE(ABORT,'simulated write error'); END"); err != nil {
		t.Fatal(err)
	}
	if err := s.restore(ctx, path); err == nil {
		t.Fatal("failed write was ignored")
	}
	st, err := s.lookup(ctx, "ZZ0KEEP")
	if err != nil || st.Notes != "Keep" {
		t.Fatal("partial restore committed", st, err)
	}
	s.db.Exec("DROP TRIGGER simulated_write_error")
	old := recordsVersionFour(t)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(recordsSchema); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err = validateBackup(ctx, old); err != nil {
		t.Fatal("schema 5 rejected", err)
	}
	if err = s.restore(ctx, old); err != nil {
		t.Fatal("schema 5 upgrade failed", err)
	}
	c, err := s.contact(ctx, 11)
	if err != nil || c.StationID != 7 || c.Notes != "Contact note" {
		t.Fatal("old history changed", c, err)
	}
}
func TestTransferCancelAndExpiry(t *testing.T) {
	a := testApp(t)
	defer a.closeTransfers()
	data := []byte("<CALL:6>ZZ0ADI<QSO_DATE:8>20261023<TIME_ON:4>1230<MODE:2>FM<EOR>")
	if w := upload(a, "/data/import", data, a.token); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	p := a.transfers.Pending
	dir := filepath.Dir(p.Path)
	if w := request(a, "POST", "/data/cancel", url.Values{"csrf": {a.token}, "key": {p.Key}}); w.Code != 303 {
		t.Fatal(w.Code)
	}
	if a.transfers.Pending != nil {
		t.Fatal("cancelled preview retained")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("private upload not removed", err)
	}
	upload(a, "/data/import", data, a.token)
	p = a.transfers.Pending
	p.Expires = p.Expires.Add(-time.Hour)
	if w := request(a, "POST", "/data/confirm", url.Values{"csrf": {a.token}, "key": {p.Key}, "confirm": {"yes"}}); w.Code != 409 {
		t.Fatal("expired preview accepted", w.Code)
	}
	request(a, "GET", "/data", nil)
	if a.transfers.Pending != nil {
		t.Fatal("expired upload not cleaned")
	}
}

func TestStreamingUploadLimitsAndUnexpectedFields(t *testing.T) {
	a := testApp(t)
	defer a.closeTransfers()
	// Oversize ADIF is refused before parsing and leaves no pending upload.
	if w := upload(a, "/data/import", bytes.Repeat([]byte(" "), (16<<20)+1), a.token); w.Code != 400 {
		t.Fatal("oversize ADIF", w.Code)
	}
	if a.transfers.Pending != nil {
		t.Fatal("oversize preview retained")
	}
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	part, _ := m.CreateFormFile("file", "untrusted.db")
	part.Write([]byte("notebook"))
	m.WriteField("csrf", a.token)
	m.Close()
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/data/restore", &body)
	r.Header.Set("Content-Type", m.FormDataContentType())
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("file accepted before CSRF", w.Code)
	}
	entries, err := os.ReadDir(a.store.dataDirectory())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".qsobook-transfer-") {
			t.Fatal("rejected upload left staging data")
		}
	}
}

func TestBackupPreservesHistoryAcrossHostClockAdjustment(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.db.Exec("INSERT INTO rounds(name,started_at,ended_at) VALUES('Example','2026-10-23T12:00:00Z','2026-10-23T11:59:00Z')")
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := s.backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := validateBackup(ctx, path); err != nil {
		t.Fatal("valid history rejected after clock adjustment", err)
	}
}
