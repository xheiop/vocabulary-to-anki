// Package pending persists words that could not be added because Anki was
// offline, so they can be retried later. Storage is a single JSON file guarded
// by a mutex; the working set is small (a handful of words at most).
package pending

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Item is one queued add request awaiting a running Anki.
type Item struct {
	Word    string `json:"word"`
	Context string `json:"context,omitempty"`
	Source  string `json:"source,omitempty"`
}

// Store is a file-backed, concurrency-safe set of pending items.
type Store struct {
	path string
	mu   sync.Mutex
}

// New returns a store backed by path, creating the parent directory.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

// Add appends an item, de-duplicating by (word, context).
func (s *Store) Add(item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.readLocked()
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Word == item.Word && it.Context == item.Context {
			return nil // already queued
		}
	}
	items = append(items, item)
	return s.writeLocked(items)
}

// List returns a copy of the currently pending items.
func (s *Store) List() ([]Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked()
}

// Remove deletes an item matching (word, context).
func (s *Store) Remove(item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.readLocked()
	if err != nil {
		return err
	}
	kept := items[:0]
	for _, it := range items {
		if it.Word == item.Word && it.Context == item.Context {
			continue
		}
		kept = append(kept, it)
	}
	return s.writeLocked(kept)
}

func (s *Store) readLocked() ([]Item, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) writeLocked(items []Item) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically via a temp file + rename.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
