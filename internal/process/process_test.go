package process

import "testing"

func TestResolveWord(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantWord string
		wantSrc  string // "" means expect the passed-in source unchanged
		wantOK   bool
	}{
		{
			name:     "apple news url with highlight",
			raw:      "https://apple.news/agkeun-bxs1ubk2zvwuheqq?highlight=bicentennial",
			wantWord: "bicentennial",
			wantSrc:  "https://apple.news/agkeun-bxs1ubk2zvwuheqq?highlight=bicentennial",
			wantOK:   true,
		},
		{
			name:     "highlight with encoded space",
			raw:      "https://apple.news/xyz?highlight=give%20up",
			wantWord: "give up",
			wantSrc:  "https://apple.news/xyz?highlight=give%20up",
			wantOK:   true,
		},
		{
			name:   "bare url without highlight",
			raw:    "https://apple.news/agkeun-bxs1ubk2zvwuheqq",
			wantOK: false,
		},
		{
			name:   "headline sentence",
			raw:    "at 250, is the u.s. too divided to celebrate as one? - reuters",
			wantOK: false,
		},
		{
			name:     "plain word",
			raw:      "Serendipity",
			wantWord: "serendipity",
			wantOK:   true,
		},
		{
			name:     "short phrase",
			raw:      "give up",
			wantWord: "give up",
			wantOK:   true,
		},
		{
			name:   "empty",
			raw:    "   ",
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			word, src, ok := resolveWord(c.raw, "iOS")
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v (word=%q)", ok, c.wantOK, word)
			}
			if !ok {
				return
			}
			if word != c.wantWord {
				t.Errorf("word = %q, want %q", word, c.wantWord)
			}
			wantSrc := c.wantSrc
			if wantSrc == "" {
				wantSrc = "iOS"
			}
			if src != wantSrc {
				t.Errorf("src = %q, want %q", src, wantSrc)
			}
		})
	}
}
