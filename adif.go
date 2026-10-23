package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// This portable payload preserves UTF-8 and QSObook-specific values in ASCII ADI.
// It contains contact snapshots only, never internal IDs or permanent notes.
type PortableContact struct {
	Callsign, Name, City, State, Country, Mode, Band, RoundName, Notes string
	OccurredAt                                                         time.Time
}

func portable(c Contact) PortableContact {
	return PortableContact{c.Callsign, c.Name, c.City, c.State, c.Country, c.Mode, c.Band, c.RoundName, c.Notes, c.OccurredAt}
}
func (p PortableContact) contact() Contact {
	return Contact{Station: Station{Callsign: p.Callsign, Name: p.Name, City: p.City, State: p.State, Country: p.Country, Notes: p.Notes}, Mode: p.Mode, Band: p.Band, RoundName: p.RoundName, OccurredAt: p.OccurredAt}
}

type ADIFPreview struct {
	Contacts            []PortableContact
	Sample              []PortableContact
	Duplicates, Blocked int
	Unsupported         []string
}

var errADIFInvalid = errors.New("ADIF inválido. Use um arquivo ADI ASCII com CALL, QSO_DATE, TIME_ON e MODE. Nenhum contato foi importado.")

func writeADIField(w io.Writer, name, value string) error {
	if value == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "<%s:%d>%s ", name, len(value), value)
	return err
}
func asciiText(value string) string {
	var out strings.Builder
	for _, r := range value {
		if r >= 32 && r <= 126 {
			out.WriteRune(r)
			continue
		}
		if r == '\n' {
			out.WriteString("\r\n")
			continue
		}
		if r == '\r' || unicode.Is(unicode.Mn, r) {
			continue
		}
		fold := foldSearch(string(r))
		if unicode.IsUpper(r) {
			fold = strings.ToUpper(fold)
		}
		valid := true
		for _, f := range fold {
			if f < 32 || f > 126 {
				valid = false
				break
			}
		}
		if valid {
			out.WriteString(fold)
		} else {
			out.WriteByte('?')
		}
	}
	return out.String()
}
func adifMode(mode string) (string, string) {
	switch strings.ToUpper(mode) {
	case "AM", "FM", "SSB", "CW", "RTTY", "PSK", "MFSK", "FT8", "JT65", "JT9", "OLIVIA", "HELL", "FSK441", "MSK144":
		return strings.ToUpper(mode), ""
	case "FT4", "Q65":
		return "MFSK", strings.ToUpper(mode)
	case "DMR", "C4FM":
		return "DIGITALVOICE", strings.ToUpper(mode)
	case "D-STAR", "DSTAR":
		return "DIGITALVOICE", "DSTAR"
	case "DIGITAL", "DIGITALVOICE":
		return "DIGITALVOICE", ""
	case "USB", "LSB":
		return "SSB", strings.ToUpper(mode)
	}
	return "", ""
}
func standardBand(band string) string {
	key := strings.ToLower(strings.ReplaceAll(band, " ", ""))
	for _, b := range strings.Fields("2190m 630m 560m 160m 80m 60m 40m 30m 20m 17m 15m 12m 10m 8m 6m 5m 4m 2m 1.25m 70cm 33cm 23cm 13cm 9cm 6cm 3cm 1.25cm 6mm 4mm 2.5mm 2mm 1mm submm") {
		if key == b {
			return key
		}
	}
	return ""
}
func displayBand(band string) string {
	for _, suffix := range []string{"cm", "mm", "m"} {
		if strings.HasSuffix(band, suffix) {
			return strings.TrimSuffix(band, suffix) + " " + suffix
		}
	}
	return band
}
func exportADI(ctx context.Context, s *Store, w io.Writer) error {
	if _, err := io.WriteString(w, "QSObook contact export\r\n<ADIF_VER:5>3.1.6 <PROGRAMID:7>QSObook <EOH>\r\n"); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+contactColumns+" FROM qsos WHERE deleted_at IS NULL ORDER BY occurred_at,id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(portable(c))
		if err != nil {
			return err
		}
		mode, sub := adifMode(c.Mode)
		fields := [][2]string{{"CALL", c.Callsign}, {"QSO_DATE", c.OccurredAt.UTC().Format("20060102")}, {"TIME_ON", c.OccurredAt.UTC().Format("150405")}, {"MODE", mode}, {"SUBMODE", sub}, {"BAND", standardBand(c.Band)}, {"NAME", asciiText(c.Name)}, {"QTH", asciiText(c.City)}, {"STATE", asciiText(c.State)}, {"COUNTRY", asciiText(c.Country)}, {"COMMENT", asciiText(c.Notes)}, {"APP_QSOBOOK_DATA", base64.StdEncoding.EncodeToString(payload)}}
		for _, field := range fields {
			if err = writeADIField(w, field[0], field[1]); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(w, "<EOR>\r\n"); err != nil {
			return err
		}
	}
	return rows.Err()
}
func parseADI(data []byte) ([]map[string]string, error) {
	// ADI is ASCII by specification; ADX/UTF-8 are not silently misdecoded.
	for _, b := range data {
		if b > 127 || b == 0 {
			return nil, errADIFInvalid
		}
	}
	text := string(data)
	if strings.HasPrefix(strings.TrimSpace(text), "<?xml") {
		return nil, errADIFInvalid
	}
	records := []map[string]string{}
	current := map[string]string{}
	header := false
	for len(text) > 0 {
		start := strings.IndexByte(text, '<')
		if start < 0 {
			break
		}
		text = text[start+1:]
		end := strings.IndexByte(text, '>')
		if end < 0 || end > 100 {
			return nil, errADIFInvalid
		}
		tag := strings.ToUpper(text[:end])
		text = text[end+1:]
		if tag == "EOH" {
			if header || len(records) > 0 {
				return nil, errADIFInvalid
			}
			header = true
			current = map[string]string{}
			continue
		}
		if tag == "EOR" {
			if len(current) == 0 {
				return nil, errADIFInvalid
			}
			records = append(records, current)
			current = map[string]string{}
			if len(records) > 10000 {
				return nil, errADIFInvalid
			}
			continue
		}
		parts := strings.Split(tag, ":")
		if len(parts) < 2 || len(parts) > 3 || len(parts[0]) == 0 || len(parts[0]) > 64 {
			return nil, errADIFInvalid
		}
		for _, r := range parts[0] {
			if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
				return nil, errADIFInvalid
			}
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 0 || n > 32768 || n > len(text) {
			return nil, errADIFInvalid
		}
		if _, exists := current[parts[0]]; exists {
			return nil, errADIFInvalid
		}
		if len(current) >= 128 {
			return nil, errADIFInvalid
		}
		current[parts[0]] = text[:n]
		text = text[n:]
	}
	if len(current) > 0 || len(records) == 0 {
		return nil, errADIFInvalid
	}
	return records, nil
}
func fromADIF(fields map[string]string) (PortableContact, error) {
	var p PortableContact
	if original := fields["APP_QSOBOOK_DATA"]; original != "" {
		data, err := base64.StdEncoding.DecodeString(original)
		if err != nil || json.Unmarshal(data, &p) != nil {
			return p, errADIFInvalid
		}
	} else {
		clock := fields["TIME_ON"]
		if len(clock) == 4 {
			clock += "00"
		}
		if len(clock) != 6 || len(fields["QSO_DATE"]) != 8 {
			return p, errADIFInvalid
		}
		stamp, err := time.Parse("20060102150405", fields["QSO_DATE"]+clock)
		if err != nil {
			return p, errADIFInvalid
		}
		mode := fields["MODE"]
		if fields["SUBMODE"] != "" {
			mode = fields["SUBMODE"]
		}
		if strings.EqualFold(mode, "DSTAR") {
			mode = "D-Star"
		}
		notes := fields["COMMENT"]
		if fields["NOTES"] != "" {
			if notes != "" {
				notes += "\n"
			}
			notes += fields["NOTES"]
		}
		p = PortableContact{Callsign: fields["CALL"], Name: fields["NAME"], City: fields["QTH"], State: fields["STATE"], Country: fields["COUNTRY"], Mode: mode, Band: displayBand(strings.ToLower(fields["BAND"])), Notes: notes, OccurredAt: stamp}
	}
	c := p.contact()
	if c.Station.validate() != nil || validateMode(c.Mode) != nil || validateRecordDetails(c.Band, c.RoundName) != nil || p.OccurredAt.IsZero() || p.OccurredAt.Year() < 1930 || p.OccurredAt.Year() > 9999 {
		return p, errADIFInvalid
	}
	// The extension may not redirect a record to a different standard callsign.
	if fields["CALL"] != "" && !strings.EqualFold(strings.TrimSpace(fields["CALL"]), c.Callsign) {
		return p, errADIFInvalid
	}
	p.Mode = strings.TrimSpace(p.Mode)
	p.Band = strings.TrimSpace(p.Band)
	p.Callsign = c.Callsign
	p.OccurredAt = p.OccurredAt.UTC()
	return p, nil
}
func duplicateIdentity(p PortableContact) string {
	return p.Callsign + "\x00" + p.OccurredAt.UTC().Format("2006-01-02T15:04:05") + "\x00" + strings.ToUpper(p.Mode) + "\x00" + strings.ReplaceAll(strings.ToLower(p.Band), " ", "")
}

type sqlQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func adifKey(p PortableContact) string {
	return fmt.Sprintf("adif:%x", sha256.Sum256([]byte(duplicateIdentity(p))))
}
func adifDuplicate(ctx context.Context, q sqlQuerier, p PortableContact) (bool, error) {
	var count int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM qsos WHERE (callsign=? AND strftime('%Y-%m-%dT%H:%M:%S',substr(occurred_at,1,19)||CASE WHEN substr(occurred_at,-1)='Z' THEN 'Z' ELSE substr(occurred_at,-6) END)=? AND upper(mode)=upper(?) AND replace(lower(band),' ','')=?) OR submission_key=?`, p.Callsign, p.OccurredAt.UTC().Format("2006-01-02T15:04:05"), p.Mode, strings.ReplaceAll(strings.ToLower(p.Band), " ", ""), adifKey(p)).Scan(&count)
	return count > 0, err
}
func (s *Store) previewADIF(ctx context.Context, path string) (*ADIFPreview, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, errADIFInvalid
	}
	fields, err := parseADI(data)
	if err != nil {
		return nil, err
	}
	preview := &ADIFPreview{}
	seen := map[string]bool{}
	unsupported := map[string]bool{}
	supported := map[string]bool{}
	for _, name := range strings.Fields("CALL QSO_DATE TIME_ON MODE SUBMODE BAND NAME QTH STATE COUNTRY COMMENT NOTES APP_QSOBOOK_DATA") {
		supported[name] = true
	}
	for i, row := range fields {
		p, err := fromADIF(row)
		if err != nil {
			return nil, fmt.Errorf("%w (%d)", errADIFInvalid, i+1)
		}
		for name := range row {
			if !supported[name] {
				unsupported[name] = true
			}
		}
		identity := duplicateIdentity(p)
		duplicate, err := adifDuplicate(ctx, s.db, p)
		if err != nil {
			return nil, err
		}
		if duplicate || seen[identity] {
			preview.Duplicates++
			continue
		}
		seen[identity] = true
		var archived bool
		err = s.db.QueryRowContext(ctx, "SELECT deleted_at IS NOT NULL FROM stations WHERE callsign=?", p.Callsign).Scan(&archived)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if archived {
			preview.Blocked++
			continue
		}
		preview.Contacts = append(preview.Contacts, p)
	}
	for name := range unsupported {
		preview.Unsupported = append(preview.Unsupported, name)
	}
	sort.Strings(preview.Unsupported)
	preview.Sample = preview.Contacts[:min(20, len(preview.Contacts))]
	return preview, nil
}
func (s *Store) importADIF(ctx context.Context, p *ADIFPreview) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	count := 0
	for _, p := range p.Contacts {
		duplicate, err := adifDuplicate(ctx, tx, p)
		if err != nil {
			return 0, err
		}
		if duplicate {
			continue
		}
		st := Station{Callsign: p.Callsign, Name: p.Name, City: p.City, State: p.State, Country: p.Country}
		if err := st.validate(); err != nil {
			return 0, err
		}
		id, err := mergeConversationStation(ctx, tx, st)
		if err != nil {
			return 0, err
		}
		key := adifKey(p)
		if _, err = tx.ExecContext(ctx, `INSERT INTO qsos(station_id,callsign,name,city,state,country,mode,band,round_name,notes,occurred_at,submission_key) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, p.Callsign, p.Name, p.City, p.State, p.Country, p.Mode, p.Band, p.RoundName, p.Notes, p.OccurredAt.Format("2006-01-02T15:04:05.000000000Z"), key); err != nil {
			return 0, err
		}
		count++
	}
	return count, tx.Commit()
}
func (a *App) downloadADIF(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp(a.store.dataDirectory(), ".qsobook-export-")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer os.RemoveAll(dir)
	path := dir + "/contacts.adi"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	err = exportADI(r.Context(), a.store, f)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		a.fail(w, r, fmt.Errorf("export failed"))
		return
	}
	a.sendFile(w, r, path, "qsobook.adi", "application/octet-stream")
}
