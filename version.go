package main

import (
	"net/http"
	"runtime/debug"
)

// Set by release builds. Source builds remain clearly identified as development.
var appVersion = "dev"
var appRevision = ""

func sourceRevision() string {
	if appRevision != "" {
		return appRevision
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "unknown"
}
func (a *App) about(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, Page{Title: "Sobre o QSObook", View: "about", Version: appVersion, Revision: sourceRevision(), Schema: schemaVersion})
}
