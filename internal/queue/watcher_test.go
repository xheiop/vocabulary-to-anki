package queue

import "testing"

func TestParseLine(t *testing.T) {
	cases := map[string]string{
		"serendipity":            "serendipity",
		"  ephemeral  ":          "ephemeral",
		"ubiquitous\t2026-07-04": "ubiquitous",
		"\t":                     "",
		"":                       "",
		"multi word phrase":      "multi word phrase",
	}
	for in, want := range cases {
		if got := parseLine(in); got != want {
			t.Errorf("parseLine(%q) = %q, want %q", in, got, want)
		}
	}
}
