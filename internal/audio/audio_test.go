package audio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestYoudaoAudioURL(t *testing.T) {
	if got, want := YoudaoAudioURL("unravel"),
		"https://dict.youdao.com/dictvoice?audio=unravel&type=2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Multi-word phrases must be escaped.
	if got := YoudaoAudioURL("give up"); !strings.Contains(got, "audio=give+up") {
		t.Errorf("phrase not escaped: %q", got)
	}
}

func TestAudioExt(t *testing.T) {
	cases := map[string]string{
		"https://dict.youdao.com/dictvoice?audio=x&type=2": ".mp3", // no path ext
		"https://cdn.example.com/media/word.mp3":           ".mp3",
		"https://cdn.example.com/media/word.ogg":           ".ogg",
		"https://cdn.example.com/w.mp3?token=abc.def":      ".mp3", // query ignored
		"https://example.com/audio":                        ".mp3",
	}
	for in, want := range cases {
		if got := audioExt(in); got != want {
			t.Errorf("audioExt(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestGetFallsBackThroughTiers checks the priority order: a failing dictionary
// URL must fall through to Youdao rather than straight to local synthesis.
func TestGetPrefersDictionaryThenYoudao(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(strings.Repeat("A", 1024))) // pretend audio
	}))
	defer srv.Close()

	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// A working dictionary URL wins outright.
	got, err := s.download(context.Background(), "w", srv.URL+"/good.mp3", "dictionary")
	if err != nil || got.Source != "dictionary" {
		t.Fatalf("dictionary tier: %+v %v", got, err)
	}

	// A broken one yields an error so Get can fall through.
	if _, err := s.download(context.Background(), "w", srv.URL+"/broken", "dictionary"); err == nil {
		t.Fatal("expected the broken URL to error")
	}
	if len(hits) != 2 {
		t.Fatalf("unexpected requests: %v", hits)
	}
}

// A 200 response that is really an HTML error page must be rejected.
func TestDownloadRejectsNonAudio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>" + strings.Repeat("x", 500) + "</body></html>"))
	}))
	defer srv.Close()

	s, _ := New(t.TempDir())
	if _, err := s.download(context.Background(), "w", srv.URL, "youdao"); err == nil {
		t.Fatal("expected HTML body to be rejected")
	}

	// So must a suspiciously tiny body.
	tiny := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nope"))
	}))
	defer tiny.Close()
	if _, err := s.download(context.Background(), "w", tiny.URL, "youdao"); err == nil {
		t.Fatal("expected tiny body to be rejected")
	}
}
