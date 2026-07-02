package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/ourgroceries"
)

// ShoppingWebhookHandler adds selected ingredients to a shopping list. When
// OurGroceries is configured it pushes items there directly (with per-item note
// metadata); otherwise it falls back to the legacy Home Assistant webhook.
type ShoppingWebhookHandler struct {
	cfg config.Config
	og  *ourgroceries.Client // nil unless OurGroceries is configured
}

// NewShoppingWebhookHandler creates a ShoppingWebhookHandler. og may be nil.
func NewShoppingWebhookHandler(cfg config.Config, og *ourgroceries.Client) *ShoppingWebhookHandler {
	return &ShoppingWebhookHandler{cfg: cfg, og: og}
}

type webhookPayload struct {
	Items []string `json:"ingredients"`
}

// HandleAddToShoppingList adds the selected ingredients to the shopping list.
// POST /meals/{id}/add-to-shopping
func (h *ShoppingWebhookHandler) HandleAddToShoppingList(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.ShoppingPushEnabled() {
		http.Error(w, "no shopping list target configured", http.StatusServiceUnavailable)
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

	// Prefer the OurGroceries direct push: it carries each item's amount as a
	// note, which the legacy webhook (name-only) cannot.
	if h.og != nil && h.cfg.OurGroceriesConfigured() {
		if err := h.pushToOurGroceries(r, items); err != nil {
			slog.Error("ourgroceries push failed", "err", err)
			http.Error(w, "failed to add to shopping list", http.StatusBadGateway)
			return
		}
		h.signalAdded(w)
		return
	}

	if err := h.pushToWebhook(items); err != nil {
		slog.Error("shopping webhook call failed", "err", err)
		http.Error(w, "failed to add to shopping list", http.StatusBadGateway)
		return
	}
	h.signalAdded(w)
}

// pushToOurGroceries adds each ingredient to the configured list as a
// "Name (amount)" item, e.g. "Butter (35g)". The amount lives in the item name
// (not the note) so it's visible at a glance in the OurGroceries apps; the
// downstream Tesco basket builder parses it from the name.
func (h *ShoppingWebhookHandler) pushToOurGroceries(r *http.Request, raw []string) error {
	items := make([]ourgroceries.Item, 0, len(raw))
	for _, ing := range raw {
		items = append(items, ourgroceries.Item{Value: formatShoppingItem(ing)})
	}
	return h.og.AddItems(r.Context(), h.cfg.OurGroceriesListID, items)
}

// pushToWebhook POSTs name-only "Name (amount)" strings to the legacy HA webhook.
func (h *ShoppingWebhookHandler) pushToWebhook(raw []string) error {
	items := make([]string, len(raw))
	for i, item := range raw {
		items[i] = formatShoppingItem(item)
	}

	body, err := json.Marshal(webhookPayload{Items: items})
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	resp, err := http.Post(h.cfg.ShoppingWebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

// signalAdded returns an HX-Trigger so the UI can show a toast / reset selection.
func (h *ShoppingWebhookHandler) signalAdded(w http.ResponseWriter) {
	w.Header().Set("HX-Trigger", `{"shoppingAdded": true}`)
	w.WriteHeader(http.StatusNoContent)
}
