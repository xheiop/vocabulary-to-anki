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
//
// hint, when non-empty, is added as an Anki cloze hint ({{c1::word::hint}}) so
// the card front shows it inside the blank; an empty hint leaves a bare blank.
func clozeText(r *llmResult, inputWord, headword, contextSentence, hint string) string {
	raw := strings.TrimSpace(r.Cloze)
	if !clozeRE.MatchString(raw) {
		base := strings.TrimSpace(contextSentence)
		if base == "" && len(r.Examples) > 0 {
			base = strings.TrimSpace(r.Examples[0])
		}
		if base == "" {
			base = headword
		}
		// Prefer wrapping the exact form seen in the context, then the lemma.
		raw = clozeWrap(base, inputWord, headword)
	}
	return html.EscapeString(injectHint(raw, hint))
}

// clozeDeletionRE captures the number and body of a cloze deletion.
var clozeDeletionRE = regexp.MustCompile(`\{\{c(\d+)::([^}]*)\}\}`)

// injectHint appends "::hint" to the first cloze deletion that does not already
// carry a hint, turning {{c1::word}} into {{c1::word::hint}}. Anki then renders
// the hint inside the blank on the card front.
func injectHint(cloze, hint string) string {
	hint = sanitizeHint(hint)
	if hint == "" {
		return cloze
	}
	patched := false
	return clozeDeletionRE.ReplaceAllStringFunc(cloze, func(m string) string {
		if patched {
			return m
		}
		sub := clozeDeletionRE.FindStringSubmatch(m)
		if sub == nil {
			return m
		}
		patched = true
		if strings.Contains(sub[2], "::") {
			return m // the model already supplied a hint
		}
		return "{{c" + sub[1] + "::" + sub[2] + "::" + hint + "}}"
	})
}

// sanitizeHint strips characters that would break cloze parsing and caps the
// length so the blank stays compact.
func sanitizeHint(hint string) string {
	hint = strings.NewReplacer("{", "", "}", "", "::", "、").Replace(strings.TrimSpace(hint))
	if r := []rune(hint); len(r) > 20 {
		hint = string(r[:20])
	}
	return strings.TrimSpace(hint)
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
