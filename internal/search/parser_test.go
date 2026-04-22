package search

import (
	"testing"
)

func TestParseFTSQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"chicken", "chicken*"},
		{"chicken broccoli", "chicken* AND broccoli*"},
		{"chicken OR beef", "chicken* OR beef*"},
		{"chicken AND broccoli", "chicken* AND broccoli*"},
		{`"olive oil"`, `"olive oil"`},
		{"(chicken OR beef) AND broccoli", "( chicken* OR beef* ) AND broccoli*"},
		// injection: raw FTS5 operators stripped from bare terms
		{"chicken*", "chicken*"},
		{"term:foo", "termfoo*"},
	}
	for _, tt := range tests {
		got := ParseFTSQuery(tt.input)
		if got != tt.want {
			t.Errorf("ParseFTSQuery(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}
