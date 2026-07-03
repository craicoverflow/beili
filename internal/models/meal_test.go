package models

import "testing"

func TestPluralize(t *testing.T) {
	cases := []struct {
		n    int
		word string
		want string
	}{
		{0, "ingredient", "0 ingredients"},
		{1, "ingredient", "1 ingredient"},
		{2, "ingredient", "2 ingredients"},
		{1, "step", "1 step"},
		{9, "step", "9 steps"},
	}
	for _, c := range cases {
		if got := Pluralize(c.n, c.word); got != c.want {
			t.Errorf("Pluralize(%d, %q) = %q, want %q", c.n, c.word, got, c.want)
		}
	}
}
