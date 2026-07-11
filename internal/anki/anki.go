// Package anki is a small client for the AnkiConnect add-on (default endpoint
// http://127.0.0.1:8765). It knows how to ensure the vocab2anki deck and note
// type exist, deduplicate by word, store media, and add notes.
package anki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"
)

// AnkiConnect protocol version this client speaks.
const apiVersion = 6

// FieldOrder is the field layout of the vocab2anki Cloze note type. Text (the
// cloze sentence) is first, as is conventional for cloze models; the rest are
// shown on the back.
var FieldOrder = []string{"Text", "Word", "IPA", "Audio", "Definition", "Examples", "Source", "AddedAt"}

// Client talks to a running Anki desktop via AnkiConnect.
type Client struct {
	url        string
	deck       string
	model      string
	httpClient *http.Client
}

// New returns a client targeting the given AnkiConnect URL, deck and note type.
func New(url, deck, model string) *Client {
	return &Client{
		url:   url,
		deck:  deck,
		model: model,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			// AnkiConnect's simple HTTP server closes connections after each
			// response, so reusing a kept-alive connection yields "connection
			// reset by peer". Force a fresh connection per request.
			Transport: &http.Transport{DisableKeepAlives: true},
		},
	}
}

type request struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
	Params  any    `json:"params,omitempty"`
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

// invoke performs a single AnkiConnect call and unmarshals result into out
// (which may be nil to discard the result).
func (c *Client) invoke(ctx context.Context, action string, params any, out any) error {
	body, err := json.Marshal(request{Action: action, Version: apiVersion, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("anki %s: %w", action, err)
	}
	defer resp.Body.Close()

	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("anki %s decode: %w", action, err)
	}
	if r.Error != nil {
		return fmt.Errorf("anki %s: %s", action, *r.Error)
	}
	if out != nil && len(r.Result) > 0 {
		if err := json.Unmarshal(r.Result, out); err != nil {
			return fmt.Errorf("anki %s result: %w", action, err)
		}
	}
	return nil
}

// Available reports whether AnkiConnect is reachable (i.e. Anki desktop is
// running with the add-on loaded).
func (c *Client) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var v int
	return c.invoke(ctx, "version", nil, &v) == nil
}

// EnsureDeck creates the configured deck if it does not already exist.
func (c *Client) EnsureDeck(ctx context.Context) error {
	return c.invoke(ctx, "createDeck", map[string]any{"deck": c.deck}, nil)
}

// EnsureModel creates the vocab2anki note type if it is missing. It is a no-op
// (and swallows the "model already exists" error) once created.
func (c *Client) EnsureModel(ctx context.Context) error {
	var names []string
	if err := c.invoke(ctx, "modelNames", nil, &names); err != nil {
		return err
	}
	if slices.Contains(names, c.model) {
		return nil
	}
	params := map[string]any{
		"modelName":     c.model,
		"inOrderFields": FieldOrder,
		"css":           cardCSS,
		"isCloze":       true,
		"cardTemplates": []map[string]string{
			{"Name": "Cloze", "Front": frontTemplate, "Back": backTemplate},
		},
	}
	return c.invoke(ctx, "createModel", params, nil)
}

// Exists reports whether a note with the given Word already exists in the deck.
func (c *Client) Exists(ctx context.Context, word string) (bool, error) {
	query := fmt.Sprintf(`deck:"%s" Word:"%s"`, c.deck, escapeQuery(word))
	var ids []int64
	if err := c.invoke(ctx, "findNotes", map[string]any{"query": query}, &ids); err != nil {
		return false, err
	}
	return len(ids) > 0, nil
}

// StoreMedia writes base64-encoded data into Anki's media collection under
// filename and returns the stored filename.
func (c *Client) StoreMedia(ctx context.Context, filename, base64Data string) (string, error) {
	var stored string
	err := c.invoke(ctx, "storeMediaFile", map[string]any{
		"filename": filename,
		"data":     base64Data,
	}, &stored)
	if err != nil {
		return "", err
	}
	if stored == "" {
		stored = filename
	}
	return stored, nil
}

// AddNote adds a note to the configured deck/model with the given field values
// and tags, returning the new note id.
func (c *Client) AddNote(ctx context.Context, fields map[string]string, tags []string) (int64, error) {
	if tags == nil {
		tags = []string{}
	}
	params := map[string]any{
		"note": map[string]any{
			"deckName":  c.deck,
			"modelName": c.model,
			"fields":    fields,
			"tags":      tags,
			"options": map[string]any{
				"allowDuplicate": false,
			},
		},
	}
	var id int64
	if err := c.invoke(ctx, "addNote", params, &id); err != nil {
		return 0, err
	}
	return id, nil
}

// escapeQuery escapes characters that are special inside an Anki search query.
func escapeQuery(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '*', '_', '\\':
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}
