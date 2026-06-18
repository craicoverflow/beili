package handlers

import "testing"

func TestFormatShoppingItem(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// amount + unit, tight and spaced
		{"grams tight", "625g mushrooms", "Mushrooms (625g)"},
		{"ml spaced", "240 ml milk", "Milk (240ml)"},
		{"kg decimal", "1.2 kg flour", "Flour (1.2kg)"},
		// count with no unit
		{"bare count", "2 onions, diced", "Onions, diced (2)"},
		{"single", "1 egg", "Egg (1)"},
		// ranges (Gemini emits these for approximate quantities)
		{"or range", "10 or 11 tomatoes", "Tomatoes (10 or 11)"},
		{"dash range", "4-5 potatoes", "Potatoes (4-5)"},
		// no parseable amount -> verbatim, unchanged
		{"to taste", "salt to taste", "salt to taste"},
		{"pinch", "a pinch of pepper", "a pinch of pepper"},
		{"already capitalised name", "Olive oil", "Olive oil"},
		// edge: amount only, no name -> verbatim
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

func TestSplitShoppingItem(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantName   string
		wantAmount string
	}{
		{"grams tight", "625g mushrooms", "Mushrooms", "625g"},
		{"ml spaced", "240 ml milk", "Milk", "240ml"},
		{"kg decimal", "1.2 kg flour", "Flour", "1.2kg"},
		{"bare count", "2 onions, diced", "Onions, diced", "2"},
		{"or range", "10 or 11 tomatoes", "Tomatoes", "10 or 11"},
		{"whitespace", "  500g  butter  ", "Butter", "500g"},
		// no parseable amount -> name verbatim, empty amount
		{"to taste", "salt to taste", "salt to taste", ""},
		{"already capitalised", "Olive oil", "Olive oil", ""},
		{"amount only", "2", "2", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotAmount := splitShoppingItem(tt.in)
			if gotName != tt.wantName || gotAmount != tt.wantAmount {
				t.Errorf("splitShoppingItem(%q) = (%q, %q), want (%q, %q)",
					tt.in, gotName, gotAmount, tt.wantName, tt.wantAmount)
			}
		})
	}
}
