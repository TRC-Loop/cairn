// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"embed"
	"fmt"
	"html/template"
	"math"
	"time"
)

//go:embed templates/*.html templates/partials/*.html templates/embed/*.html
var templateFS embed.FS

// loadTemplates parses every base and partial template into a single set so
// {{ template "..." }} lookups resolve across files. Returns a parse error
// rather than panicking so callers can decide how to fail.
func loadTemplates() (*template.Template, error) {
	return template.New("").Funcs(templateFuncs()).ParseFS(templateFS,
		"templates/*.html",
		"templates/partials/*.html",
		"templates/embed/*.html",
	)
}

// mustLoadTemplates is called from NewHandler; a parse failure here means
// broken builds reach production, so we surface the error loudly.
func mustLoadTemplates() *template.Template {
	t, err := loadTemplates()
	if err != nil {
		panic(fmt.Sprintf("statuspage: template parse failed: %v", err))
	}
	return t
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatTime":      formatTime,
		"formatTimeLocal": formatTimeLocal,
		"relativeTime":    relativeTime,
		"statusColor":     statusColor,
		"statusLabel":     statusLabel,
		"overallLabel":    overallLabel,
		"safeHTML":        safeHTML,
		"safeFooter":      safeFooter,
		"dict":            dict,
		"slice":           sliceFn,
		"percent":         percent,
		"titlecase":       titleCase,
		"sub":             func(a, b int) int { return a - b },
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// formatTimeLocal is a stub for Phase 2c. Localization-aware rendering lands
// when i18n ships; for now we emit UTC so cached pages remain correct
// regardless of viewer TZ.
func formatTimeLocal(t time.Time, _ string) string {
	return formatTime(t)
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t.UTC())
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return pluralize(int(d/time.Minute), "minute", "minutes") + " ago"
	case d < 24*time.Hour:
		return pluralize(int(d/time.Hour), "hour", "hours") + " ago"
	case d < 30*24*time.Hour:
		return pluralize(int(d/(24*time.Hour)), "day", "days") + " ago"
	case d < 365*24*time.Hour:
		return pluralize(int(d/(30*24*time.Hour)), "month", "months") + " ago"
	default:
		return pluralize(int(d/(365*24*time.Hour)), "year", "years") + " ago"
	}
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func statusColor(s string) string {
	switch s {
	case "up", "operational":
		return "s-up"
	case "degraded":
		return "s-degraded"
	case "down", "major_outage", "partial_outage":
		return "s-down"
	case "maintenance":
		return "s-maintenance"
	case "nodata":
		return "s-nodata"
	default:
		return "s-unknown"
	}
}

func statusLabel(s string) string {
	switch s {
	case "up":
		return "Operational"
	case "degraded":
		return "Degraded"
	case "down":
		return "Down"
	case "maintenance":
		return "Maintenance"
	case "nodata":
		return "No data"
	default:
		return "Unknown"
	}
}

func overallLabel(s string) string {
	switch s {
	case "operational":
		return "All systems operational"
	case "degraded":
		return "Degraded performance"
	case "partial_outage":
		return "Partial outage"
	case "major_outage":
		return "Major outage"
	case "maintenance":
		return "Scheduled maintenance"
	default:
		return "Status unknown"
	}
}

func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

func safeFooter(s string) template.HTML {
	return SanitizeFooter(s)
}

func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict requires even arg count")
	}
	out := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key %v is not a string", pairs[i])
		}
		out[key] = pairs[i+1]
	}
	return out, nil
}

func sliceFn(items ...any) []any {
	return items
}

func percent(num, denom int) string {
	if denom == 0 {
		return "N/A"
	}
	p := float64(num) / float64(denom) * 100
	return fmt.Sprintf("%.2f%%", math.Floor(p*100)/100)
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 32
	}
	return string(r)
}
