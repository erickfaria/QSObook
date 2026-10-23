package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func supportedLanguage(language string) bool {
	switch language {
	case "pt-BR", "en", "es", "fr":
		return true
	}
	return false
}

func requestLanguage(r *http.Request) string {
	if cookie, err := r.Cookie("qsobook-language"); err == nil && supportedLanguage(cookie.Value) {
		return cookie.Value
	}
	return "pt-BR"
}

func readTranslations() (map[string]map[string]string, error) {
	translations := map[string]map[string]string{"pt-BR": {}}
	for _, language := range []string{"en", "es", "fr"} {
		data, err := assets.ReadFile("locales/" + language + ".json")
		if err != nil {
			return nil, err
		}
		var catalog map[string]string
		if err := json.Unmarshal(data, &catalog); err != nil {
			return nil, fmt.Errorf("load %s translations: %w", language, err)
		}
		translations[language] = catalog
	}
	return translations, nil
}

// Portuguese source phrases are the translation keys. Catalogs are loaded once
// per app and only read afterwards, including during concurrent requests.
func (a *App) translate(language, message string) string {
	if translated := a.translations[language][message]; translated != "" {
		return translated
	}
	return message
}

func (a *App) localizedTemplates(loc *time.Location) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template, len(a.translations))
	for language := range a.translations {
		t, err := template.New("layout.html").Funcs(template.FuncMap{
			"t": func(message string) string { return a.translate(language, message) },
			"localTime": func(t time.Time) string {
				format := "02/01/2006 15:04:05 MST"
				if language == "en" {
					format = "2006-01-02 15:04:05 MST"
				}
				return t.In(loc).Format(format)
			},
		}).ParseFS(assets, "templates/*.html")
		if err != nil {
			return nil, err
		}
		templates[language] = t
	}
	return templates, nil
}

func (a *App) httpError(w http.ResponseWriter, r *http.Request, message string, status int) {
	language := requestLanguage(r)
	w.Header().Set("Content-Language", language)
	http.Error(w, a.translate(language, message), status)
}

func (a *App) languagePreference(w http.ResponseWriter, r *http.Request) {
	language := r.PostForm.Get("language")
	if !supportedLanguage(language) {
		a.httpError(w, r, "Idioma inválido", http.StatusBadRequest)
		return
	}
	// Return only to an existing local page, preserving its search/round ID.
	// Never use the Referer header or accept an arbitrary redirect destination.
	destination, err := url.Parse(r.PostForm.Get("return_to"))
	if err != nil || destination.IsAbs() || destination.Host != "" || destination.Opaque != "" || strings.ContainsAny(destination.String(), "\\\r\n") {
		a.httpError(w, r, "Destino inválido", http.StatusBadRequest)
		return
	}
	switch destination.Path {
	case "/", "/round", "/history", "/stations", "/station", "/contact", "/participant":
	default:
		a.httpError(w, r, "Destino inválido", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "qsobook-language", Value: language, Path: "/", MaxAge: 365 * 24 * 60 * 60, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	redirect(w, r, destination.String())
}

func browserMessages() []string {
	return []string{
		"Aproximar", "Afastar", "Fechar",
		"Informe um indicativo.",
		"Consultando cadastro local…",
		"Indicativo novo no cadastro local.",
		"Operador conhecido · {count} QSO(s) anteriores. Para corrigir dados salvos, use Editar.",
		"Não foi possível consultar. Você pode preencher os dados manualmente.",
		"Não foi possível abrir o mapa. Seus contatos estão na lista de cidades.",
		"O fundo do mapa está indisponível. Você ainda pode consultar os pontos e a lista de cidades.",
		"{count} contato(s) · Abrir caderno",
	}
}
