package main

import (
	"sort"
)

type SelectOption struct {
	Value, Label string
	Selected     bool
}

var modeChoices = []string{"FM", "AM", "SSB", "CW", "Digital", "DMR", "EchoLink", "D-Star", "C4FM"}
var bandChoices = []string{"70 cm", "2 m", "6 m", "10 m", "12 m", "15 m", "17 m", "20 m", "30 m", "40 m", "60 m", "80 m", "160 m", "23 cm"}

func choices(values []string, current string) []SelectOption {
	result := []SelectOption{}
	found := current == ""
	for _, value := range values {
		result = append(result, SelectOption{value, value, value == current})
		found = found || value == current
	}
	if !found {
		result = append(result, SelectOption{current, current, true})
	}
	return result
}
func (a *App) countryChoices(language, current string) []SelectOption {
	countries := a.countries
	options := []SelectOption{}
	found := current == ""
	for code, value := range countries["pt-BR"] {
		label := countries[language][code]
		if label == "" {
			label = value
		}
		options = append(options, SelectOption{value, label, value == current})
		found = found || value == current
	}
	if !found {
		options = append(options, SelectOption{current, current, true})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Label < options[j].Label })
	return options
}

func mapBandChoices(cities []ContactCity) []SelectOption {
	values := append([]string(nil), bandChoices...)
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, city := range cities {
		for _, group := range city.Groups {
			if group.Band != "" && !seen[group.Band] {
				seen[group.Band] = true
				values = append(values, group.Band)
			}
		}
	}
	return choices(values, "")
}
