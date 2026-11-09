package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRevisePrompt_IncludesLineageFeedback(t *testing.T) {
	p := revisePrompt(ReviseRequest{
		Name:         "Curry",
		Servings:     5,
		Ingredients:  []string{"60 ml soy sauce"},
		Instructions: []string{"Simmer."},
		Feedback:     []string{"too sweet", "too thin"},
	})
	for _, want := range []string{
		"Earlier feedback (already reflected in the recipe, do not undo it): too sweet",
		"NEW feedback to act on: too thin",
		"60 ml soy sauce",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestParseRevised(t *testing.T) {
	got, err := parseRevised("```json\n{\"ingredients\":[{\"qty\":40,\"unit\":\"ml\",\"name\":\"soy sauce\",\"prep\":\"\"}],\"instructions\":[\"Simmer 10 minutes.\"],\"reasons\":[{\"ref\":\"soy sauce\",\"reason\":\"less salty\"}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ingredients[0].Qty != 40 || got.Instructions[0] != "Simmer 10 minutes." || got.Reasons[0].Text != "less salty" {
		t.Errorf("parsed = %+v", got)
	}
	if _, err := parseRevised(`{"ingredients":[],"instructions":["x"]}`); err == nil {
		t.Error("expected error for a recipe with no ingredients")
	}
	if _, err := parseRevised("nope"); err == nil {
		t.Error("expected parse error")
	}
}

func TestAnthropicRevise_UsesItsOwnModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"ingredients\":[{\"qty\":40,\"unit\":\"ml\",\"name\":\"soy sauce\",\"prep\":\"\"}],\"instructions\":[\"Simmer.\"],\"reasons\":[]}"}]}`))
	}))
	defer srv.Close()

	a := NewAnthropicProvider("key", "claude-sonnet-5-5")
	a.endpoint = srv.URL
	lines, steps, _, err := Revise(context.Background(), a, ReviseRequest{
		Name: "Curry", Servings: 5, Ingredients: []string{"60 ml soy sauce"}, Instructions: []string{"Simmer."}, Feedback: []string{"too salty"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "claude-sonnet-5-5" {
		t.Errorf("model = %q", gotModel)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "soy sauce") || steps[0] != "Simmer." {
		t.Errorf("lines = %v steps = %v", lines, steps)
	}
}
