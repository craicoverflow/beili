package ai

import (
	"context"
	"log/slog"
	"time"

	"github.com/craicoverflow/beili/internal/ingredient"
)

// Standardize runs p over a recipe's ingredients and instructions (two
// attempts, 45s each) and returns them in the canonical stored form: metric,
// one plain number, "<qty> <unit> <name>[, <prep>]". An ingredient the model
// returns without a name keeps its original line.
func Standardize(ctx context.Context, p Provider, ingredients, instructions []string) ([]string, []string, error) {
	req := StandardizeRequest{Ingredients: ingredients, Instructions: instructions}

	var resp StandardizeResponse
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		aiCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		resp, err = p.Standardize(aiCtx, req)
		cancel()
		if err == nil {
			break
		}
		slog.Warn("ai standardisation attempt failed", "attempt", attempt, "err", err)
	}
	if err != nil {
		return nil, nil, err
	}

	lines := make([]string, len(resp.Ingredients))
	for i, parsed := range resp.Ingredients {
		lines[i] = ingredient.Canonical(parsed)
		if lines[i] == "" {
			lines[i] = ingredients[i]
		}
	}
	return lines, resp.Instructions, nil
}
