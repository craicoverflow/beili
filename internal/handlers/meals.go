package handlers

import (
	"log/slog"
	"net/http"

	"github.com/craicoverflow/my-recipe-manager/internal/config"
	"github.com/craicoverflow/my-recipe-manager/internal/store"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/components"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/layout"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/meals"
)

// MealsHandler handles all meal-related HTTP routes.
type MealsHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewMealsHandler creates a new MealsHandler.
func NewMealsHandler(s *store.MealStore, cfg config.Config) *MealsHandler {
	return &MealsHandler{store: s, cfg: cfg}
}

// HandleList renders the meal list page.
func (h *MealsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	filters := store.ListFilters{
		MealType: r.URL.Query().Get("meal_type"),
	}

	mealList, err := h.store.List(r.Context(), filters)
	if err != nil {
		slog.Error("list meals", "err", err)
		http.Error(w, "failed to load meals", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		// Return just the grid for HTMX filter/search swaps
		if err := components.MealGrid(mealList, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render meal grid", "err", err)
		}
		return
	}

	page := meals.List(mealList, h.cfg.BasePath)
	if err := layout.Base("Meals", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render meals list", "err", err)
	}
}
