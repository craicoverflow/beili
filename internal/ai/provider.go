package ai

import "context"

// Provider normalises recipe ingredients and instructions to a target serving size.
type Provider interface {
	NormalizeRecipe(ctx context.Context, req NormalizeRequest) (NormalizeResponse, error)
}

// NormalizeRequest carries the raw recipe data and the desired serving conversion.
type NormalizeRequest struct {
	Ingredients  []string
	Instructions []string
	FromServings int
	ToServings   int
}

// NormalizeResponse holds the scaled ingredients and instructions.
type NormalizeResponse struct {
	Ingredients  []string
	Instructions []string
}
