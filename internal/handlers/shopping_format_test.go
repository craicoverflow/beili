package handlers

import "testing"

func TestFormatShoppingItem(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"grams tight", "625g mushrooms", "Mushrooms (625g)"},
		{"canonical grams", "500 g carrots, sliced", "Carrots (500g)"},
		{"ml spaced", "240 ml milk", "Milk (240ml)"},
		{"kg decimal", "1.2 kg flour", "Flour (1.2kg)"},
		{"bare count drops prep", "2 onions, diced", "Onions (2)"},
		{"bare count", "3 carrots", "Carrots (3)"},
		{"single", "1 egg", "Egg (1)"},
		{"count unit", "2 cloves garlic, crushed", "Garlic (2 cloves)"},
		{"count unit with of", "1 bunch of parsley", "Parsley (1 bunch)"},
		{"unit-like prefix is a name", "2 cantaloupes", "Cantaloupes (2)"},
		{"g-prefixed name", "3 garlic bulbs", "Garlic bulbs (3)"},
		{"or range", "10 or 11 tomatoes", "Tomatoes (10 or 11)"},
		{"dash range", "4-5 potatoes", "Potatoes (4-5)"},
		{"parenthetical dropped", "200 g feta (optional)", "Feta (200g)"},
		{"to taste", "Salt, to taste", "Salt"},
		{"no amount verbatim", "a pinch of pepper", "A pinch of pepper"},
		{"already capitalised name", "Olive oil", "Olive oil"},
		{"amount only", "2", "2"},
		{"whitespace", "  500g  butter  ", "Butter (500g)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatShoppingItem(tt.in); got != tt.want {
				t.Errorf("formatShoppingItem(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
