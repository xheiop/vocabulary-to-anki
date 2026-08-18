// Package audio obtains pronunciation audio for a word, trying three sources in
// order of quality: the dictionary API's human recording, then Youdao's
// pronunciation service, then local synthesis with macOS's built-in `say`. It
// returns a filename and base64 payload ready for AnkiConnect's storeMediaFile.
package audio

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Result is a piece of audio ready to hand to Anki.
type Result struct {
	Filename string // e.g. "vocab2anki-serendipity-ab12cd.mp3"
	Base64   string // file contents, base64-encoded
	Source   string // which tier produced it: "dictionary", "youdao" or "say"
}

// Service produces audio. cacheDir is where synthesized/downloaded files are
// written before being read back and encoded.
type Service struct {
	cacheDir   string
	httpClient *http.Client
}

// New returns an audio service writing temporary files under cacheDir.
func New(cacheDir string) (*Service, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create audio cache dir: %w", err)
	}
	return &Service{
		cacheDir:   cacheDir,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

// Get returns audio for word, preferring a real human recording and falling
// back through progressively more synthetic sources:
//
//  1. dictAudioURL - the Free Dictionary API's human recording, when it had one;
//  2. Youdao's pronunciation service, which covers almost every headword;
//  3. macOS `say`, which always works offline.
//
// Each tier is skipped silently when it fails, so a card still gets audio.
func (s *Service) Get(ctx context.Context, word, dictAudioURL string) (Result, error) {
	slug := slugify(word)

	if dictAudioURL != "" {
		if r, err := s.download(ctx, slug, dictAudioURL, "dictionary"); err == nil {
			return r, nil
		}
	}
	if r, err := s.download(ctx, slug, YoudaoAudioURL(word), "youdao"); err == nil {
		return r, nil
	}
	return s.synthesize(ctx, word, slug)
}

// youdaoVoiceType selects Youdao's accent: 2 is US, 1 is UK.
const youdaoVoiceType = 2

// YoudaoAudioURL builds the Youdao dictionary pronunciation URL for a word.
func YoudaoAudioURL(word string) string {
	return fmt.Sprintf("https://dict.youdao.com/dictvoice?audio=%s&type=%d",
		url.QueryEscape(word), youdaoVoiceType)
}

// minAudioBytes rejects error pages and empty bodies served with a 200 status;
// even a very short real recording is comfortably larger than this.
const minAudioBytes = 256

// download fetches audio from rawURL into the cache and encodes it. source
// labels which tier the URL came from.
func (s *Service) download(ctx context.Context, slug, rawURL, source string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("audio download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, err
	}
	// Some services answer 200 with an HTML error page rather than audio.
	if len(data) < minAudioBytes || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
		return Result{}, fmt.Errorf("%s returned %d bytes, not audio", source, len(data))
	}

	filename := fmt.Sprintf("vocab2anki-%s-%s%s", slug, shortHash(rawURL), audioExt(rawURL))
	path := filepath.Join(s.cacheDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Result{}, err
	}
	return Result{
		Filename: filename,
		Base64:   base64.StdEncoding.EncodeToString(data),
		Source:   source,
	}, nil
}

// audioExt picks the file extension from a URL's path, ignoring any query
// string so that "…/a.mp3?token=x" does not become part of the extension.
func audioExt(rawURL string) string {
	path := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Path != "" {
		path = u.Path
	}
	if ext := filepath.Ext(path); ext != "" {
		return ext
	}
	return ".mp3"
}

// synthesize generates audio with the macOS `say` command (AAC in an .m4a
// container, which Anki plays via mpv).
func (s *Service) synthesize(ctx context.Context, word, slug string) (Result, error) {
	filename := fmt.Sprintf("vocab2anki-%s-tts.m4a", slug)
	path := filepath.Join(s.cacheDir, filename)

	cmd := exec.CommandContext(ctx, "say", "-o", path, "--data-format=aac", word)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("say synthesis: %w: %s", err, strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Filename: filename,
		Base64:   base64.StdEncoding.EncodeToString(data),
		Source:   "say",
	}, nil
}

// slugify makes a filesystem/Anki-safe token from a word.
func slugify(word string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(word) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' {
			b.WriteRune('-')
		}
	}
	s := b.String()
	if s == "" {
		s = "word"
	}
	return s
}

func shortHash(s string) string {
	h := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", h)[:6]
}
