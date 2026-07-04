// Package queue watches the iCloud Drive file that the iOS Shortcut appends to.
// Each non-empty line is a word (optionally "word\ttimestamp"); consumed lines
// are cleared from the file. Because iCloud may create the file after startup,
// the watcher observes the parent directory rather than the file itself.
package queue

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/xheiop/vocab2anki/internal/process"
)

// Watcher tails the queue file and forwards words to the jobs channel.
type Watcher struct {
	path string
	jobs chan<- process.Request
}

// New returns a watcher for the file at path.
func New(path string, jobs chan<- process.Request) *Watcher {
	return &Watcher{path: path, jobs: jobs}
}

// Run watches until ctx is cancelled. It processes any words already in the
// file at startup, then reacts to writes. Debouncing coalesces the burst of
// events iCloud emits per sync.
func (w *Watcher) Run(ctx context.Context) error {
	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(dir); err != nil {
		return err
	}
	log.Printf("watching queue file %s", w.path)

	// Drain anything already sitting in the file.
	w.drain(ctx)

	var debounce *time.Timer
	debounced := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Clean(event.Name) != filepath.Clean(w.path) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(400*time.Millisecond, func() {
				select {
				case debounced <- struct{}{}:
				default:
				}
			})

		case <-debounced:
			w.drain(ctx)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("queue watcher error: %v", err)
		}
	}
}

// drain reads every line, forwards each as a job, and truncates the file.
func (w *Watcher) drain(ctx context.Context) {
	data, err := os.ReadFile(w.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("queue read: %v", err)
		}
		return
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return
	}

	lines := strings.Split(string(data), "\n")
	sent := 0
	for _, line := range lines {
		word := parseLine(line)
		if word == "" {
			continue
		}
		req := process.Request{Word: word, Source: "iOS"}
		select {
		case w.jobs <- req:
			sent++
			log.Printf("intake(icloud): %q", word)
		case <-ctx.Done():
			return
		}
	}

	// Clear the consumed lines. Any line written between the read above and
	// this truncate would be lost; the debounce window makes that unlikely, and
	// duplicates are harmless because Anki deduplicates by word.
	if err := os.WriteFile(w.path, []byte{}, 0o644); err != nil {
		log.Printf("queue truncate: %v", err)
	}
	if sent > 0 {
		log.Printf("queue drained %d word(s)", sent)
	}
}

// parseLine extracts the word from a queue line. Lines are "word" or
// "word\ttimestamp"; leading/trailing whitespace is trimmed.
func parseLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if i := strings.IndexByte(line, '\t'); i >= 0 {
		return strings.TrimSpace(line[:i])
	}
	return line
}
