package main

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func accessPassword(t *testing.T, password string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte(password), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOptionalPasswordAndExplicitHosts(t *testing.T) {
	a := testApp(t)
	password := "synthetic-test-password-only"
	file := accessPassword(t, password+"\n")
	for _, hosts := range []string{"*", "http://tortuga.local:8088", "tortuga.local", "tortuga.local:0", "tortuga.local:65536", "bad/name:8088", "tortuga.local:abc"} {
		if err := a.configureAccess(file, hosts); err == nil {
			t.Errorf("accepted unsafe host list %q", hosts)
		}
	}
	if err := a.configureAccess(file, "Tortuga.local:8088,192.168.1.10:8088"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/records", "/location", "/round", "/lookup?callsign=ZZ0TEST", "/static/app.js", "/license", "/language", "/data", "/data/backup", "/data/restore", "/data/import", "/data/confirm", "/data/export", "/data/recovery", "/data/cancel", "/trash", "/trash/confirm?kind=station&id=1", "/map/edit", "/about"} {
		r := httptest.NewRequest("GET", "http://tortuga.local:8088"+path, nil)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 401 || !strings.Contains(w.Header().Get("WWW-Authenticate"), "Basic") || w.Header().Get("X-Frame-Options") != "DENY" || strings.Contains(w.Body.String(), a.token) {
			t.Errorf("unauthenticated %s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, tc := range []struct {
		host, user, password string
		code                 int
	}{
		{"tortuga.local:8088", "qsobook", password, 200},
		{"192.168.1.10:8088", "qsobook", password, 200},
		{"127.0.0.1:8088", "qsobook", password, 200},
		{"tortuga.local:8088", "qsobook", "wrong", 401},
		{"tortuga.local:8088", "other", password, 401},
		{"evil.example:8088", "qsobook", password, 403},
		{"tortuga.local:8089", "qsobook", password, 403},
	} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		r.SetBasicAuth(tc.user, tc.password)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Errorf("%s %s: got %d want %d", tc.host, tc.user, w.Code, tc.code)
		}
	}
	form := url.Values{"csrf": {a.token}, "language": {"en"}, "return_to": {"/round"}}
	for _, tc := range []struct {
		origin, token string
		code          int
	}{
		{"http://tortuga.local:8088", a.token, 303},
		{"http://evil.example:8088", a.token, 403},
		{"http://tortuga.local:8089", a.token, 403},
		{"http://tortuga.local:8088", "wrong", 403},
	} {
		form.Set("csrf", tc.token)
		r := httptest.NewRequest("POST", "http://tortuga.local:8088/language", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", tc.origin)
		r.SetBasicAuth("qsobook", password)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Errorf("origin=%q got %d want %d", tc.origin, w.Code, tc.code)
		}
	}
}

func TestAccessPasswordValidationAndLocalDefaults(t *testing.T) {
	a := testApp(t)
	for _, password := range []string{"", "short", strings.Repeat("a", 257), "sixteen-characters\nextra", "sixteen-characters\x00"} {
		if err := a.configureAccess(accessPassword(t, password), "tortuga.local:8088"); err == nil {
			t.Error("accepted invalid password")
		}
	}
	if err := a.configureAccess(filepath.Join(t.TempDir(), "missing"), ""); err == nil {
		t.Error("accepted missing password file")
	}
	if err := a.configureAccess(t.TempDir(), ""); err == nil {
		t.Error("accepted directory password")
	}
	if err := a.configureAccess("", ""); err != nil {
		t.Fatal(err)
	}
	if !a.acceptsHost("localhost:8080") || !a.acceptsHost("[::1]:8080") || a.acceptsHost("tortuga.local:8088") {
		t.Fatal("local host protection changed")
	}
	if w := request(a, "GET", "/", nil); w.Code != 200 {
		t.Fatalf("local mode: %d", w.Code)
	}
}

func TestNetworkAccessWithoutPassword(t *testing.T) {
	a := testApp(t)
	if err := a.configureAccess("", "tortuga.local:18088,100.101.102.103:18088"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host string
		code int
	}{
		{"tortuga.local:18088", 200}, {"100.101.102.103:18088", 200},
		{"evil.example:18088", 403}, {"tortuga.local:18089", 403},
	} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.code || w.Header().Get("WWW-Authenticate") != "" || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("host %s: status %d headers %v", tc.host, w.Code, w.Header())
		}
	}
	for _, tc := range []struct {
		origin, token string
		code          int
	}{
		{"http://100.101.102.103:18088", a.token, 303},
		{"http://evil.example:18088", a.token, 403},
		{"http://100.101.102.103:18089", a.token, 403},
		{"http://100.101.102.103:18088", "wrong", 403},
	} {
		form := url.Values{"csrf": {tc.token}, "language": {"en"}, "return_to": {"/round"}}
		r := httptest.NewRequest("POST", "http://100.101.102.103:18088/language", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Errorf("origin %s: got %d want %d", tc.origin, w.Code, tc.code)
		}
	}
	// A configured but invalid password must fail, never silently disable authentication.
	file := accessPassword(t, "synthetic-valid-password")
	if err := a.configureAccess(file, "tortuga.local:18088"); err != nil {
		t.Fatal(err)
	}
	if err := a.configureAccess(filepath.Join(t.TempDir(), "missing"), "tortuga.local:18088"); err == nil {
		t.Fatal("missing password accepted")
	}
	r := httptest.NewRequest("GET", "http://tortuga.local:18088/", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("failed configuration disabled authentication: %d", w.Code)
	}
}
