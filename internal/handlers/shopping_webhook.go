package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/craicoverflow/beili/internal/config"
)

// ShoppingWebhookHandler forwards selected ingredients to an external webhook.
type ShoppingWebhookHandler struct {
	cfg config.Config
}

// NewShoppingWebhookHandler creates a ShoppingWebhookHandler.
func NewShoppingWebhookHandler(cfg config.Config) *ShoppingWebhookHandler {
	return &ShoppingWebhookHandler{cfg: cfg}
}

type webhookPayload struct {
	Items []string `json:"ingredients"`
}

// HandleAddToShoppingList POSTs selected ingredients to the configured webhook URL.
// POST /meals/{id}/add-to-shopping
func (h *ShoppingWebhookHandler) HandleAddToShoppingList(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ShoppingWebhookURL == "" {
		http.Error(w, "shopping webhook not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	items := r.Form["ingredient"]
	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	payload := webhookPayload{Items: items}
	body, err := json.Marshal(payload)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to encode payload", "err", err)
		return
	}

	resp, err := http.Post(h.cfg.ShoppingWebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("shopping webhook call failed", "err", err)
		respondError(w, r, http.StatusBadGateway, "webhook request failed", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		slog.Error("shopping webhook returned error", "status", resp.StatusCode)
		http.Error(w, fmt.Sprintf("webhook returned %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	// Return an HX-Trigger so the UI can show a toast/reset selection.
	w.Header().Set("HX-Trigger", `{"shoppingAdded": true}`)
	w.WriteHeader(http.StatusNoContent)
}
