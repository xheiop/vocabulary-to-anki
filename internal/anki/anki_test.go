package anki

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockAnki spins up an httptest server that answers AnkiConnect actions from a
// map of action -> result, recording the last request for each action.
func mockAnki(t *testing.T, results map[string]any) (*Client, *map[string]json.RawMessage) {
	t.Helper()
	captured := map[string]json.RawMessage{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Action string          `json:"action"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		captured[req.Action] = req.Params

		res, ok := results[req.Action]
		if !ok {
			res = nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": res, "error": nil})
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "Vocabulary::English", "Vocab2Anki Cloze"), &captured
}

func TestExists(t *testing.T) {
	c, _ := mockAnki(t, map[string]any{"findNotes": []int64{101}})
	ok, err := c.Exists(context.Background(), "serendipity")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected word to exist")
	}

	c2, _ := mockAnki(t, map[string]any{"findNotes": []int64{}})
	ok, _ = c2.Exists(context.Background(), "nonesuch")
	if ok {
		t.Fatal("expected word to be absent")
	}
}

func TestAddNoteSendsFields(t *testing.T) {
	c, captured := mockAnki(t, map[string]any{"addNote": 555})
	fields := map[string]string{"Word": "serendipity", "Definition": "luck"}
	id, err := c.AddNote(context.Background(), fields, []string{"vocab2anki"})
	if err != nil {
		t.Fatal(err)
	}
	if id != 555 {
		t.Fatalf("note id = %d, want 555", id)
	}

	var params struct {
		Note struct {
			DeckName  string            `json:"deckName"`
			ModelName string            `json:"modelName"`
			Fields    map[string]string `json:"fields"`
			Tags      []string          `json:"tags"`
		} `json:"note"`
	}
	if err := json.Unmarshal((*captured)["addNote"], &params); err != nil {
		t.Fatal(err)
	}
	if params.Note.DeckName != "Vocabulary::English" {
		t.Errorf("deck = %q", params.Note.DeckName)
	}
	if params.Note.Fields["Word"] != "serendipity" {
		t.Errorf("Word field = %q", params.Note.Fields["Word"])
	}
	if len(params.Note.Tags) != 1 || params.Note.Tags[0] != "vocab2anki" {
		t.Errorf("tags = %v", params.Note.Tags)
	}
}

func TestEnsureModelSkipsWhenPresent(t *testing.T) {
	c, captured := mockAnki(t, map[string]any{"modelNames": []string{"Basic", "Vocab2Anki Cloze"}})
	if err := c.EnsureModel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, created := (*captured)["createModel"]; created {
		t.Fatal("createModel should not be called when model already exists")
	}
}

func TestEnsureModelCreatesWhenMissing(t *testing.T) {
	c, captured := mockAnki(t, map[string]any{"modelNames": []string{"Basic"}})
	if err := c.EnsureModel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, created := (*captured)["createModel"]; !created {
		t.Fatal("createModel should be called when model is missing")
	}
}

func TestEscapeQuery(t *testing.T) {
	if got := escapeQuery(`a"b*c_d\e`); got != `a\"b\*c\_d\\e` {
		t.Fatalf("escapeQuery = %q", got)
	}
}
