// Package audio obtains pronunciation audio for a word: it downloads the
// dictionary's human-recorded mp3 when one exists, and otherwise synthesizes a
// fallback with macOS's built-in `say` command. Either way it returns a
// filename and base64 payload ready for AnkiConnect's storeMediaFile.
package audio

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
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

// Get returns audio for word. If dictAudioURL is non-empty it is downloaded;
// otherwise the word is synthesized with `say`. The returned Result is empty
// (zero value) with no error only if both paths are unavailable.
func (s *Service) Get(ctx context.Context, word, dictAudioURL string) (Result, error) {
	slug := slugify(word)

	if dictAudioURL != "" {
		if r, err := s.download(ctx, slug, dictAudioURL); err == nil {
			return r, nil
		}
		// fall through to TTS on download failure
	}
	return s.synthesize(ctx, word, slug)
}

// download fetches the dictionary audio into the cache and encodes it.
func (s *Service) download(ctx context.Context, slug, url string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".mp3"
	}
	filename := fmt.Sprintf("vocab2anki-%s-%s%s", slug, shortHash(url), ext)
	path := filepath.Join(s.cacheDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Result{}, err
	}
	return Result{Filename: filename, Base64: base64.StdEncoding.EncodeToString(data)}, nil
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
	return Result{Filename: filename, Base64: base64.StdEncoding.EncodeToString(data)}, nil
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
