package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/layout"
	tmplplan "github.com/craicoverflow/beili/internal/templates/plan"
)

// ShoppingHandler serves the weekly shopping list.
type ShoppingHandler struct {
	planStore *store.PlanStore
	mealStore *store.MealStore
	cfg       config.Config
}

// NewShoppingHandler creates a ShoppingHandler.
func NewShoppingHandler(ps *store.PlanStore, ms *store.MealStore, cfg config.Config) *ShoppingHandler {
	return &ShoppingHandler{planStore: ps, mealStore: ms, cfg: cfg}
}

// HandleList renders the shopping list for the requested (or current) week.
// GET /shopping?week=2025-W15
func (h *ShoppingHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	weekStart, err := parseWeekParam(r.URL.Query().Get("week"))
	if err != nil {
		weekStart = currentWeekStart()
	}

	entries, err := h.planStore.GetWeek(r.Context(), weekStart)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load plan", "err", err)
		return
	}

	groups, err := h.buildShoppingGroups(r, entries)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to build shopping list", "err", err)
		return
	}

	data := tmplplan.ShoppingListData{
		WeekStart: weekStart,
		Groups:    groups,
		BasePath:  h.cfg.BasePath,
	}

	page := tmplplan.ShoppingList(data)
	if err := layout.Base("Shopping List", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render shopping list", "err", err)
	}
}

// buildShoppingGroups converts plan entries into deduplicated ShoppingGroups,
// fetching full meal data (including ingredients) for each unique meal ID.
func (h *ShoppingHandler) buildShoppingGroups(r *http.Request, entries []models.MealPlanEntry) ([]tmplplan.ShoppingGroup, error) {
	// Collect unique meal IDs while preserving first-seen order, and track slots per meal.
	type mealMeta struct {
		slots []string
		order int
	}
	seen := make(map[string]*mealMeta)
	order := 0

	for _, e := range entries {
		if e.MealID == nil {
			continue
		}
		id := *e.MealID
		slot := slotLabel(e.Date, e.MealType)
		if m, ok := seen[id]; ok {
			m.slots = append(m.slots, slot)
		} else {
			seen[id] = &mealMeta{slots: []string{slot}, order: order}
			order++
		}
	}

	if len(seen) == 0 {
		return nil, nil
	}

	// Fetch full meals (with ingredients) in order.
	groups := make([]tmplplan.ShoppingGroup, len(seen))
	for id, meta := range seen {
		meal, err := h.mealStore.GetByID(r.Context(), id)
		if err != nil {
			return nil, fmt.Errorf("get meal %s: %w", id, err)
		}
		groups[meta.order] = tmplplan.ShoppingGroup{
			Meal:  meal,
			Slots: meta.slots,
		}
	}
	return groups, nil
}

// slotLabel formats a date + meal type into a human-readable label like "Mon · Dinner".
func slotLabel(date string, mealType models.MealType) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return string(mealType)
	}
	return t.Format("Mon") + " · " + capitalize(string(mealType))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
