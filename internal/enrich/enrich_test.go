package enrich

import "testing"

func TestParseLLMJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // expected Definition
		err  bool
	}{
		{
			name: "plain json",
			in:   `{"part_of_speech":"noun","definition":"a happy accident","examples":["It was serendipity."],"collocations":["pure serendipity"]}`,
			want: "a happy accident",
		},
		{
			name: "fenced json",
			in:   "```json\n{\"definition\":\"luck\",\"examples\":[]}\n```",
			want: "luck",
		},
		{
			name: "prose then json",
			in:   "Here you go: {\"definition\":\"fortune\"}",
			want: "fortune",
		},
		{
			name: "no json",
			in:   "sorry I cannot help",
			err:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseLLMJSON(c.in)
			if c.err {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Definition != c.want {
				t.Fatalf("definition = %q, want %q", got.Definition, c.want)
			}
		})
	}
}

func TestParseCLIOutput(t *testing.T) {
	// Real envelope shape from `claude -p --output-format json`, whose result
	// field wraps the model's JSON in markdown fences.
	envelope := `{"type":"result","subtype":"success","is_error":false,"result":"` +
		"```json\\n{\\\"part_of_speech\\\":\\\"noun\\\",\\\"definition\\\":\\\"a happy accident\\\",\\\"examples\\\":[\\\"Pure serendipity.\\\"]}\\n```" +
		`"}`
	got, err := parseCLIOutput([]byte(envelope))
	if err != nil {
		t.Fatalf("parseCLIOutput: %v", err)
	}
	if got.Definition != "a happy accident" || got.PartOfSpeech != "noun" {
		t.Fatalf("parsed = %+v", got)
	}

	// is_error envelope surfaces an error.
	if _, err := parseCLIOutput([]byte(`{"is_error":true,"result":"model failed"}`)); err == nil {
		t.Fatal("expected error for is_error envelope")
	}

	// Bare JSON (non-envelope) still parses.
	got, err = parseCLIOutput([]byte(`{"definition":"luck"}`))
	if err != nil || got.Definition != "luck" {
		t.Fatalf("bare json fallback: %+v, %v", got, err)
	}
}

func TestEnvelopeError(t *testing.T) {
	// Real shape from a non-zero-exit run: is_error with the reason in result.
	out := `{"is_error":true,"duration_api_ms":0,"result":"Failed to authenticate: OAuth session expired and could not be refreshed"}`
	if got := envelopeError([]byte(out)); got != "Failed to authenticate: OAuth session expired and could not be refreshed" {
		t.Fatalf("envelopeError = %q", got)
	}
	if got := envelopeError([]byte("not json at all")); got != "" {
		t.Fatalf("non-envelope should yield empty, got %q", got)
	}
}

func TestBoldWord(t *testing.T) {
	cases := []struct {
		text, word, want string
	}{
		{"The serendipity of it.", "serendipity", "The <b>serendipity</b> of it."},
		{"Serendipity struck.", "serendipity", "<b>Serendipity</b> struck."},
		{"unserendipitous mood", "serendip", "unserendipitous mood"}, // no whole-word match
		{"run runs running", "run", "<b>run</b> runs running"},       // only exact word
		{"nothing here", "", "nothing here"},                         // empty word is a no-op
	}
	for _, c := range cases {
		if got := boldWord(c.text, c.word); got != c.want {
			t.Errorf("boldWord(%q,%q) = %q, want %q", c.text, c.word, got, c.want)
		}
	}
}
