package openai

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
	"time"
)

// templateFuncMap returns the template functions available in system prompts.
//
// Available functions:
//
//	{{ now }}                      — current time as time.Time
//	{{ now | date "2006-01-02" }}  — formatted date (Go reference time layout)
//	{{ env "VAR" }}                — environment variable, "" if unset
var templateFuncMap = template.FuncMap{
	// now returns the current local time.
	"now": time.Now,

	// date formats a time.Time value using a Go reference-time layout string.
	// Usage: {{ now | date "Mon, 02 Jan 2006 15:04 MST" }}
	"date": func(layout string, t time.Time) string {
		return t.Format(layout)
	},

	// env returns the value of an environment variable, or "" if not set.
	"env": os.Getenv,
}

// renderSystemPrompt executes the system prompt string as a Go text/template.
// If the string contains no template directives it is returned unchanged.
// An error is returned only when the template is syntactically invalid.
func renderSystemPrompt(system string) (string, error) {
	tmpl, err := template.New("system").Funcs(templateFuncMap).Parse(system)
	if err != nil {
		return "", fmt.Errorf("system prompt template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", fmt.Errorf("system prompt template render error: %w", err)
	}

	return buf.String(), nil
}
