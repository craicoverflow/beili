package ai

import (
	"context"

	"github.com/craicoverflow/beili/internal/ingredient"
)

// Provider cleans up a freshly imported recipe so the deterministic scaler
// (internal/scaling) can scale it exactly. It runs at import time only;
// manual edits are never sent through it.
type Provider interface {
	Standardize(ctx context.Context, req StandardizeRequest) (StandardizeResponse, error)
}

// StandardizeRequest carries the raw scraped recipe text.
type StandardizeRequest struct {
	Ingredients  []string
	Instructions []string
}

// StandardizeResponse holds each ingredient parsed into fields (the caller
// renders them with ingredient.Canonical) and the instruction steps with any
// imperial quantities converted to metric. Both are the same length and order
// as the request.
type StandardizeResponse struct {
	Ingredients  []ingredient.Parsed
	Instructions []string
}
