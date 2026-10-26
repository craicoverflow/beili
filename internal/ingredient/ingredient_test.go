package ingredient

import (
	"testing"

	"github.com/craicoverflow/beili/internal/scaling"
)

func TestCanonical(t *testing.T) {
	tests := []struct {
		in   Parsed
		want string
	}{
		{Parsed{500, "g", "carrots", "sliced"}, "500 g carrots, sliced"},
		{Parsed{1.5, "kg", "potatoes", ""}, "1.5 kg potatoes"},
		{Parsed{2, "cup", "flour", ""}, "480 ml flour"},
		{Parsed{1, "tbsp", "olive oil", ""}, "15 ml olive oil"},
		{Parsed{1, "tsp", "salt", ""}, "5 ml salt"},
		{Parsed{8, "oz", "butter", ""}, "220 g butter"},
		{Parsed{1, "lb", "beef mince", ""}, "450 g beef mince"},
		{Parsed{3, "", "carrots", ""}, "3 carrots"},
		{Parsed{2, "clove", "garlic", "crushed"}, "2 cloves garlic, crushed"},
		{Parsed{1, "clove", "garlic", ""}, "1 clove garlic"},
		{Parsed{0, "", "salt", "to taste"}, "Salt, to taste"},
		{Parsed{1000, "ml", "milk", ""}, "1 l milk"},
		{Parsed{0.75, "tsp", "table salt", "(use 1/2 teaspoon if salted)"}, "4 ml table salt"},
		{Parsed{1, "", "egg", "large"}, "1 egg, large"},
	}
	for _, tt := range tests {
		if got := Canonical(tt.in); got != tt.want {
			t.Errorf("Canonical(%+v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Canonical lines must scale exactly: every amount scales by the ratio and
// nothing else in the line changes.
func TestCanonicalScales(t *testing.T) {
	tests := []struct {
		in    Parsed
		ratio float64
		want  string
	}{
		{Parsed{500, "g", "carrots", "sliced"}, 2, "1 kg carrots, sliced"},
		{Parsed{500, "g", "carrots", "sliced"}, 0.5, "250 g carrots, sliced"},
		{Parsed{240, "ml", "milk", ""}, 1.5, "360 ml milk"},
		{Parsed{3, "", "carrots", ""}, 2, "6 carrots"},
		{Parsed{1, "clove", "garlic", ""}, 3, "3 cloves garlic"},
		{Parsed{2, "clove", "garlic", "crushed"}, 0.5, "1 clove garlic, crushed"},
		{Parsed{0, "", "salt", "to taste"}, 2, "Salt, to taste"},
	}
	for _, tt := range tests {
		got := scaling.ScaleIngredient(Canonical(tt.in), tt.ratio)
		if got != tt.want {
			t.Errorf("scale(%q, %v) = %q, want %q", Canonical(tt.in), tt.ratio, got, tt.want)
		}
	}
}
