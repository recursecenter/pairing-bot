package main

import (
	"embed"
	"strings"
	"text/template"
	"time"
)

//go:embed templates
var templatesFS embed.FS
var templates = template.Must(template.ParseFS(templatesFS, "templates/*.tmpl"))

// renderTemplate executes the template and returns the resulting string.
func renderTemplate(path string, data any) (string, error) {
	var sb strings.Builder
	if err := templates.ExecuteTemplate(&sb, path, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func renderWelcome(now time.Time) (string, error) {
	return renderTemplate("welcome.md.tmpl", map[string]any{
		"Now": now,
	})
}

func renderCheckin(now time.Time, numPairings int, numRecursers int, review string) (string, error) {
	return renderTemplate("checkin.md.tmpl", map[string]any{
		"Now":       now,
		"Recursers": numRecursers,
		"Pairings":  numPairings,
		"Review":    review,
	})
}
