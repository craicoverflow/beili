package ai

import (
	"context"
	"encoding/json"
	"fmt"

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

// Standardize parses every ingredient line into {qty, unit, name, prep} and
// rewrites instruction quantities in metric. The model only reads the text; it
// is never asked to do arithmetic or rounding — unit conversion and formatting
// happen in code (ingredient.Canonical) so results are exact and repeatable.
func (g *GeminiProvider) Standardize(ctx context.Context, req StandardizeRequest) (StandardizeResponse, error) {
	ingredientsJSON, _ := json.Marshal(req.Ingredients)
	instructionsJSON, _ := json.Marshal(req.Instructions)

	prompt := standardizePrompt(string(ingredientsJSON), string(instructionsJSON))

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"ingredients": {Type: genai.TypeArray, Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"qty":  {Type: genai.TypeNumber},
						"unit": {Type: genai.TypeString},
						"name": {Type: genai.TypeString},
						"prep": {Type: genai.TypeString},
					},
					Required: []string{"qty", "unit", "name", "prep"},
				}},
				"instructions": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			},
			Required: []string{"ingredients", "instructions"},
		},
	}

	result, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(prompt), cfg)
	if err != nil {
		return StandardizeResponse{}, fmt.Errorf("gemini generate: %w", err)
	}

	return parseStandardized(result.Text(), req)
}
