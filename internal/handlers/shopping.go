package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/ourgroceries"
	"github.com/craicoverflow/beili/internal/scaling"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/layout"
	tmplplan "github.com/craicoverflow/beili/internal/templates/plan"
)

// ShoppingHandler serves the weekly shopping list.
type ShoppingHandler struct {
	planStore   *store.PlanStore
	mealStore   *store.MealStore
	extrasStore *store.ShoppingExtrasStore
	og          *ourgroceries.Client // nil unless OurGroceries is configured
	cfg         config.Config
}

// NewShoppingHandler creates a ShoppingHandler. og may be nil.
func NewShoppingHandler(ps *store.PlanStore, ms *store.MealStore, es *store.ShoppingExtrasStore, og *ourgroceries.Client, cfg config.Config) *ShoppingHandler {
	return &ShoppingHandler{planStore: ps, mealStore: ms, extrasStore: es, og: og, cfg: cfg}
}

// ourGroceriesEnabled reports whether the live OurGroceries mirror is available.
func (h *ShoppingHandler) ourGroceriesEnabled() bool {
	return h.og != nil && h.cfg.OurGroceriesConfigured()
}

// HandleList renders the shopping list. The default "Planned" view is derived
// from the week's meal plan; "?view=list" mirrors the live shared OurGroceries
// list (the single source of truth both shoppers edit), when configured.
// GET /shopping?week=2025-W15&view=planned|list
func (h *ShoppingHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("view") == "list" && h.ourGroceriesEnabled() {
		h.renderLiveList(w, r)
		return
	}

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

	extras, err := h.extrasStore.ListForWeek(r.Context(), isoWeekString(weekStart))
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load extras", "err", err)
		return
	}

	data := tmplplan.ShoppingListData{
		WeekStart:           weekStart,
		Groups:              groups,
		Extras:              extras,
		BasePath:            h.cfg.BasePath,
		OurGroceriesEnabled: h.ourGroceriesEnabled(),
	}

	page := tmplplan.ShoppingList(data)
	if err := layout.Base("Shopping List", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render shopping list", "err", err)
	}
}

// HandleAddExtra adds an ad-hoc shopping item and returns the updated extras list.
// POST /shopping/extras  (body: week, text)
func (h *ShoppingHandler) HandleAddExtra(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}
	week := r.FormValue("week")
	text := r.FormValue("text")
	if week == "" || text == "" {
		respondError(w, r, http.StatusBadRequest, "week and text are required")
		return
	}

	if _, err := h.extrasStore.Add(r.Context(), week, text); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to add item", "err", err)
		return
	}

	h.renderExtrasList(w, r, week)
}

// HandleToggleExtra flips the checked state of an extra item.
// POST /shopping/extras/{id}/toggle  (body: week, checked)
func (h *ShoppingHandler) HandleToggleExtra(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}
	week := r.FormValue("week")
	checked := r.FormValue("checked") == "true"

	if err := h.extrasStore.SetChecked(r.Context(), id, checked); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to update item", "err", err)
		return
	}

	h.renderExtrasList(w, r, week)
}

// HandleRemoveExtra deletes an ad-hoc shopping item.
// DELETE /shopping/extras/{id}?week=...
func (h *ShoppingHandler) HandleRemoveExtra(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	week := r.URL.Query().Get("week")

	if err := h.extrasStore.Remove(r.Context(), id); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to remove item", "err", err)
		return
	}

	h.renderExtrasList(w, r, week)
}

func (h *ShoppingHandler) renderExtrasList(w http.ResponseWriter, r *http.Request, week string) {
	extras, err := h.extrasStore.ListForWeek(r.Context(), week)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load extras", "err", err)
		return
	}
	if err := tmplplan.ExtrasList(week, extras, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render extras list", "err", err)
	}
}

// isoWeekString formats a time as its ISO week label, e.g. "2025-W15".
func isoWeekString(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// renderLiveList fetches and renders the live OurGroceries list mirror.
func (h *ShoppingHandler) renderLiveList(w http.ResponseWriter, r *http.Request) {
	items, err := h.og.GetList(r.Context(), h.cfg.OurGroceriesListID)
	if err != nil {
		respondError(w, r, http.StatusBadGateway, "failed to load OurGroceries list", "err", err)
		return
	}

	data := tmplplan.LiveListData{
		Items:    items,
		BasePath: h.cfg.BasePath,
	}

	page := tmplplan.LiveShoppingList(data)
	if err := layout.Base("Shopping List", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render live shopping list", "err", err)
	}
}

// buildShoppingGroups converts plan entries into deduplicated ShoppingGroups,
// fetching full meal data (including ingredients) for each unique meal ID.
// Leftover entries are skipped — they're extra portions of an already-counted
// batch and need no new ingredients. When an entry requests a serving count
// other than the meal's base, its ingredients are scaled deterministically
// (internal/scaling, never AI) before being merged into the group; the first
// non-default serving count seen for a meal wins if the week plans it twice
// at different sizes.
func (h *ShoppingHandler) buildShoppingGroups(r *http.Request, entries []models.MealPlanEntry) ([]tmplplan.ShoppingGroup, error) {
	// Collect unique meal IDs while preserving first-seen order, and track slots per meal.
	type mealMeta struct {
		slots    []string
		order    int
		servings int // 0 means "use meal base"
	}
	seen := make(map[string]*mealMeta)
	order := 0

	for _, e := range entries {
		if e.MealID == nil || e.IsLeftover {
			continue
		}
		id := *e.MealID
		slot := slotLabel(e.Date, e.MealType)
		if m, ok := seen[id]; ok {
			m.slots = append(m.slots, slot)
			if m.servings == 0 && e.Servings != nil {
				m.servings = *e.Servings
			}
		} else {
			meta := &mealMeta{slots: []string{slot}, order: order}
			if e.Servings != nil {
				meta.servings = *e.Servings
			}
			seen[id] = meta
			order++
		}
	}

	if len(seen) == 0 {
		return nil, nil
	}

	// Fetch full meals (with ingredients) in order.
	groups := make([]tmplplan.ShoppingGroup, len(seen))
	for id, meta := range seen {
		meal, err := h.mealStore.GetByID(r.Context(), id, "")
		if err != nil {
			return nil, fmt.Errorf("get meal %s: %w", id, err)
		}

		ingredients := meal.Ingredients
		displayServings := 0
		if meta.servings > 0 && meal.Servings != nil && meta.servings != *meal.Servings {
			ratio := float64(meta.servings) / float64(*meal.Servings)
			ingredients = scaling.ScaleIngredients(meal.Ingredients, ratio)
			displayServings = meta.servings
		}

		groups[meta.order] = tmplplan.ShoppingGroup{
			Meal:        meal,
			Slots:       meta.slots,
			Ingredients: ingredients,
			Servings:    displayServings,
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
