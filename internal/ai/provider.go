package ai

import "context"

// Provider normalises recipe ingredients and instructions to a target serving size.
type Provider interface {
	NormalizeRecipe(ctx context.Context, req NormalizeRequest) (NormalizeResponse, error)
	// ExtractShoppingNames derives a short, shopping-list-friendly product name
	// for each ingredient line (stripping quantities, units and prep notes,
	// e.g. "240 ml cherry tomatoes, halved" -> "Cherry tomatoes"). Returns
	// exactly len(ingredients) names in the same order.
	ExtractShoppingNames(ctx context.Context, ingredients []string) ([]string, error)
}

// NormalizeRequest carries the raw recipe data and the desired serving conversion.
type NormalizeRequest struct {
	Ingredients  []string
	Instructions []string
	FromServings int
	ToServings   int
}

// NormalizeResponse holds the scaled ingredients and instructions, plus a
// derived shopping-list-friendly name per ingredient (same order as
// Ingredients).
type NormalizeResponse struct {
	Ingredients   []string
	Instructions  []string
	ShoppingNames []string
}
