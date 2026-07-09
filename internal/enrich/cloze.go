package enrich

import (
	"html"
	"regexp"
	"strings"
)

// clozeRE matches a well-formed Anki cloze deletion like {{c1::word}}.
var clozeRE = regexp.MustCompile(`\{\{c\d+::[^}]+\}\}`)

// clozeText produces the cloze sentence for the Text field: HTML-escaped, with
// the target word wrapped in {{c1::...}}. It trusts the model's cloze sentence
// when well-formed, and otherwise builds one deterministically from the context
// sentence or an example. HTML escaping leaves the cloze braces intact because
// "{", "}" and ":" are not HTML-special.
func clozeText(r *llmResult, inputWord, headword, contextSentence string) string {
	if raw := strings.TrimSpace(r.Cloze); clozeRE.MatchString(raw) {
		return html.EscapeString(raw)
	}

	base := strings.TrimSpace(contextSentence)
	if base == "" && len(r.Examples) > 0 {
		base = strings.TrimSpace(r.Examples[0])
	}
	if base == "" {
		base = headword
	}
	// Prefer wrapping the exact form seen in the context sentence, then the lemma.
	return html.EscapeString(clozeWrap(base, inputWord, headword))
}

// clozeWrap wraps the first whole-word, case-insensitive occurrence of any
// target in sentence with {{c1::...}}. If none match, it clozes the last target
// on its own so the note still has a valid deletion.
func clozeWrap(sentence string, targets ...string) string {
	for _, t := range targets {
		if wrapped, ok := wrapFirstWord(sentence, strings.TrimSpace(t)); ok {
			return wrapped
		}
	}
	fallback := sentence
	for _, t := range targets {
		if s := strings.TrimSpace(t); s != "" {
			fallback = s
		}
	}
	return "{{c1::" + fallback + "}}"
}

// wrapFirstWord wraps the first whole-word occurrence of word (case-insensitive)
// in text with {{c1::...}}, preserving the original casing of the match.
func wrapFirstWord(text, word string) (string, bool) {
	if word == "" {
		return "", false
	}
	lowerText := strings.ToLower(text)
	lowerWord := strings.ToLower(word)
	from := 0
	for {
		i := strings.Index(lowerText[from:], lowerWord)
		if i < 0 {
			return "", false
		}
		start := from + i
		end := start + len(word)
		if isWordBoundary(lowerText, start, end) {
			return text[:start] + "{{c1::" + text[start:end] + "}}" + text[end:], true
		}
		from = start + 1
	}
}
