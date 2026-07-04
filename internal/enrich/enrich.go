// Package enrich turns a bare word (plus optional context sentence) into the
// content of an Anki card: an LLM-generated definition and examples from Claude,
// plus IPA and a pronunciation audio URL from the Free Dictionary API.
package enrich

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// generator produces the LLM portion of a card. It is implemented by both the
// HTTP API client (claudeClient) and the local `claude` CLI client (cliClient).
type generator interface {
	generate(ctx context.Context, word, contextSentence string) (*llmResult, error)
}

// Card is the enriched, render-ready content for one vocabulary note. Audio is
// attached later by the audio package (this struct only carries the source URL
// the dictionary provided, which may be empty).
type Card struct {
	Word           string
	IPA            string
	DefinitionHTML string
	ExamplesHTML   string
	// DictAudioURL is the human-pronunciation mp3 from the dictionary, or "" if
	// none was found (the audio package then synthesizes a fallback).
	DictAudioURL string
}

// Service produces Cards. It is safe for concurrent use.
type Service struct {
	gen       generator
	lemmatize bool
}

// Options selects and configures the LLM backend.
type Options struct {
	// Provider is "cli" (shell out to the local `claude` command) or "api"
	// (call the Anthropic HTTP API). Empty defaults to "cli".
	Provider string
	// Model is the model name/alias. For the CLI, aliases like "haiku" work; for
	// the API, use a full model id. Empty lets the backend pick its default.
	Model string
	// MaxTokens caps generation (API only).
	MaxTokens int
	// APIKey is required when Provider is "api".
	APIKey string
	// CLIPath is the `claude` executable (CLI only); empty defaults to "claude".
	CLIPath string
	// Lemmatize, when true, stores the base dictionary form (lemma) the model
	// infers rather than the exact word that was sent in.
	Lemmatize bool
}

// New builds an enrichment service using the configured backend.
func New(opts Options) (*Service, error) {
	var gen generator
	switch opts.Provider {
	case "", "cli":
		gen = newCLIClient(opts.CLIPath, opts.Model)
	case "api":
		if opts.APIKey == "" {
			return nil, fmt.Errorf("claude provider %q requires ANTHROPIC_API_KEY", opts.Provider)
		}
		gen = newClaudeClient(opts.APIKey, opts.Model, opts.MaxTokens)
	default:
		return nil, fmt.Errorf("unknown claude provider %q (want \"cli\" or \"api\")", opts.Provider)
	}
	return &Service{gen: gen, lemmatize: opts.Lemmatize}, nil
}

// Enrich generates card content for word. contextSentence, if non-empty, steers
// the sense and the first example.
//
// The LLM runs first because it also resolves the base dictionary form (lemma):
// when lemmatization is enabled, the returned Card.Word may differ from the
// input (e.g. "running" -> "run"), and the IPA/audio lookup is done against that
// lemma. A dictionary miss is non-fatal.
func (s *Service) Enrich(ctx context.Context, word, contextSentence string) (*Card, error) {
	res, err := s.gen.generate(ctx, word, contextSentence)
	if err != nil {
		return nil, err
	}

	headword := word
	if s.lemmatize {
		if lemma := strings.ToLower(strings.TrimSpace(res.Headword)); lemma != "" {
			headword = lemma
		}
	}

	dict, _ := lookup(ctx, headword) // dictionary failures degrade gracefully

	return &Card{
		Word:           headword,
		IPA:            dict.IPA,
		DefinitionHTML: definitionHTML(res),
		ExamplesHTML:   examplesHTML(res, headword, contextSentence),
		DictAudioURL:   dict.AudioURL,
	}, nil
}

// definitionHTML renders the part of speech and definition.
func definitionHTML(r *llmResult) string {
	var b strings.Builder
	if r.PartOfSpeech != "" {
		b.WriteString("<i>")
		b.WriteString(html.EscapeString(r.PartOfSpeech))
		b.WriteString("</i> ")
	}
	b.WriteString(html.EscapeString(r.Definition))
	if len(r.Collocations) > 0 {
		b.WriteString(`<div class="collocations" style="margin-top:8px;color:#777;font-size:15px">`)
		escaped := make([]string, len(r.Collocations))
		for i, c := range r.Collocations {
			escaped[i] = html.EscapeString(c)
		}
		b.WriteString(strings.Join(escaped, " · "))
		b.WriteString("</div>")
	}
	return b.String()
}

// examplesHTML renders the examples as a bulleted list with the headword bolded.
func examplesHTML(r *llmResult, word, contextSentence string) string {
	if len(r.Examples) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<ul>")
	for _, ex := range r.Examples {
		b.WriteString("<li>")
		b.WriteString(boldWord(html.EscapeString(ex), word))
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	return b.String()
}

// boldWord wraps case-insensitive whole-word occurrences of word in <b> tags.
// It operates on already-escaped text.
func boldWord(text, word string) string {
	if word == "" {
		return text
	}
	lowerText := strings.ToLower(text)
	lowerWord := strings.ToLower(word)
	var b strings.Builder
	i := 0
	for {
		idx := strings.Index(lowerText[i:], lowerWord)
		if idx < 0 {
			b.WriteString(text[i:])
			break
		}
		start := i + idx
		end := start + len(word)
		if isWordBoundary(lowerText, start, end) {
			b.WriteString(text[i:start])
			b.WriteString("<b>")
			b.WriteString(text[start:end])
			b.WriteString("</b>")
			i = end
		} else {
			b.WriteString(text[i : start+1])
			i = start + 1
		}
	}
	return b.String()
}

func isWordBoundary(s string, start, end int) bool {
	leftOK := start == 0 || !isLetter(s[start-1])
	rightOK := end >= len(s) || !isLetter(s[end])
	return leftOK && rightOK
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
