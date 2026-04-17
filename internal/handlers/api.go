package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/store"
)

// APIHandler handles JSON API endpoints for Home Assistant integration.
type APIHandler struct {
	planStore *store.PlanStore
	mealStore *store.MealStore
	cfg       config.Config
}

// NewAPIHandler creates an APIHandler.
func NewAPIHandler(ps *store.PlanStore, ms *store.MealStore, cfg config.Config) *APIHandler {
	return &APIHandler{planStore: ps, mealStore: ms, cfg: cfg}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

// HandlePlanWeek returns this week's meal plan as JSON.
// GET /api/plan/week?week=2025-W15  (defaults to current week)
func (h *APIHandler) HandlePlanWeek(w http.ResponseWriter, r *http.Request) {
	weekStart, err := parseWeekParam(r.URL.Query().Get("week"))
	if err != nil {
		weekStart = currentWeekStart()
	}

	entries, err := h.planStore.GetWeek(r.Context(), weekStart)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load plan"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"week_start": weekStart.Format("2006-01-02"),
		"entries":    entries,
	})
}

// HandleMeals returns a list of meals as JSON.
// GET /api/meals?q=&meal_type=&min_rating=&limit=&offset=
func (h *APIHandler) HandleMeals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := store.ListFilters{
		Query:    q.Get("q"),
		MealType: q.Get("meal_type"),
	}
	if v := q.Get("min_rating"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.MinRating = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.Offset = n
		}
	}

	if filters.Limit > 0 {
		meals, hasMore, err := h.mealStore.Page(r.Context(), filters)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load meals"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"meals": meals, "has_more": hasMore})
		return
	}

	meals, err := h.mealStore.List(r.Context(), filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load meals"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"meals": meals, "has_more": false})
}
