package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/genai"
)

const geminiModel = "gemini-2.5-flash"

type GeminiProvider struct {
	client *genai.Client
}

func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini client: %w", err)
	}
	return &GeminiProvider{client: client}, nil
}

func (g *GeminiProvider) NormalizeRecipe(ctx context.Context, req NormalizeRequest) (NormalizeResponse, error) {
	ingredientsJSON, _ := json.Marshal(req.Ingredients)
	instructionsJSON, _ := json.Marshal(req.Instructions)

	prompt := fmt.Sprintf(`You are a recipe scaling assistant. Scale the following recipe from %d servings to %d servings.

IMPORTANT RULES:
- Use metric units (grams, ml, litres, kg) wherever possible. Convert imperial measurements to metric.
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

	result, err := g.client.Models.GenerateContent(ctx, geminiModel, genai.Text(prompt), nil)
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
