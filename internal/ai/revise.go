package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/craicoverflow/beili/internal/ingredient"
)

// Reviser rewrites a recipe to address cooking feedback. It is separate from
// Provider because it runs on a stronger model than import standardisation.
type Reviser interface {
	Revise(ctx context.Context, req ReviseRequest) (ReviseResponse, error)
}

// ReviseRequest carries the stored recipe and the feedback to act on.
type ReviseRequest struct {
	Name         string
	Servings     int // the serving count the ingredient quantities are for
	Ingredients  []string
	Instructions []string
	// Feedback is the lineage's notes, oldest first; the last entry is the
	// one being acted on. Earlier entries are already reflected in the recipe
	// and must not be undone.
	Feedback []string
}

// Reason explains one change. Ref is the ingredient name for an ingredient
// ("soy sauce") or "step N" for an instruction.
type Reason struct {
	Ref  string
	Text string
}

// ReviseResponse is the full revised recipe. What changed is worked out in
// code by comparing it with the original (internal/revision), never taken from
// the model's own description.
type ReviseResponse struct {
	Ingredients  []ingredient.Parsed
	Instructions []string
	Reasons      []Reason
}

const reviseSchemaHint = `Respond with ONLY a JSON object of the form {"ingredients":[{"qty":number,"unit":string,"name":string,"prep":string}],"instructions":[string],"reasons":[{"ref":string,"reason":string}]} and nothing else.`

func revisePrompt(req ReviseRequest) string {
	ingredientsJSON, _ := json.Marshal(req.Ingredients)
	instructionsJSON, _ := json.Marshal(req.Instructions)

	var fb strings.Builder
	for i, f := range req.Feedback {
		switch {
		case i == len(req.Feedback)-1:
			fmt.Fprintf(&fb, "NEW feedback to act on: %s\n", f)
		default:
			fmt.Fprintf(&fb, "Earlier feedback (already reflected in the recipe, do not undo it): %s\n", f)
		}
	}

	return fmt.Sprintf(`You are improving a home cook's recipe "%s" (quantities are for %d servings) based on their feedback after cooking it.

%s
Return the COMPLETE revised recipe: every ingredient and every instruction step, not just the ones you changed. Change only what the new feedback needs; keep everything else word-for-word identical so unchanged items can be recognised. You may add, remove, reword or reorder items when the feedback calls for it.

Ingredient field rules:
- qty: a single plain decimal number. Use 0 when there is no amount.
- unit: metric only — one of g, kg, ml, l, tsp, tbsp, or a count unit (clove, can, tin, pack, packet, slice, bunch, handful, pinch, stick, sprig, stalk, rasher, fillet) — or "" for plain counts. Keep quantities for %d servings.
- name: the product name only, lower case, no quantities or prep words.
- prep: optional short prep note without numbers or units, else "".
- Do not alter the wording or quantity of an ingredient the feedback does not concern.

Instruction rules: metric quantities and decimals, temperatures and times as written unless the feedback concerns them.

Also return "reasons": one short sentence per change you made. "ref" is the ingredient name exactly as in your "name" field (for a removed ingredient, its name from the original), or "step N" with N the 1-based step number (in your revised list; for a removed step, in the original).

Original ingredients (JSON array of strings):
%s

Original instructions (JSON array of step strings):
%s`, req.Name, req.Servings, fb.String(), req.Servings, ingredientsJSON, instructionsJSON)
}

// parseRevised decodes a reviser's JSON reply.
func parseRevised(text string) (ReviseResponse, error) {
	raw := strings.TrimSpace(text)
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
		Reasons      []struct {
			Ref    string `json:"ref"`
			Reason string `json:"reason"`
		} `json:"reasons"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ReviseResponse{}, fmt.Errorf("parse response: %w (raw: %.200s)", err, raw)
	}
	if len(parsed.Ingredients) == 0 || len(parsed.Instructions) == 0 {
		return ReviseResponse{}, fmt.Errorf("revised recipe is missing ingredients or instructions")
	}

	var out ReviseResponse
	out.Instructions = parsed.Instructions
	for _, p := range parsed.Ingredients {
		out.Ingredients = append(out.Ingredients, ingredient.Parsed{Qty: p.Qty, Unit: p.Unit, Name: p.Name, Prep: p.Prep})
	}
	for _, r := range parsed.Reasons {
		out.Reasons = append(out.Reasons, Reason{Ref: r.Ref, Text: r.Reason})
	}
	return out, nil
}

// Revise asks r to revise a recipe (two attempts, 90s each) and returns the
// revised ingredients in canonical stored form plus the instruction steps and
// the model's reasons.
func Revise(ctx context.Context, r Reviser, req ReviseRequest) ([]string, []string, []Reason, error) {
	var resp ReviseResponse
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		aiCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		resp, err = r.Revise(aiCtx, req)
		cancel()
		if err == nil {
			break
		}
		slog.Warn("ai revision attempt failed", "attempt", attempt, "err", err)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	lines := make([]string, 0, len(resp.Ingredients))
	for _, p := range resp.Ingredients {
		if line := ingredient.Canonical(p); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return nil, nil, nil, fmt.Errorf("revised recipe has no usable ingredients")
	}
	return lines, resp.Instructions, resp.Reasons, nil
}
