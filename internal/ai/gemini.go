package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type GeminiProvider struct {
	client *genai.Client
	model  string
}

func NewGeminiProvider(ctx context.Context, apiKey, model string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini client: %w", err)
	}
	return &GeminiProvider{client: client, model: model}, nil
}

func (g *GeminiProvider) NormalizeRecipe(ctx context.Context, req NormalizeRequest) (NormalizeResponse, error) {
	ingredientsJSON, _ := json.Marshal(req.Ingredients)
	instructionsJSON, _ := json.Marshal(req.Instructions)

	var task string
	if req.FromServings > 0 && req.FromServings != req.ToServings {
		task = fmt.Sprintf("Scale the following recipe from %d servings to %d servings, converting all quantities to metric units.", req.FromServings, req.ToServings)
	} else {
		task = "Convert all quantities in the following recipe to metric units. Do NOT change the amounts — only convert units."
	}

	prompt := fmt.Sprintf(`You are a recipe scaling assistant. %s

IMPORTANT RULES:
- Output MUST use metric units only (g, kg, ml, l). Never output tsp, tbsp, teaspoon, tablespoon, cup, fl oz, oz, or lb in the ingredients list.
- Convert imperial to metric BEFORE scaling, using this table:
  - 1 tsp = 5 ml
  - 1 tbsp = 15 ml
  - 1 cup = 240 ml
  - 1 fl oz = 30 ml
  - 1 oz = 28 g
  - 1 lb = 454 g
- After scaling by the serving ratio, round to a sensible value:
  - under 10 ml/g: round to nearest 1
  - 10–100 ml/g: round to nearest 5
  - 100–1000 ml/g: round to nearest 10
  - over 1000 ml/g: prefer kg/l with one decimal (e.g. "1.2 kg")
- Never use fractions like "5/12" or "1/3". Use whole numbers or at most one decimal place.
- Preserve the original wording and style for non-quantifiable items (e.g. "salt to taste", "a pinch of pepper").
- For approximate quantities like "4 or 5 tomatoes", scale proportionally and keep the approximate style (e.g. "10 or 11 tomatoes").
- Update any serving-count references in the instructions (e.g. "serves 4" → "serves %d").
- Return EXACTLY the same number of ingredients and the same number of instruction steps as the input. Never merge, split, drop, or add items.
- Do not change oven temperatures or cooking times.
- Also return "shopping_names": for each (scaled) ingredient, a short shopping-list-friendly product name — strip the quantity, unit, and any trailing prep note (e.g. "240 ml cherry tomatoes, halved" -> "Cherry tomatoes", "2 onions, diced" -> "Onions"). Capitalise the first letter. Keep brand/variety words that matter for shopping (e.g. "Self-raising flour"). Return exactly one name per ingredient, same order.

Ingredients (JSON array of strings):
%s

Instructions (JSON array of step strings):
%s`,
		task, req.ToServings,
		string(ingredientsJSON),
		string(instructionsJSON),
	)

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"ingredients":    {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"instructions":   {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"shopping_names": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			},
			Required: []string{"ingredients", "instructions", "shopping_names"},
		},
	}

	result, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(prompt), cfg)
	if err != nil {
		return NormalizeResponse{}, fmt.Errorf("gemini generate: %w", err)
	}

	raw := strings.TrimSpace(result.Text())
	// Strip markdown code fences if the model adds them despite instructions
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		Ingredients   []string `json:"ingredients"`
		Instructions  []string `json:"instructions"`
		ShoppingNames []string `json:"shopping_names"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return NormalizeResponse{}, fmt.Errorf("gemini: parse response: %w (raw: %.200s)", err, raw)
	}
	if len(parsed.Ingredients) != len(req.Ingredients) {
		return NormalizeResponse{}, fmt.Errorf("gemini: ingredient count mismatch: sent %d, got %d", len(req.Ingredients), len(parsed.Ingredients))
	}
	if len(parsed.Instructions) != len(req.Instructions) {
		return NormalizeResponse{}, fmt.Errorf("gemini: instruction count mismatch: sent %d, got %d", len(req.Instructions), len(parsed.Instructions))
	}
	if len(parsed.ShoppingNames) != len(req.Ingredients) {
		// Non-fatal: shopping names are a nice-to-have. Leave empty so
		// callers fall back to regex parsing of the raw ingredient string.
		parsed.ShoppingNames = nil
	}

	return NormalizeResponse{
		Ingredients:   parsed.Ingredients,
		Instructions:  parsed.Instructions,
		ShoppingNames: parsed.ShoppingNames,
	}, nil
}

// ExtractShoppingNames derives a shopping-list-friendly name for each
// ingredient line without touching quantities or instructions. Used when a
// recipe is saved without triggering full metric/serving normalisation (e.g.
// it's already metric and at the target serving size) but still needs
// shopping names generated.
func (g *GeminiProvider) ExtractShoppingNames(ctx context.Context, ingredients []string) ([]string, error) {
	ingredientsJSON, _ := json.Marshal(ingredients)

	prompt := fmt.Sprintf(`For each recipe ingredient line below, derive a short shopping-list-friendly product name: strip the quantity, unit, and any trailing prep note (e.g. "240 ml cherry tomatoes, halved" -> "Cherry tomatoes", "2 onions, diced" -> "Onions"). Capitalise the first letter. Keep brand/variety words that matter for shopping (e.g. "Self-raising flour"). Return exactly one name per ingredient, same order, as many names as ingredients.

Ingredients (JSON array of strings):
%s`, string(ingredientsJSON))

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"shopping_names": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			},
			Required: []string{"shopping_names"},
		},
	}

	result, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(prompt), cfg)
	if err != nil {
		return nil, fmt.Errorf("gemini generate: %w", err)
	}

	raw := strings.TrimSpace(result.Text())
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		ShoppingNames []string `json:"shopping_names"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("gemini: parse response: %w (raw: %.200s)", err, raw)
	}
	if len(parsed.ShoppingNames) != len(ingredients) {
		return nil, fmt.Errorf("gemini: shopping name count mismatch: sent %d, got %d", len(ingredients), len(parsed.ShoppingNames))
	}
	return parsed.ShoppingNames, nil
}
