package revision

import (
	"reflect"
	"testing"
)

func TestNameKey(t *testing.T) {
	cases := map[string]string{
		"30 ml soy sauce, to taste": "soy sauce",
		"2 cloves garlic":           "garlic",
		"3 carrots":                 "carrots",
		"500 g Chicken thighs":      "chicken thighs",
		"salt":                      "salt",
	}
	for in, want := range cases {
		if got := NameKey(in); got != want {
			t.Errorf("NameKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIngredients(t *testing.T) {
	original := []string{"500 g chicken thighs", "60 ml soy sauce", "2 tbsp honey", "1 tsp chilli flakes"}
	revised := []string{"500 g chicken thighs", "40 ml soy sauce", "1 tbsp honey", "2 cloves garlic"}
	reasons := []Reason{{Ref: "Soy Sauce", Text: "less salty"}, {Ref: "garlic", Text: "more depth"}}

	got := Ingredients(original, revised, reasons)
	want := []Change{
		{Kind: Changed, Old: "60 ml soy sauce", New: "40 ml soy sauce", Reason: "less salty"},
		{Kind: Changed, Old: "2 tbsp honey", New: "1 tbsp honey"},
		{Kind: Removed, Old: "1 tsp chilli flakes"},
		{Kind: Added, New: "2 cloves garlic", Reason: "more depth"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestIngredients_Identical(t *testing.T) {
	l := []string{"1 onion", "1 onion", "2 carrots"}
	if got := Ingredients(l, l, nil); len(got) != 0 {
		t.Errorf("identical lists produced changes: %+v", got)
	}
}

func TestIngredients_DuplicatesAreCounted(t *testing.T) {
	got := Ingredients([]string{"1 onion", "1 onion"}, []string{"1 onion"}, nil)
	want := []Change{{Kind: Removed, Old: "1 onion"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestInstructions(t *testing.T) {
	original := []string{"Chop the onion.", "Fry for 5 minutes.", "Add stock.", "Serve."}
	revised := []string{"Chop the onion.", "Fry for 8 minutes.", "Add stock.", "Simmer 10 minutes.", "Serve."}
	reasons := []Reason{{Ref: "step 2", Text: "softer"}, {Ref: "step 4", Text: "thicker sauce"}}

	got := Instructions(original, revised, reasons)
	want := []Change{
		{Kind: Changed, Old: "Fry for 5 minutes.", New: "Fry for 8 minutes.", Reason: "softer"},
		{Kind: Added, New: "Simmer 10 minutes.", Reason: "thicker sauce"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestInstructions_Removed(t *testing.T) {
	got := Instructions([]string{"A.", "B.", "C."}, []string{"A.", "C."}, nil)
	want := []Change{{Kind: Removed, Old: "B."}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestInstructions_Identical(t *testing.T) {
	l := []string{"A.", "B."}
	if got := Instructions(l, l, nil); len(got) != 0 {
		t.Errorf("identical lists produced changes: %+v", got)
	}
}
