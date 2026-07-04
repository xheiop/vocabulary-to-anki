package pending

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "pending.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if items, _ := s.List(); len(items) != 0 {
		t.Fatalf("new store not empty: %v", items)
	}

	a := Item{Word: "serendipity", Context: "pure serendipity"}
	b := Item{Word: "ephemeral"}
	if err := s.Add(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(b); err != nil {
		t.Fatal(err)
	}
	// Duplicate add is a no-op.
	if err := s.Add(a); err != nil {
		t.Fatal(err)
	}

	items, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d: %v", len(items), items)
	}

	if err := s.Remove(a); err != nil {
		t.Fatal(err)
	}
	items, _ = s.List()
	if len(items) != 1 || items[0].Word != "ephemeral" {
		t.Fatalf("after remove want [ephemeral], got %v", items)
	}
}

func TestStorePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.json")
	s1, _ := New(path)
	if err := s1.Add(Item{Word: "quixotic"}); err != nil {
		t.Fatal(err)
	}
	s2, _ := New(path)
	items, _ := s2.List()
	if len(items) != 1 || items[0].Word != "quixotic" {
		t.Fatalf("reload want [quixotic], got %v", items)
	}
}
