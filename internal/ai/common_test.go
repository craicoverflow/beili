package ai

import "testing"

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
