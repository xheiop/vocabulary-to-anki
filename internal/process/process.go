// Package process wires enrichment, audio, and Anki together into the single
// operation of turning an incoming word into a card. It also owns the
// Anki-offline fallback and the retry of the pending queue.
package process

import (
	"context"
	"log"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/xheiop/vocab2anki/internal/anki"
	"github.com/xheiop/vocab2anki/internal/audio"
	"github.com/xheiop/vocab2anki/internal/enrich"
	"github.com/xheiop/vocab2anki/internal/pending"
)

// Request is one word to add, from any intake (HTTP or file queue).
type Request struct {
	Word    string
	Context string
	Source  string
}

// Processor turns Requests into Anki notes.
type Processor struct {
	anki    *anki.Client
	enrich  *enrich.Service
	audio   *audio.Service
	pending *pending.Store

	// schemaReady flips true once the deck and note type have been confirmed to
	// exist, so we ensure them at most once per run regardless of intake path.
	schemaReady atomic.Bool
}

// New assembles a Processor from its collaborators.
func New(a *anki.Client, e *enrich.Service, au *audio.Service, p *pending.Store) *Processor {
	return &Processor{anki: a, enrich: e, audio: au, pending: p}
}

// Handle processes one request end to end. Errors are logged rather than
// returned because there is no caller waiting on the result (intake is async).
func (p *Processor) Handle(ctx context.Context, req Request) {
	// Some share sources (notably Apple News) hand the Shortcut an article URL or
	// headline instead of the selected word. Recover the real word — Apple News
	// puts it in the URL's "highlight" param — and drop inputs that clearly are
	// not a word, so we never enrich a headline or URL.
	word, source, ok := resolveWord(req.Word, req.Source)
	if !ok {
		log.Printf("ignoring non-word input %q", strings.TrimSpace(req.Word))
		return
	}

	// If Anki is not running, stash the word and bail; the retry loop will pick
	// it up once Anki comes online. Enriching first would waste an API call the
	// word may not be added with.
	if !p.anki.Available(ctx) {
		if err := p.pending.Add(pending.Item{Word: word, Context: req.Context, Source: source}); err != nil {
			log.Printf("pending add %q: %v", word, err)
		} else {
			log.Printf("anki offline; queued %q for retry", word)
		}
		return
	}

	if err := p.addNow(ctx, word, req.Context, source); err != nil {
		log.Printf("add %q: %v", word, err)
	}
}

// resolveWord extracts the vocabulary word to add from a raw intake string and
// picks the best source link. It returns ok=false when no usable word is found.
//
//   - A URL carrying the selected word in a "highlight" query param (Apple News
//     share) -> that word, with the URL used as the source.
//   - A plain single word or short phrase -> used as-is.
//   - A bare URL or a headline/sentence -> rejected.
func resolveWord(raw, source string) (word, src string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", source, false
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if u, err := url.Parse(raw); err == nil {
			if h := strings.TrimSpace(u.Query().Get("highlight")); h != "" {
				return normalize(h), raw, true
			}
		}
		return "", source, false // a URL with no highlighted word
	}
	if plausibleWord(raw) {
		return normalize(raw), source, true
	}
	return "", source, false
}

// plausibleWord accepts a single word or short phrase while rejecting sentences
// and headlines.
func plausibleWord(s string) bool {
	if len(s) > 40 || strings.ContainsAny(s, "?!") {
		return false
	}
	return len(strings.Fields(s)) <= 4
}

// ensureSchema creates the deck and note type if they do not yet exist,
// remembering success so the calls are made at most once. Anki must be
// reachable when this is called.
func (p *Processor) ensureSchema(ctx context.Context) error {
	if p.schemaReady.Load() {
		return nil
	}
	if err := p.anki.EnsureDeck(ctx); err != nil {
		return err
	}
	if err := p.anki.EnsureModel(ctx); err != nil {
		return err
	}
	p.schemaReady.Store(true)
	return nil
}

// addNow assumes Anki is reachable. It ensures the schema, enriches (which also
// resolves the base form / lemma), deduplicates on that resolved word, attaches
// audio, and adds the note.
func (p *Processor) addNow(ctx context.Context, inputWord, contextSentence, source string) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	// Enrich first: it may rewrite the word to its dictionary base form (e.g.
	// "running" -> "run"), so dedup and audio must run against the result.
	card, err := p.enrich.Enrich(ctx, inputWord, contextSentence)
	if err != nil {
		return err
	}
	word := card.Word

	exists, err := p.anki.Exists(ctx, word)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("skip %q (from %q): already in deck", word, inputWord)
		return nil
	}

	audioField := ""
	if res, err := p.audio.Get(ctx, word, card.DictAudioURL); err != nil {
		log.Printf("audio %q: %v (continuing without audio)", word, err)
	} else if res.Filename != "" {
		stored, err := p.anki.StoreMedia(ctx, res.Filename, res.Base64)
		if err != nil {
			log.Printf("store media %q: %v (continuing without audio)", word, err)
		} else {
			audioField = "[sound:" + stored + "]"
			log.Printf("audio %q: %s", word, res.Source)
		}
	}

	fields := map[string]string{
		"Text":       card.ClozeText, // the cloze sentence (front/back)
		"Word":       word,
		"IPA":        card.IPA,
		"Audio":      audioField,
		"Definition": card.DefinitionHTML,
		"Examples":   card.ExamplesHTML,
		"Source":     sourceHTML(source),
		"AddedAt":    time.Now().Format("2006-01-02 15:04"),
	}

	id, err := p.anki.AddNote(ctx, fields, []string{"vocab2anki"})
	if err != nil {
		return err
	}
	if word != inputWord {
		log.Printf("added %q (lemma of %q) (note %d)", word, inputWord, id)
	} else {
		log.Printf("added %q (note %d)", word, id)
	}
	return nil
}

// RetryPending attempts to flush the pending queue. It is a no-op while Anki is
// offline. Intended to be called on a ticker.
func (p *Processor) RetryPending(ctx context.Context) {
	items, err := p.pending.List()
	if err != nil {
		log.Printf("pending list: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}
	if !p.anki.Available(ctx) {
		return
	}
	log.Printf("anki online; retrying %d pending word(s)", len(items))
	for _, it := range items {
		if err := p.addNow(ctx, it.Word, it.Context, it.Source); err != nil {
			log.Printf("retry %q: %v", it.Word, err)
			continue // leave in queue for the next tick
		}
		if err := p.pending.Remove(it); err != nil {
			log.Printf("pending remove %q: %v", it.Word, err)
		}
	}
}

func normalize(word string) string {
	return strings.ToLower(strings.TrimSpace(word))
}

// sourceHTML renders a source URL as a link, or plain text otherwise.
func sourceHTML(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return `<a href="` + enrich.EscapeAttr(source) + `">` + enrich.EscapeText(source) + `</a>`
	}
	return enrich.EscapeText(source)
}
