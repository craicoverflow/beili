package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/craicoverflow/beili/internal/ingredient"
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

	prompt := fmt.Sprintf(`You are cleaning up a scraped recipe so a program can scale it. Parse each ingredient line into fields. Do NOT do any maths, conversion or rounding of amounts in "qty" — copy the number as written, with the unit as written.

Ingredient field rules:
- qty: a single plain decimal number (e.g. 0.5, 1.25, 3). Convert fractions and mixed numbers ("1 1/2", "½") to decimals. For ranges ("4 or 5", "2-3") use the upper value. Use 0 when there is no amount ("salt to taste", "a pinch" with no number: use qty 1, unit "pinch").
- unit: the unit as written, normalised to ONE of: g, kg, mg, ml, l, cl, dl, tsp, tbsp, cup, floz, pint, oz, lb, clove, can, tin, pack, packet, slice, bunch, handful, pinch, stick, sprig, stalk, rasher, fillet — or "" for plain counts ("3 carrots") and when there is no amount. If a package size is given ("1 x 400g tin tomatoes"), set qty to the total weight/volume (400), unit g/ml, and put "tinned" in prep if useful.
- name: the shopping-list product name only, lower case unless a proper noun, singular/plural as natural ("carrots", "olive oil", "self-raising flour"). No quantities, no prep words.
- prep: optional trailing prep or note ("sliced", "finely chopped", "to taste", "at room temperature"), else "".

Instruction rules:
- Return every instruction step with the same wording, except convert imperial quantities to metric (tsp=5ml, tbsp=15ml, cup=240ml, fl oz=30ml, oz=28g, lb=454g, round sensibly) and replace fractions with decimals. Do not change temperatures or times.

Return EXACTLY one ingredient object per input ingredient and one string per input step, in the same order. Never merge, split, drop or add items.

Ingredients (JSON array of strings):
%s

Instructions (JSON array of step strings):
%s`, string(ingredientsJSON), string(instructionsJSON))

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

	raw := strings.TrimSpace(result.Text())
	// Strip markdown code fences if the model adds them despite instructions
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		Ingredients []struct {
			Qty  float64 `json:"qty"`
			Unit string  `json:"unit"`
			Name string  `json:"name"`
			Prep string  `json:"prep"`
		} `json:"ingredients"`
		Instructions []string `json:"instructions"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return StandardizeResponse{}, fmt.Errorf("gemini: parse response: %w (raw: %.200s)", err, raw)
	}
	if len(parsed.Ingredients) != len(req.Ingredients) {
		return StandardizeResponse{}, fmt.Errorf("gemini: ingredient count mismatch: sent %d, got %d", len(req.Ingredients), len(parsed.Ingredients))
	}
	if len(parsed.Instructions) != len(req.Instructions) {
		return StandardizeResponse{}, fmt.Errorf("gemini: instruction count mismatch: sent %d, got %d", len(req.Instructions), len(parsed.Instructions))
	}

	out := StandardizeResponse{Instructions: parsed.Instructions}
	for _, p := range parsed.Ingredients {
		out.Ingredients = append(out.Ingredients, ingredient.Parsed{Qty: p.Qty, Unit: p.Unit, Name: p.Name, Prep: p.Prep})
	}
	return out, nil
}
