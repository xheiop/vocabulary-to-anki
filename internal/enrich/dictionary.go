package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// dictBase is the Free Dictionary API. No API key required.
const dictBase = "https://api.dictionaryapi.dev/api/v2/entries/en/"

// dictResult holds the phonetics extracted from the dictionary.
type dictResult struct {
	IPA      string // e.g. "/ˌsɛr.ənˈdɪp.ɪ.ti/"
	AudioURL string // real human pronunciation mp3, may be empty
}

type dictEntry struct {
	Phonetic  string `json:"phonetic"`
	Phonetics []struct {
		Text  string `json:"text"`
		Audio string `json:"audio"`
	} `json:"phonetics"`
}

// lookup fetches IPA and a pronunciation audio URL. A miss (word not in the
// dictionary) returns an empty result and no error; callers fall back to TTS.
func lookup(ctx context.Context, word string) (dictResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	u := dictBase + url.PathEscape(word)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return dictResult{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return dictResult{}, fmt.Errorf("dictionary request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return dictResult{}, nil // not a hard error; TTS will cover it
	}
	if resp.StatusCode != http.StatusOK {
		return dictResult{}, fmt.Errorf("dictionary status %d", resp.StatusCode)
	}

	var entries []dictEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return dictResult{}, fmt.Errorf("dictionary decode: %w", err)
	}

	var res dictResult
	for _, e := range entries {
		if res.IPA == "" && e.Phonetic != "" {
			res.IPA = e.Phonetic
		}
		for _, p := range e.Phonetics {
			if res.IPA == "" && p.Text != "" {
				res.IPA = p.Text
			}
			if res.AudioURL == "" && p.Audio != "" {
				res.AudioURL = normalizeAudioURL(p.Audio)
			}
		}
	}
	return res, nil
}

// normalizeAudioURL upgrades protocol-relative "//..." URLs the API sometimes
// returns to https.
func normalizeAudioURL(u string) string {
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return u
}
