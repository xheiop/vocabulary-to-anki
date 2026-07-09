package enrich

import "html"

// Exported rendering helpers shared with the process package so that all HTML
// escaping for card fields lives in one place.

// EscapeText escapes text for use in element content.
func EscapeText(s string) string {
	return html.EscapeString(s)
}

// EscapeAttr escapes s for use inside a double-quoted HTML attribute.
// html.EscapeString escapes quotes, so it is sufficient here.
func EscapeAttr(s string) string {
	return html.EscapeString(s)
}
