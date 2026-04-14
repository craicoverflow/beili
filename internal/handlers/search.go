package handlers

import (
	"log/slog"
	"net/http"
	"net/url"
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
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	minRating, _ := strconv.Atoi(r.URL.Query().Get("min_rating"))

	filters := store.ListFilters{
		Query:     q,
		MinRating: minRating,
		Offset:    offset,
		Limit:     defaultPageSize,
	}

	mealList, hasMore, err := h.store.Page(r.Context(), filters)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "search failed", "q", q, "err", err)
		return
	}

	nextURL := searchNextURL(h.cfg.BasePath, q, filters, hasMore)

	if r.Header.Get("HX-Request") == "true" {
		if offset > 0 {
			if err := components.MealGridAppend(mealList, nextURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
				slog.Error("render search grid append", "err", err)
			}
			return
		}
		if err := components.MealGrid(mealList, nextURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render search grid", "err", err)
		}
		return
	}

	page := meals.SearchResults(mealList, q, nextURL, h.cfg.BasePath)
	if err := layout.Base("Search", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render search page", "err", err)
	}
}

func searchNextURL(basePath, q string, f store.ListFilters, hasMore bool) string {
	if !hasMore {
		return ""
	}
	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	if f.MinRating > 0 {
		params.Set("min_rating", strconv.Itoa(f.MinRating))
	}
	params.Set("offset", strconv.Itoa(f.Offset+f.Limit))
	return basePath + "/search?" + params.Encode()
}
