package enrich

import "testing"

func TestClozeText(t *testing.T) {
	t.Run("trusts well-formed model cloze", func(t *testing.T) {
		r := &llmResult{Cloze: "The detective was determined to {{c1::unravel}} the mystery."}
		got := clozeText(r, "unravel", "unravel", "", "")
		want := "The detective was determined to {{c1::unravel}} the mystery."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("escapes html but keeps cloze braces", func(t *testing.T) {
		r := &llmResult{Cloze: `He said "it will {{c1::unravel}}" & fail.`}
		got := clozeText(r, "unravel", "unravel", "", "")
		want := `He said &#34;it will {{c1::unravel}}&#34; &amp; fail.`
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to context sentence wrapping the seen form", func(t *testing.T) {
		r := &llmResult{Cloze: "no braces here"}
		got := clozeText(r, "unraveling", "unravel", "She was unraveling the whole plot.", "")
		want := "She was {{c1::unraveling}} the whole plot."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to first example when no context", func(t *testing.T) {
		r := &llmResult{Examples: []string{"They unravel it slowly."}}
		got := clozeText(r, "unravel", "unravel", "", "")
		want := "They {{c1::unravel}} it slowly."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("degenerate fallback clozes the headword alone", func(t *testing.T) {
		r := &llmResult{}
		got := clozeText(r, "xyzzy", "xyzzy", "", "")
		want := "{{c1::xyzzy}}"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestClozeTextWithHint(t *testing.T) {
	t.Run("adds the chinese gloss as an anki cloze hint", func(t *testing.T) {
		r := &llmResult{Cloze: "The detective was determined to {{c1::unravel}} the mystery."}
		got := clozeText(r, "unravel", "unravel", "", "解开、阐明")
		want := "The detective was determined to {{c1::unravel::解开、阐明}} the mystery."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("hint reaches the deterministic fallback too", func(t *testing.T) {
		r := &llmResult{}
		got := clozeText(r, "unravel", "unravel", "They unravel it slowly.", "解开")
		want := "They {{c1::unravel::解开}} it slowly."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("does not double-hint when the model already supplied one", func(t *testing.T) {
		r := &llmResult{Cloze: "It will {{c1::unravel::already}} soon."}
		got := clozeText(r, "unravel", "unravel", "", "解开")
		want := "It will {{c1::unravel::already}} soon."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("only the first deletion gets the hint", func(t *testing.T) {
		r := &llmResult{Cloze: "{{c1::run}} and {{c2::walk}}."}
		got := clozeText(r, "run", "run", "", "跑")
		want := "{{c1::run::跑}} and {{c2::walk}}."
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestSanitizeHint(t *testing.T) {
	cases := map[string]string{
		"  解开  ": "解开",
		"解{开}":   "解开",
		"解::开":   "解、开",
		"一二三四五六七八九十一二三四五六七八九十一二三": "一二三四五六七八九十一二三四五六七八九十", // capped at 20 runes
		"": "",
	}
	for in, want := range cases {
		if got := sanitizeHint(in); got != want {
			t.Errorf("sanitizeHint(%q) = %q, want %q", in, got, want)
		}
	}
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
