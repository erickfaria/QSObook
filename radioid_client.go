package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const radioIDEndpoint = "https://radioid.net/api/dmr/user/"

var errRadioIDUnavailable = errors.New("RadioID unavailable")

type radioIDClient struct {
	endpoint string
	http     *http.Client
}

type radioIDResult struct {
	Station Station
	IDs     []int64
	Found   bool
	RetryAt time.Time
}

func newRadioIDClient() *radioIDClient {
	return &radioIDClient{endpoint: radioIDEndpoint, http: &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *radioIDClient) lookup(ctx context.Context, callsign string) (radioIDResult, error) {
	result := radioIDResult{Station: Station{Callsign: callsign}}
	if err := result.Station.validate(); err != nil {
		return result, err
	}
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return result, errRadioIDUnavailable
	}
	q := u.Query()
	q.Set("callsign", result.Station.Callsign)
	q.Set("callsign_sel", "=")
	q.Set("per_page", "200")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return result, errRadioIDUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "QSObook/"+appVersion+" (+https://github.com/erickfaria/QSObook/issues)")
	response, err := c.http.Do(req)
	if err != nil {
		return result, errRadioIDUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if seconds, e := strconv.ParseInt(response.Header.Get("Retry-After"), 10, 32); e == nil && seconds > 0 {
			result.RetryAt = time.Now().Add(time.Duration(seconds) * time.Second)
		} else if when, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil {
			result.RetryAt = when
		}
		return result, errRadioIDUnavailable
	}
	const limit = 256 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(body) > limit {
		return result, errRadioIDUnavailable
	}
	var payload struct {
		Results *[]struct {
			Callsign, Fname, Surname, Name, City, State, Country string
			RadioID                                              json.RawMessage `json:"radio_id"`
			ID                                                   json.RawMessage
		} `json:"results"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Results == nil || len(*payload.Results) > 200 {
		return result, errRadioIDUnavailable
	}
	ids := map[int64]bool{}
	var chosen int64
	for _, row := range *payload.Results {
		if strings.ToUpper(strings.TrimSpace(row.Callsign)) != result.Station.Callsign {
			continue
		}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = strings.TrimSpace(strings.TrimSpace(row.Fname) + " " + strings.TrimSpace(row.Surname))
		}
		st := Station{Callsign: result.Station.Callsign, Name: name, City: row.City, State: row.State, Country: row.Country}
		if st.validate() != nil {
			return result, errRadioIDUnavailable
		}
		id := radioIDNumber(row.RadioID)
		if id == 0 {
			id = radioIDNumber(row.ID)
		}
		if id > 0 {
			ids[id] = true
		}
		// Pick one consistent profile, deterministically by its lowest known ID.
		// Do not combine conflicting cities/states from separate registrations.
		if !result.Found || (id > 0 && (chosen == 0 || id < chosen)) {
			result.Station, chosen = st, id
		}
		result.Found = true
	}
	for id := range ids {
		result.IDs = append(result.IDs, id)
	}
	sort.Slice(result.IDs, func(i, j int) bool { return result.IDs[i] < result.IDs[j] })
	return result, nil
}

func radioIDNumber(raw json.RawMessage) int64 {
	value := strings.Trim(string(raw), `"`)
	if len(value) == 0 || len(value) > 10 {
		return 0
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0
		}
	}
	id, _ := strconv.ParseInt(value, 10, 64)
	return id
}
