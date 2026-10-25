package handlers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/ingredient"
	"github.com/craicoverflow/beili/internal/scaling"
	"github.com/craicoverflow/beili/internal/scraper"
)

type stubAI struct {
	resp  ai.StandardizeResponse
	err   error
	calls int
}

func (s *stubAI) Standardize(context.Context, ai.StandardizeRequest) (ai.StandardizeResponse, error) {
	s.calls++
	return s.resp, s.err
}

func TestStandardizeRewritesImport(t *testing.T) {
	stub := &stubAI{resp: ai.StandardizeResponse{
		Ingredients: []ingredient.Parsed{
			{Qty: 2, Unit: "cup", Name: "flour"},
			{Qty: 1.5, Unit: "", Name: "carrots", Prep: "sliced"},
		},
		Instructions: []string{"Mix 480 ml flour."},
	}}
	h := &ScrapeHandler{aiProvider: stub}
	data := &scraper.RecipeData{
		Ingredients:  []string{"2 cups flour", "1 1/2 carrots, sliced"},
		Instructions: []string{"Mix 2 cups flour."},
	}
	h.standardize(context.Background(), data)

	if want := []string{"480 ml flour", "1.5 carrots, sliced"}; !reflect.DeepEqual(data.Ingredients, want) {
		t.Errorf("ingredients = %q, want %q", data.Ingredients, want)
	}
	if data.Instructions[0] != "Mix 480 ml flour." {
		t.Errorf("instructions = %q", data.Instructions)
	}
}

func TestStandardizeFailureKeepsScrapedValues(t *testing.T) {
	stub := &stubAI{err: errors.New("boom")}
	h := &ScrapeHandler{aiProvider: stub}
	data := &scraper.RecipeData{Ingredients: []string{"2 cups flour"}, Instructions: []string{"Mix."}}
	h.standardize(context.Background(), data)

	if data.Ingredients[0] != "2 cups flour" || stub.calls != 2 {
		t.Errorf("ingredients = %q after %d calls, want unchanged after 2", data.Ingredients, stub.calls)
	}
}

func TestStandardizeNoProviderIsNoop(t *testing.T) {
	h := &ScrapeHandler{}
	data := &scraper.RecipeData{Ingredients: []string{"2 cups flour"}}
	h.standardize(context.Background(), data)
	if data.Ingredients[0] != "2 cups flour" {
		t.Errorf("changed without provider: %q", data.Ingredients)
	}
}

// End to end from a messy imperial import: the (stubbed) model supplies parsed
// fields exactly as the prompt specifies, then the stored lines must scale
// exactly and produce clean shopping items.
func TestImportScalesAndShopsExactly(t *testing.T) {
	raw := []string{"2 cups plain flour", "1 1/2 tsp baking powder", "8 oz butter, softened", "1 lb carrots, sliced", "3 cloves garlic, crushed", "4 or 5 tomatoes", "salt to taste"}
	stub := &stubAI{resp: ai.StandardizeResponse{
		Ingredients: []ingredient.Parsed{
			{Qty: 2, Unit: "cup", Name: "plain flour"},
			{Qty: 1.5, Unit: "tsp", Name: "baking powder"},
			{Qty: 8, Unit: "oz", Name: "butter", Prep: "softened"},
			{Qty: 1, Unit: "lb", Name: "carrots", Prep: "sliced"},
			{Qty: 3, Unit: "clove", Name: "garlic", Prep: "crushed"},
			{Qty: 5, Unit: "", Name: "tomatoes"},
			{Qty: 0, Unit: "", Name: "salt", Prep: "to taste"},
		},
		Instructions: []string{},
	}}
	data := &scraper.RecipeData{Ingredients: raw}
	(&ScrapeHandler{aiProvider: stub}).standardize(context.Background(), data)

	wantStored := []string{"480 ml plain flour", "7.5 ml baking powder", "220 g butter, softened", "450 g carrots, sliced", "3 cloves garlic, crushed", "5 tomatoes", "Salt, to taste"}
	if !reflect.DeepEqual(data.Ingredients, wantStored) {
		t.Fatalf("stored = %q\nwant   %q", data.Ingredients, wantStored)
	}
	wantShop := []string{"Plain flour (480ml)", "Baking powder (7.5ml)", "Butter (220g)", "Carrots (450g)", "Garlic (3 cloves)", "Tomatoes (5)", "Salt"}
	for i, l := range data.Ingredients {
		if got := formatShoppingItem(l); got != wantShop[i] {
			t.Errorf("shopping(%q) = %q, want %q", l, got, wantShop[i])
		}
	}
	wantX2 := []string{"960 ml plain flour", "15 ml baking powder", "440 g butter, softened", "900 g carrots, sliced", "6 cloves garlic, crushed", "10 tomatoes", "Salt, to taste"}
	for i, l := range data.Ingredients {
		if got := scaling.ScaleIngredient(l, 2); got != wantX2[i] {
			t.Errorf("x2 %q = %q, want %q", l, got, wantX2[i])
		}
		// scaling is always from the stored base, so x2 then back to x1 is identity by construction
		if got := scaling.ScaleIngredient(l, 1); got != l {
			t.Errorf("x1 %q = %q", l, got)
		}
	}
}
