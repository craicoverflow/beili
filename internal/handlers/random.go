package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/layout"
	"github.com/craicoverflow/beili/internal/templates/meals"
)

// RandomHandler picks a random meal.
type RandomHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewRandomHandler creates a RandomHandler.
func NewRandomHandler(s *store.MealStore, cfg config.Config) *RandomHandler {
	return &RandomHandler{store: s, cfg: cfg}
}

// HandleRandom picks a random meal matching optional query-param filters and
// renders the detail view with a "Try another" button.
// GET /meals/random?meal_type=dinner&min_rating=4
func (h *RandomHandler) HandleRandom(w http.ResponseWriter, r *http.Request) {
	minRating, _ := strconv.Atoi(r.URL.Query().Get("min_rating"))
	filters := store.ListFilters{
		MealType:  r.URL.Query().Get("meal_type"),
		MinRating: minRating,
	}

	meal, err := h.store.Random(r.Context(), filters)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, r, http.StatusNotFound, "No meals found matching your filters")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to pick a random meal", "err", err)
		return
	}

	page := meals.RandomMealPage(meal, h.cfg.BasePath)

	if r.Header.Get("HX-Request") == "true" {
		if err := page.Render(r.Context(), w); err != nil {
			slog.Error("render random meal partial", "err", err)
		}
		return
	}

	if err := layout.Base("Surprise Me", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render random meal page", "err", err)
	}
}
