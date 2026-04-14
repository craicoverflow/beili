package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/craicoverflow/my-recipe-manager/internal/config"
	"github.com/craicoverflow/my-recipe-manager/internal/store"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/components"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/layout"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/meals"
)

// SearchHandler handles meal search and filtering.
type SearchHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewSearchHandler creates a SearchHandler.
func NewSearchHandler(s *store.MealStore, cfg config.Config) *SearchHandler {
	return &SearchHandler{store: s, cfg: cfg}
}

// HandleSearch responds to GET /search?q=... — returns the meal grid partial
// for HTMX requests, or a full page for direct navigation.
func (h *SearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	mealType := r.URL.Query().Get("meal_type")
	minRating, _ := strconv.Atoi(r.URL.Query().Get("min_rating"))

	filters := store.ListFilters{
		Query:     q,
		MealType:  mealType,
		MinRating: minRating,
	}

	mealList, err := h.store.List(r.Context(), filters)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "search failed", "q", q, "err", err)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		// Return only the grid — HTMX swaps it into #meal-grid
		if err := components.MealGrid(mealList, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render search grid", "err", err)
		}
		return
	}

	// Direct navigation: full page with the search term pre-filled in the header
	page := meals.SearchResults(mealList, q, h.cfg.BasePath)
	if err := layout.Base("Search", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render search page", "err", err)
	}
}
