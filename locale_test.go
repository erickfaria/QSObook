package main

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func languageRequest(a *App, language, method, path string, form url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(form.Encode()))
	if method == "POST" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "http://127.0.0.1:8080")
	}
	if language != "" {
		r.AddCookie(&http.Cookie{Name: "qsobook-language", Value: language})
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestTranslationCatalogsCoverUI(t *testing.T) {
	a := testApp(t)
	files, err := assets.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, file := range files {
		part, err := assets.ReadFile("templates/" + file.Name())
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, part...)
	}
	keys := browserMessages()
	for _, match := range regexp.MustCompile(`{{t "([^"]+)"}}`).FindAllStringSubmatch(string(data), -1) {
		keys = append(keys, match[1])
	}
	// Cover dynamically rendered page titles, validation failures and notices.
	keys = append(keys, "Seu caderno", "Revise os dados", "Detalhes e edição do QSO", "Editar participante", "Editar estação",
		"Participante salvo", "Participante removido da lista", "QSO registrado no caderno", "Rodada encerrada. Seus registros foram preservados.",
		"informe um indicativo de até 32 caracteres: letras, números, /, ponto ou hífen", "campo inválido: limite de 120 caracteres",
		"campo inválido: limite de 4000 caracteres", "informe um modo de até 40 caracteres", "observações inválidas: limite de 4000 caracteres",
		"nome da rodada inválido: limite de 120 caracteres", errRoundChanged.Error())
	placeholder := regexp.MustCompile(`\{\w+\}`)
	for _, language := range []string{"en", "es", "fr"} {
		catalog := a.translations[language]
		if len(catalog) != len(a.translations["en"]) {
			t.Errorf("%s catalog differs in size", language)
		}
		for key := range a.translations["en"] {
			if catalog[key] == "" {
				t.Errorf("%s missing catalog key %q", language, key)
			}
			if strings.Join(placeholder.FindAllString(key, -1), ",") != strings.Join(placeholder.FindAllString(catalog[key], -1), ",") {
				t.Errorf("%s changed placeholders in %q", language, key)
			}
		}
		for _, key := range keys {
			if catalog[key] == "" {
				t.Errorf("%s missing UI phrase %q", language, key)
			}
		}
	}
}

func TestLanguagePreferenceValidationAndSecurity(t *testing.T) {
	a := testApp(t)
	for _, language := range []string{"pt-BR", "en", "es", "fr"} {
		form := url.Values{"csrf": {a.token}, "language": {language}, "return_to": {"/history?q=ZZ0TEST&city=Example"}}
		w := languageRequest(a, "", "POST", "/language", form)
		cookies := w.Result().Cookies()
		if w.Code != 303 || w.Header().Get("Location") != form.Get("return_to") || len(cookies) != 1 || cookies[0].Value != language || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" || cookies[0].MaxAge <= 0 {
			t.Fatalf("%s preference: status %d, cookies %+v, headers %v", language, w.Code, cookies, w.Header())
		}
	}
	for _, destination := range []string{"https://example.com/", "//example.com/", "/\\example.com", "/%2fexample.com", "/static/app.js", "javascript:alert(1)", "", "/\r\nLocation:bad"} {
		w := languageRequest(a, "en", "POST", "/language", url.Values{"csrf": {a.token}, "language": {"en"}, "return_to": {destination}})
		if w.Code != 400 || len(w.Result().Cookies()) != 0 {
			t.Errorf("accepted destination %q: %d", destination, w.Code)
		}
	}
	for _, language := range []string{"de", "", "EN", "../en"} {
		w := languageRequest(a, "en", "POST", "/language", url.Values{"csrf": {a.token}, "language": {language}, "return_to": {"/"}})
		if w.Code != 400 || !strings.Contains(w.Body.String(), "Invalid language") || len(w.Result().Cookies()) != 0 {
			t.Errorf("accepted language %q: %d %s", language, w.Code, w.Body)
		}
	}
	if w := languageRequest(a, "en", "POST", "/language", url.Values{"language": {"en"}, "return_to": {"/"}}); w.Code != 403 || len(w.Result().Cookies()) != 0 {
		t.Fatal("language preference bypassed CSRF")
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/language", strings.NewReader(url.Values{"csrf": {a.token}, "language": {"en"}, "return_to": {"/"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 || len(w.Result().Cookies()) != 0 {
		t.Fatal("language preference bypassed Origin validation")
	}
	if w := languageRequest(a, "en", "GET", "/language", nil); w.Code != 405 {
		t.Fatal("language preference accepts GET")
	}
	for _, language := range []string{"", "de", "../fr"} {
		w := languageRequest(a, language, "GET", "/", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `<html lang="pt-BR"`) {
			t.Fatalf("invalid/missing cookie did not default to Portuguese: %d", w.Code)
		}
	}
}

