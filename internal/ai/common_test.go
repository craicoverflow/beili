package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseStandardized(t *testing.T) {
	req := StandardizeRequest{Ingredients: []string{"2 cups flour"}, Instructions: []string{"Mix."}}

	got, err := parseStandardized("```json\n{\"ingredients\":[{\"qty\":2,\"unit\":\"cup\",\"name\":\"flour\",\"prep\":\"\"}],\"instructions\":[\"Mix.\"]}\n```", req)
	if err != nil {
		t.Fatal(err)
	}
	if p := got.Ingredients[0]; p.Qty != 2 || p.Unit != "cup" || p.Name != "flour" {
		t.Errorf("parsed = %+v", p)
	}

	if _, err := parseStandardized(`{"ingredients":[],"instructions":["Mix."]}`, req); err == nil {
		t.Error("expected ingredient count mismatch error")
	}
	if _, err := parseStandardized(`not json`, req); err == nil {
		t.Error("expected parse error")
	}
}

func TestAnthropicStandardize(t *testing.T) {
	var gotKey, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"ingredients\":[{\"qty\":2,\"unit\":\"cup\",\"name\":\"flour\",\"prep\":\"\"}],\"instructions\":[\"Mix.\"]}"}]}`))
	}))
	defer srv.Close()

	p := NewAnthropicProvider("k", "claude-haiku-5-5")
	p.endpoint = srv.URL
	got, err := p.Standardize(context.Background(), StandardizeRequest{Ingredients: []string{"2 cups flour"}, Instructions: []string{"Mix."}})
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "k" || gotModel != "claude-haiku-5-5" || got.Ingredients[0].Name != "flour" {
		t.Errorf("key=%q model=%q got=%+v", gotKey, gotModel, got)
	}
}
