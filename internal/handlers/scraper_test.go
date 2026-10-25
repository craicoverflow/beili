package handlers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/ingredient"
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
