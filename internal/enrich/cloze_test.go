package enrich

import "testing"

func TestClozeText(t *testing.T) {
	t.Run("trusts well-formed model cloze", func(t *testing.T) {
		r := &llmResult{Cloze: "The detective was determined to {{c1::unravel}} the mystery."}
		got := clozeText(r, "unravel", "unravel", "")
		want := "The detective was determined to {{c1::unravel}} the mystery."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("escapes html but keeps cloze braces", func(t *testing.T) {
		r := &llmResult{Cloze: `He said "it will {{c1::unravel}}" & fail.`}
		got := clozeText(r, "unravel", "unravel", "")
		want := `He said &#34;it will {{c1::unravel}}&#34; &amp; fail.`
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to context sentence wrapping the seen form", func(t *testing.T) {
		r := &llmResult{Cloze: "no braces here"}
		got := clozeText(r, "unraveling", "unravel", "She was unraveling the whole plot.")
		want := "She was {{c1::unraveling}} the whole plot."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to first example when no context", func(t *testing.T) {
		r := &llmResult{Examples: []string{"They unravel it slowly."}}
		got := clozeText(r, "unravel", "unravel", "")
		want := "They {{c1::unravel}} it slowly."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("degenerate fallback clozes the headword alone", func(t *testing.T) {
		r := &llmResult{}
		got := clozeText(r, "xyzzy", "xyzzy", "")
		want := "{{c1::xyzzy}}"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestWrapFirstWord(t *testing.T) {
	cases := []struct {
		text, word, want string
		ok               bool
	}{
		{"The cat sat.", "cat", "The {{c1::cat}} sat.", true},
		{"Cat and cats.", "cat", "{{c1::Cat}} and cats.", true}, // whole-word, keeps case
		{"scatter the concatenation", "cat", "", false},         // no whole-word match
	}
	for _, c := range cases {
		got, ok := wrapFirstWord(c.text, c.word)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("wrapFirstWord(%q,%q) = %q,%v; want %q,%v", c.text, c.word, got, ok, c.want, c.ok)
		}
	}
}
