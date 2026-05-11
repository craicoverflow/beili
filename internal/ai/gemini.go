package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

	prompt := fmt.Sprintf(`You are a recipe scaling assistant. Scale the following recipe from %d servings to %d servings.

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
- Return ONLY valid JSON with no markdown, no code fences, and no explanation.

Ingredients (JSON array of strings):
%s

Instructions (JSON array of step strings):
%s

Required JSON response format:
{"ingredients": [...], "instructions": [...]}`,
		req.FromServings, req.ToServings, req.ToServings,
		string(ingredientsJSON),
		string(instructionsJSON),
	)

	result, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(prompt), nil)
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
		Ingredients  []string `json:"ingredients"`
		Instructions []string `json:"instructions"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		slog.Warn("gemini: failed to parse response, using original values", "err", err, "raw", raw)
		return NormalizeResponse{
			Ingredients:  req.Ingredients,
			Instructions: req.Instructions,
		}, nil
	}

	return NormalizeResponse{
		Ingredients:  parsed.Ingredients,
		Instructions: parsed.Instructions,
	}, nil
}
