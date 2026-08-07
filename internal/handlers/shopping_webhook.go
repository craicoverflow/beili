package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/ourgroceries"
	"github.com/craicoverflow/beili/internal/store"
)

// ShoppingWebhookHandler adds selected ingredients to a shopping list. When
// OurGroceries is configured it pushes items there directly (with per-item note
// metadata); otherwise it falls back to the legacy Home Assistant webhook.
type ShoppingWebhookHandler struct {
	cfg   config.Config
	og    *ourgroceries.Client // nil unless OurGroceries is configured
	meals *store.MealStore
}

// NewShoppingWebhookHandler creates a ShoppingWebhookHandler. og may be nil.
func NewShoppingWebhookHandler(cfg config.Config, og *ourgroceries.Client, meals *store.MealStore) *ShoppingWebhookHandler {
	return &ShoppingWebhookHandler{cfg: cfg, og: og, meals: meals}
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

	selected := r.Form["ingredient"]
	if len(selected) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	items := h.resolveItems(r, selected)

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

// resolveItems turns the submitted "index|raw ingredient" checkbox values
// into formatted "Name (amount)" strings, preferring the meal's AI-derived
// shopping name for each index and falling back to regex parsing of the raw
// string when no meal, index, or name is available (e.g. older meals saved
// before shopping names existed).
func (h *ShoppingWebhookHandler) resolveItems(r *http.Request, selected []string) []string {
	var shoppingNames []string
	if h.meals != nil {
		if meal, err := h.meals.GetByID(r.Context(), chi.URLParam(r, "id"), ""); err == nil {
			shoppingNames = meal.ShoppingNames
		} else {
			slog.Warn("shopping push: could not load meal for shopping names", "err", err)
		}
	}

	items := make([]string, 0, len(selected))
	for _, sel := range selected {
		idx, after, ok := strings.Cut(sel, "|")
		raw := sel
		var name string
		if ok {
			if i, err := strconv.Atoi(idx); err == nil {
				raw = after
				if i >= 0 && i < len(shoppingNames) {
					name = shoppingNames[i]
				}
			}
			// Atoi failed: not our "idx|raw" format (e.g. a legacy value that
			// happens to contain a pipe) -- keep raw = sel, whole and unsplit.
		}
		items = append(items, formatShoppingItem(raw, name))
	}
	return items
}

// pushToOurGroceries adds each already-formatted "Name (amount)" item to the
// configured list, e.g. "Butter (35g)".
func (h *ShoppingWebhookHandler) pushToOurGroceries(r *http.Request, items []string) error {
	ogItems := make([]ourgroceries.Item, 0, len(items))
	for _, item := range items {
		ogItems = append(ogItems, ourgroceries.Item{Value: item})
	}
	return h.og.AddItems(r.Context(), h.cfg.OurGroceriesListID, ogItems)
}

// pushToWebhook POSTs already-formatted "Name (amount)" strings to the legacy HA webhook.
func (h *ShoppingWebhookHandler) pushToWebhook(items []string) error {
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
