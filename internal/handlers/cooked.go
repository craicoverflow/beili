package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/components"
)

// CookedHandler records when a meal was cooked.
type CookedHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewCookedHandler creates a CookedHandler.
func NewCookedHandler(s *store.MealStore, cfg config.Config) *CookedHandler {
	return &CookedHandler{store: s, cfg: cfg}
}

// HandleMarkCooked records today as a cook date and returns the updated
// CookStatus partial so HTMX can swap it in-place on the detail page.
// POST /meals/{id}/cooked
func (h *CookedHandler) HandleMarkCooked(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify the meal exists.
	if _, err := h.store.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to find meal", "id", id, "err", err)
		return
	}

	if err := h.store.LogCooked(r.Context(), id); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to log", "id", id, "err", err)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	if err := components.CookStatus(id, &today, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render cook status", "err", err)
	}
}
