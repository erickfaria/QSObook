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
	for _, mutation := range []string{"CREATE TRIGGER malicious AFTER INSERT ON stations BEGIN DELETE FROM qsos;END", "CREATE VIEW malicious AS SELECT * FROM stations", "PRAGMA user_version=999", "UPDATE qsos SET occurred_at='invalid'", "INSERT INTO participants(round_id,station_id,mode,notes) VALUES(999,999,'FM','')"} {
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
