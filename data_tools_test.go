package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTrashPreservesLinksSnapshotsAndReplayKeys(t *testing.T) {
	a := testApp(t)
	s := a.store
	ctx := context.Background()
	st := Station{Callsign: "ZZ0TRASH", Name: "Example", City: "São Paulo", Notes: "Permanent"}
	qid, err := s.saveRecord(ctx, st, "FM", "2 m", "Example round", "Conversation", strings.Repeat("a", 32), true)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.contact(ctx, qid)
	if err = s.saveParticipant(ctx, 1, 0, st, "FM", "Round note"); err != nil {
		t.Fatal(err)
	}
	var participant int64
	s.db.QueryRow("SELECT id FROM participants").Scan(&participant)
	s.db.Exec("UPDATE participants SET qso_id=? WHERE id=?", qid, participant)
	for i := 0; i < 2; i++ {
		if err = s.moveTrash(ctx, "contact", qid, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.contact(ctx, qid); err != sql.ErrNoRows {
		t.Fatal("trashed contact still visible", err)
	}
	list, _ := s.contacts(ctx, "")
	cities, _ := s.contactCities(ctx)
	if len(list) != 0 || len(cities) != 0 {
		t.Fatal("trash leaked into notebook/map")
	}
	if w := request(a, "GET", "/", nil); !strings.Contains(w.Body.String(), "<strong>0</strong>") {
		t.Fatal("dashboard includes trash")
	}
	got, err := s.saveRecord(ctx, st, "DMR", "70 cm", "Changed", "Changed", strings.Repeat("a", 32), true)
	if err != nil || got != qid {
		t.Fatal("replay changed identity", got, err)
	}
	if err = s.moveTrash(ctx, "station", c.StationID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.lookup(ctx, st.Callsign); err != sql.ErrNoRows {
		t.Fatal("archived station in lookup")
	}
	if _, err = s.saveRecord(ctx, st, "FM", "", "", "", strings.Repeat("b", 32), true); err == nil {
		t.Fatal("archived callsign reused")
	}
	if err = s.moveTrash(ctx, "contact", qid, true); err != nil {
		t.Fatal(err)
	}
	after, err := s.contact(ctx, qid)
	if err != nil || after != c {
		t.Fatal("snapshot changed", after, err)
	}
	var linked int64
	if err = s.db.QueryRow("SELECT qso_id FROM participants WHERE id=?", participant).Scan(&linked); err != nil || linked != qid {
		t.Fatal("participation link lost", err)
	}
	if err = s.moveTrash(ctx, "station", c.StationID, true); err != nil {
		t.Fatal(err)
	}
	if got, err := s.lookup(ctx, st.Callsign); err != nil || got.Notes != st.Notes {
		t.Fatal("registration changed", got, err)
	}
}
func TestPaginatedAccentAndCitySearch(t *testing.T) {
	a := testApp(t)
	s := a.store
	ctx := context.Background()
	for i := 0; i < 205; i++ {
		if _, err := s.saveStation(ctx, Station{Callsign: fmt.Sprintf("ZZ%03d", i), Name: "José", City: "São João"}); err != nil {
			t.Fatal(err)
		}
	}
	f := ListFilter{Query: "SAO JOAO", Page: 5}
	list, total, err := s.stationPage(ctx, f)
	if err != nil || total != 205 || len(list) != 5 || list[4].Callsign != "ZZ204" {
		t.Fatal("last page", len(list), total, err)
	}
	f.Query = "JOSE"
	f.Page = 1
	list, total, err = s.stationPage(ctx, f)
	if err != nil || total != 205 || len(list) != 50 {
		t.Fatal("accent search", err, total)
	}
	w := request(a, "GET", "/stations?q=sao+joao&page=5", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "ZZ204") || !strings.Contains(w.Body.String(), "page=4") {
		t.Fatal("page navigation", w.Code)
	}
	for _, path := range []string{"/history?from=bad", "/history?from=2026-10-23&to=2026-10-20", "/stations?page=-1"} {
		if w := request(a, "GET", path, nil); w.Code != 400 {
			t.Fatal("invalid filter", path, w.Code)
		}
	}
	qid, err := s.createContact(ctx, Station{Callsign: "ZZDATE", Name: "Érica", City: "Bogotá"}, "FM", "")
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec("UPDATE qsos SET occurred_at='2026-10-24T02:59:59Z' WHERE id=?", qid)
	f, err = a.listFilter(httptest.NewRequest("GET", "/history?q=bogota&from=2026-10-23&to=2026-10-23", nil))
	if err != nil {
		t.Fatal(err)
	}
	contacts, total, err := s.contactPage(ctx, f)
	if err != nil || total != 1 || len(contacts) != 1 {
		t.Fatal("local date boundary", total, err)
	}
	f, _ = a.listFilter(httptest.NewRequest("GET", "/history?from=2026-10-24&to=2026-10-24", nil))
	_, total, err = s.contactPage(ctx, f)
	if err != nil || total != 0 {
		t.Fatal("wrong day", total, err)
	}
}
func TestTrashHTTPRequiresExplicitProtectedWrite(t *testing.T) {
	a := testApp(t)
	id, _ := a.store.saveStation(context.Background(), Station{Callsign: "ZZSAFE"})
	path := fmt.Sprintf("/trash/confirm?kind=station&id=%d", id)
	if w := request(a, "GET", path, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := a.store.lookup(context.Background(), "ZZSAFE"); err != nil {
		t.Fatal("GET deleted data")
	}
	f := url.Values{"kind": {"station"}, "id": {fmt.Sprint(id)}, "action": {"delete"}}
	if w := request(a, "POST", "/trash", f); w.Code != 403 {
		t.Fatal("no CSRF", w.Code)
	}
	f.Set("csrf", a.token)
	if w := request(a, "POST", "/trash", f); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	f.Set("action", "restore")
	if w := request(a, "POST", "/trash", f); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := a.store.lookup(context.Background(), "ZZSAFE"); err != nil {
		t.Fatal(err)
	}
}
