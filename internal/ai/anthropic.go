package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// anthropicEndpoint is the Messages API. It is called over plain HTTP rather
// than through the official SDK: the SDK is a single enormous package that
// needs far more memory to compile than the small machines this app is built
// on (Docker VMs, HA add-on builders) have.
const anthropicEndpoint = "https://api.anthropic.com/v1/messages"

// AnthropicProvider standardises recipes with a Claude model.
type AnthropicProvider struct {
	apiKey   string
	model    string
	endpoint string
	http     *http.Client
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{apiKey: apiKey, model: model, endpoint: anthropicEndpoint, http: http.DefaultClient}
}

func (a *AnthropicProvider) Standardize(ctx context.Context, req StandardizeRequest) (StandardizeResponse, error) {
	ingredientsJSON, _ := json.Marshal(req.Ingredients)
	instructionsJSON, _ := json.Marshal(req.Instructions)
	prompt := standardizePrompt(string(ingredientsJSON), string(instructionsJSON)) + "\n\n" + standardizeSchemaHint

	body, _ := json.Marshal(map[string]any{
		"model":      a.model,
		"max_tokens": 16000,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return StandardizeResponse{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return StandardizeResponse{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return StandardizeResponse{}, fmt.Errorf("anthropic read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return StandardizeResponse{}, fmt.Errorf("anthropic: status %d: %.300s", resp.StatusCode, raw)
	}

	var msg struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return StandardizeResponse{}, fmt.Errorf("anthropic: decode: %w", err)
	}
	var text strings.Builder
	for _, b := range msg.Content {
		if b.Type == "text" {
			text.WriteString(b.Text)
		}
	}
	return parseStandardized(text.String(), req)
}
