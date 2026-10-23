package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func adifFile(t *testing.T, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "contacts.adi")
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestADIFRoundTripPreservesUnicodeAndSnapshots(t *testing.T) {
	ctx := context.Background()
	source := testStore(t)
	qid, err := source.saveRecord(ctx, Station{Callsign: "ZZ0ADI", Name: "José <Example>", City: "São João", Country: "Brasil", Notes: "Permanent private note"}, "DMR", "70 cm", "Rodada fictícia", "Conversa com acentuação\n<eor>", strings.Repeat("a", 32), true)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = exportADI(ctx, source, &output); err != nil {
		t.Fatal(err)
	}
	for _, b := range output.Bytes() {
		if b > 127 {
			t.Fatal("non-ASCII ADI output")
		}
	}
	if strings.Contains(output.String(), "Permanent private note") || !strings.Contains(output.String(), "<MODE:12>DIGITALVOICE") || !strings.Contains(output.String(), "<SUBMODE:3>DMR") {
		t.Fatal("bad export mapping", output.String())
	}
	target := testStore(t)
	target.saveStation(ctx, Station{Callsign: "ZZ0ADI", Name: "Current name", City: "Current city", Notes: "Keep permanent"})
	preview, err := target.previewADIF(ctx, adifFile(t, output.String()))
	if err != nil || len(preview.Contacts) != 1 || preview.Duplicates != 0 {
		t.Fatal("preview", preview, err)
	}
	if n, err := target.importADIF(ctx, preview); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	contacts, _ := target.contacts(ctx, "")
	old, _ := source.contact(ctx, qid)
	if len(contacts) != 1 || portable(contacts[0]) != portable(old) {
		t.Fatal("round trip data changed", contacts)
	}
	st, _ := target.lookup(ctx, "ZZ0ADI")
	if st.Name != "Current name" || st.City != "Current city" || st.Notes != "Keep permanent" {
		t.Fatal("registration overwritten", st)
	}
	if n, err := target.importADIF(ctx, preview); err != nil || n != 0 {
		t.Fatal("replay duplicated", n, err)
	}
	target.moveTrash(ctx, "contact", contacts[0].ID, false)
	preview, err = target.previewADIF(ctx, adifFile(t, output.String()))
	if err != nil || preview.Duplicates != 1 || len(preview.Contacts) != 0 {
		t.Fatal("trash resurrected", preview, err)
	}
}
func TestADIFPreviewDuplicatesUnknownFieldsAndTransaction(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	record := "<CALL:6>ZZ0ADI<QSO_DATE:8>20261023<TIME_ON:4>1230<MODE:2>FM<BAND:3>2m <RST_SENT:2>59<EOR>"
	// Length-counted fields retain '>' and '<EOR>' as text, never delimiters.
	record = strings.Replace(record, "<BAND:3>2m ", "<BAND:2>2m<COMMENT:5><EOR>", 1)
	preview, err := s.previewADIF(ctx, adifFile(t, "Example<ADIF_VER:5>3.1.6<EOH>"+record+record))
	if err != nil || len(preview.Contacts) != 1 || preview.Duplicates != 1 || strings.Join(preview.Unsupported, ",") != "RST_SENT" {
		t.Fatal("preview", preview, err)
	}
	if preview.Contacts[0].Notes != "<EOR>" || preview.Contacts[0].OccurredAt != time.Date(2026, 10, 23, 12, 30, 0, 0, time.UTC) {
		t.Fatal("bad parsing", preview.Contacts)
	}
	// A later invalid row rolls back earlier inserts and registration changes.
	broken := *preview
	broken.Contacts = append(append([]PortableContact{}, preview.Contacts...), PortableContact{Callsign: "INVALID SPACE"})
	if n, err := s.importADIF(ctx, &broken); err == nil || n != 0 {
		t.Fatal("invalid import committed")
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM stations").Scan(&count)
	if count != 0 {
		t.Fatal("partial registration committed")
	}
	if n, err := s.importADIF(ctx, preview); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	list, _ := s.contacts(ctx, "")
	if list[0].Band != "2 m" {
		t.Fatal("bad band", list)
	}
}
func TestADIFRejectsMalformedWholeFile(t *testing.T) {
	good := "<CALL:6>ZZ0ADI<QSO_DATE:8>20261023<TIME_ON:6>123000<MODE:2>FM<EOR>"
	for _, data := range []string{"", good + "<CALL:99>short", strings.Replace(good, "20261023", "20261399", 1), good + strings.Replace(good, "<MODE:2>FM", "", 1), strings.Replace(good, "<CALL:6>", "<CALL:-1>", 1), strings.Replace(good, "ZZ0ADI", "ZZ0ÁDI", 1), "<?xml version='1.0'?><ADX/>", good + "<EOH>", strings.Replace(good, "<MODE:2>FM", "<MODE:2>FM<MODE:2>AM", 1)} {
		s := testStore(t)
		if _, err := s.previewADIF(context.Background(), adifFile(t, data)); err == nil {
			t.Fatal("accepted malformed input", data)
		}
	}
}
func TestADIFHTTPPreviewDoesNotImportUntilConfirmed(t *testing.T) {
	a := testApp(t)
	defer a.closeTransfers()
	data := []byte("<CALL:6>ZZ0ADI<QSO_DATE:8>20261023<TIME_ON:4>1230<MODE:2>FM<EOR>")
	if w := upload(a, "/data/import", data, a.token); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	list, _ := a.store.contacts(context.Background(), "")
	if len(list) != 0 {
		t.Fatal("preview wrote QSO")
	}
	key := a.transfers.Pending.Key
	f := url.Values{"csrf": {a.token}, "key": {key}, "confirm": {"yes"}}
	if w := request(a, "POST", "/data/confirm", f); w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(a, "POST", "/data/confirm", f); w.Code != 409 {
		t.Fatal("confirmation replay", w.Code)
	}
	list, _ = a.store.contacts(context.Background(), "")
	if len(list) != 1 {
		t.Fatal("wrong count", len(list))
	}
	if w := request(a, "POST", "/data/export", url.Values{"csrf": {a.token}}); w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf("<CALL:%d>ZZ0ADI", 6)) {
		t.Fatal("download", w.Code, w.Body)
	}
}
