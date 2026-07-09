package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const anthropicURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// claudeClient is a minimal wrapper over the Anthropic Messages API. We call it
// over plain net/http to keep dependencies small and insulate the project from
// SDK churn.
type claudeClient struct {
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

func newClaudeClient(apiKey, model string, maxTokens int) *claudeClient {
	return &claudeClient{
		apiKey:     apiKey,
		model:      model,
		maxTokens:  maxTokens,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// llmResult is the JSON contract we ask the model to return.
type llmResult struct {
	Headword     string   `json:"headword"`
	PartOfSpeech string   `json:"part_of_speech"`
	Definition   string   `json:"definition"`
	Cloze        string   `json:"cloze"`
	Examples     []string `json:"examples"`
	Collocations []string `json:"collocations"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

const systemPrompt = `You are a lexicographer building English vocabulary flashcards for an advanced learner.
Given a word (and optionally the sentence the learner saw it in), respond with ONLY a JSON object, no markdown fences, no commentary. Schema:
{
  "headword": "the base dictionary form (lemma) of the GIVEN WORD ITSELF, to store as the flashcard word. Convert an inflected form to its base: plural->singular noun, conjugated verb (past/participle/gerund/3rd-person)->infinitive, comparative/superlative->base adjective/adverb, and fix an obvious misspelling. IMPORTANT: derive it from the given word alone and do NOT merge in adjacent words from the context sentence; if the given word is a single word, the headword must be a single word too (unless that word is itself a fixed idiom/phrasal verb). The context is only for choosing the right sense. Lowercase unless it is a proper noun.",
  "part_of_speech": "the most relevant part of speech for the given context",
  "definition": "a clear, learner-friendly English definition of the headword in COBUILD style (a full sentence explaining meaning and typical use)",
  "cloze": "ONE natural sentence for sentence-mining, with the target word wrapped in Anki cloze syntax, e.g. 'The detective was determined to {{c1::unravel}} the mystery.' If a context sentence is provided, use THAT sentence (lightly cleaned) and wrap the exact word form that appears in it; otherwise write a natural sentence using the word. Wrap ONLY the single target word occurrence in {{c1::...}}, nothing else.",
  "examples": ["1-2 ADDITIONAL example sentences using the headword, different from the cloze sentence"],
  "collocations": ["2-4 common collocations or phrases"]
}
Use the sense that fits the provided context when there is one. Keep it concise.`

// buildUserPrompt renders the per-word instruction shared by both backends.
func buildUserPrompt(word, contextSentence string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Headword: %s\n", word)
	if contextSentence != "" {
		fmt.Fprintf(&b, "Context sentence: %s\n", contextSentence)
	}
	return b.String()
}

// generate calls Claude and returns the parsed result.
func (c *claudeClient) generate(ctx context.Context, word, contextSentence string) (*llmResult, error) {
	reqBody, err := json.Marshal(anthropicRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    systemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: buildUserPrompt(word, contextSentence)}},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	var ar anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("anthropic decode: %w", err)
	}
	if ar.Error != nil {
		return nil, fmt.Errorf("anthropic: %s: %s", ar.Error.Type, ar.Error.Message)
	}
	if len(ar.Content) == 0 {
		return nil, fmt.Errorf("anthropic: empty response")
	}

	text := strings.TrimSpace(ar.Content[0].Text)
	return parseLLMJSON(text)
}

// parseLLMJSON tolerates the model wrapping its JSON in markdown fences or
// prose by extracting the outermost {...} object.
func parseLLMJSON(text string) (*llmResult, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON object in model output: %q", text)
	}
	var r llmResult
	if err := json.Unmarshal([]byte(text[start:end+1]), &r); err != nil {
		return nil, fmt.Errorf("parse model JSON: %w", err)
	}
	return &r, nil
}
