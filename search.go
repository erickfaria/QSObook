package main

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("qsobook_fold", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		value, _ := args[0].(string)
		return foldSearch(value), nil
	})
}

// Latin accents used by the four UI languages, plus decomposed combining marks.
// Values in the database are never normalized or rewritten.
// Immutable after initialization; reused by SQLite queries without per-rune tables.
var searchReplacer = strings.NewReplacer("à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "ā", "a", "ă", "a", "ą", "a", "ç", "c", "ć", "c", "ĉ", "c", "ċ", "c", "č", "c", "ď", "d", "đ", "d", "è", "e", "é", "e", "ê", "e", "ë", "e", "ē", "e", "ĕ", "e", "ė", "e", "ę", "e", "ě", "e", "ĝ", "g", "ğ", "g", "ġ", "g", "ģ", "g", "ĥ", "h", "ħ", "h", "ì", "i", "í", "i", "î", "i", "ï", "i", "ĩ", "i", "ī", "i", "ĭ", "i", "į", "i", "ı", "i", "ĵ", "j", "ķ", "k", "ĺ", "l", "ļ", "l", "ľ", "l", "ŀ", "l", "ł", "l", "ñ", "n", "ń", "n", "ņ", "n", "ň", "n", "ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "ō", "o", "ŏ", "o", "ő", "o", "ŕ", "r", "ŗ", "r", "ř", "r", "ś", "s", "ŝ", "s", "ş", "s", "š", "s", "ţ", "t", "ť", "t", "ŧ", "t", "ù", "u", "ú", "u", "û", "u", "ü", "u", "ũ", "u", "ū", "u", "ŭ", "u", "ů", "u", "ű", "u", "ų", "u", "ŵ", "w", "ý", "y", "ÿ", "y", "ŷ", "y", "ź", "z", "ż", "z", "ž", "z", "œ", "oe", "æ", "ae", "ß", "ss")

func foldSearch(value string) string {
	value = searchReplacer.Replace(strings.ToLower(value))
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, value)
}

type ListFilter struct {
	Query, City, State, Country, Mode, Band, From, To string
	Page                                              int
	Start, End                                        string
}
type Pagination struct {
	Page           int
	Total          int64
	Previous, Next string
}

const pageSize = 50

func (a *App) listFilter(r *http.Request) (ListFilter, error) {
	q := r.URL.Query()
	f := ListFilter{Query: q.Get("q"), City: q.Get("city"), State: q.Get("state"), Country: q.Get("country"), Mode: q.Get("mode"), Band: q.Get("band"), From: q.Get("from"), To: q.Get("to"), Page: 1}
	if len(f.Query) > 200 {
		return f, errors.New("Busca muito longa")
	}
	st := Station{Callsign: "SEARCH", City: f.City, State: f.State, Country: f.Country}
	if err := st.validate(); err != nil {
		return f, err
	}
	if len(f.Mode) > 40 || len(f.Band) > 40 {
		return f, errors.New("Filtro inválido")
	}
	if q.Get("page") != "" {
		page, err := strconv.Atoi(q.Get("page"))
		if err != nil || page < 1 || page > 1000000 {
			return f, errors.New("Página inválida")
		}
		f.Page = page
	}
	for i, raw := range []string{f.From, f.To} {
		if raw == "" {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", raw, a.location)
		if err != nil {
			return f, errors.New("Período inválido")
		}
		if i == 0 {
			f.Start = d.UTC().Format(time.RFC3339Nano)
		} else {
			f.End = d.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)
		}
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		return f, errors.New("Período inválido")
	}
	return f, nil
}
func pagination(r *http.Request, page int, total int64) Pagination {
	p := Pagination{Page: page, Total: total}
	link := func(n int) string {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(n))
		return r.URL.Path + "?" + q.Encode()
	}
	if page > 1 {
		p.Previous = link(page - 1)
	}
	if int64(page*pageSize) < total {
		p.Next = link(page + 1)
	}
	return p
}
func searchWhere(f ListFilter, contact bool) (string, []any) {
	term := foldSearch(strings.TrimSpace(f.Query))
	where := "deleted_at IS NULL AND (instr(qsobook_fold(callsign),?)>0 OR instr(qsobook_fold(name),?)>0 OR instr(qsobook_fold(city),?)>0)"
	args := []any{term, term, term}
	if contact {
		if f.City != "" {
			where += " AND city=? AND state=? AND country=?"
			args = append(args, f.City, f.State, f.Country)
		}
		where += ` AND (?='' OR upper(mode)=upper(?) OR (?='digital' AND upper(mode) IN ('DIGITAL','DMR','ECHOLINK','D-STAR','C4FM')) OR (?='other' AND upper(mode) NOT IN ('FM','AM','SSB','CW','DIGITAL','DMR','ECHOLINK','D-STAR','C4FM'))) AND (?='' OR band=?)`
		args = append(args, f.Mode, f.Mode, f.Mode, f.Mode, f.Band, f.Band)
		if f.Start != "" {
			where += " AND julianday(occurred_at)>=julianday(?)"
			args = append(args, f.Start)
		}
		if f.End != "" {
			where += " AND julianday(occurred_at)<julianday(?)"
			args = append(args, f.End)
		}
	}
	return where, args
}
func (s *Store) stationPage(ctx context.Context, f ListFilter) ([]Station, int64, error) {
	where, args := searchWhere(f, false)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM stations WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+stationColumns+" FROM stations WHERE "+where+" ORDER BY callsign LIMIT ? OFFSET ?", append(args, pageSize, (f.Page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []Station{}
	for rows.Next() {
		st, err := scanStation(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, st)
	}
	return list, total, rows.Err()
}
func (s *Store) contactPage(ctx context.Context, f ListFilter) ([]Contact, int64, error) {
	where, args := searchWhere(f, true)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM qsos WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+contactColumns+" FROM qsos WHERE "+where+" ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?", append(args, pageSize, (f.Page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []Contact{}
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, rows.Err()
}

// Retain only local, generated navigation destinations.
func dataMessage(message string) string { return "/data?message=" + url.QueryEscape(message) }