func TestLocalizedPagesPreserveDataAndIndependentRequests(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	st := Station{Callsign: "ZZ0LANG", Name: "Renée <Example>", City: "São Paulo", State: "SP", Country: "Brasil", Notes: "Nota permanente"}
	id, err := a.store.saveStation(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	st.ID = id
	if err := a.store.saveParticipant(ctx, 1, 0, st, "DMR", "Nota da conversa"); err != nil {
		t.Fatal(err)
	}
	if err := a.store.recordParticipant(ctx, 1); err != nil {
		t.Fatal(err)
	}
	before, err := a.store.contact(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, language := range []string{"pt-BR", "en", "es", "fr"} {
		for _, path := range []string{"/", "/round", "/history", "/stations", "/station?id=1", "/contact?id=1", "/participant?id=1"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w := languageRequest(a, language, "GET", path, nil)
				body := w.Body.String()
				if w.Code != 200 || w.Header().Get("Content-Language") != language || !strings.Contains(body, `<html lang="`+language+`"`) || !strings.Contains(body, `value="`+language+`" selected`) || !strings.Contains(body, a.translate(language, "Ir para o conteúdo")) || strings.Contains(body, "ZgotmplZ") {
					t.Errorf("%s %s: %d %s", language, path, w.Code, body)
				}
				match := regexp.MustCompile(`data-messages="([^"]+)"`).FindStringSubmatch(body)
				var messages map[string]string
				if len(match) != 2 || json.Unmarshal([]byte(html.UnescapeString(match[1])), &messages) != nil || messages["Informe um indicativo."] != a.translate(language, "Informe um indicativo.") {
					t.Errorf("%s %s: missing/invalid browser messages", language, path)
				}
				if path == "/round" && (!strings.Contains(body, "Renée &lt;Example&gt;") || !strings.Contains(body, "Nota da conversa") || !strings.Contains(body, "Nota permanente") || !strings.Contains(body, "São Paulo")) {
					t.Errorf("%s changed or failed to escape operator details", language)
				}
			}()
		}
	}
	wg.Wait()
	for _, language := range []string{"pt-BR", "en", "es", "fr"} {
		w := languageRequest(a, language, "GET", "/lookup?callsign=zz0lang", nil)
		var data struct {
			Station
			Contacts int `json:"contacts"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &data) != nil || data.Station != st || data.Contacts != 1 {
			t.Errorf("%s changed local lookup: %d %s", language, w.Code, w.Body)
		}
		w = languageRequest(a, language, "POST", "/participant", url.Values{"csrf": {a.token}, "round_id": {"1"}, "callsign": {"invalid space"}, "mode": {"FM"}, "name": {"Draft name"}})
		if w.Code != 400 || !strings.Contains(w.Body.String(), a.translate(language, "informe um indicativo de até 32 caracteres: letras, números, /, ponto ou hífen")) || !strings.Contains(w.Body.String(), `value="Draft name"`) || !strings.Contains(w.Body.String(), `name="return_to" value="/round"`) {
			t.Errorf("%s validation lost translation/draft: %d %s", language, w.Code, w.Body)
		}
		w = languageRequest(a, language, "GET", "/station?id=999", nil)
		if w.Code != 404 || !strings.Contains(w.Body.String(), a.translate(language, "Registro não encontrado")) {
			t.Errorf("%s missing localized error: %d %s", language, w.Code, w.Body)
		}
	}
	after, err := a.store.contact(ctx, 1)
	if err != nil || before != after {
		t.Fatalf("language selection modified historical data: %+v %v", after, err)
	}
	got, err := a.store.station(ctx, id)
	if err != nil || got != st {
		t.Fatalf("language selection modified operator data: %+v %v", got, err)
	}
}
