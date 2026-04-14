package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/my-recipe-manager/internal/config"
	"github.com/craicoverflow/my-recipe-manager/internal/models"
	"github.com/craicoverflow/my-recipe-manager/internal/store"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/layout"
	tmplplan "github.com/craicoverflow/my-recipe-manager/internal/templates/plan"
)

// PlanHandler handles all meal-plan calendar routes.
type PlanHandler struct {
	planStore *store.PlanStore
	mealStore *store.MealStore
	cfg       config.Config
}

// NewPlanHandler creates a PlanHandler.
func NewPlanHandler(ps *store.PlanStore, ms *store.MealStore, cfg config.Config) *PlanHandler {
	return &PlanHandler{planStore: ps, mealStore: ms, cfg: cfg}
}

// HandleWeek renders the weekly calendar.
// GET /plan?week=2025-W15  (defaults to current week)
func (h *PlanHandler) HandleWeek(w http.ResponseWriter, r *http.Request) {
	weekStart, err := parseWeekParam(r.URL.Query().Get("week"))
	if err != nil {
		weekStart = currentWeekStart()
	}

	entries, err := h.planStore.GetWeek(r.Context(), weekStart)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load plan", "err", err)
		return
	}

	data := tmplplan.WeekData{
		WeekStart: weekStart,
		Entries:   indexEntries(entries),
		BasePath:  h.cfg.BasePath,
	}

	calendarComponent := tmplplan.Week(data)

	if r.Header.Get("HX-Request") == "true" {
		if err := calendarComponent.Render(r.Context(), w); err != nil {
			slog.Error("render week partial", "err", err)
		}
		return
	}

	if err := layout.Base("Meal Plan", h.cfg.BasePath, calendarComponent).Render(r.Context(), w); err != nil {
		slog.Error("render week page", "err", err)
	}
}

// HandleAssignModal returns the assign-meal modal partial.
// GET /plan/assign?date=2025-04-14&meal_type=dinner&q=optional_search
func (h *PlanHandler) HandleAssignModal(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	mealType := models.MealType(r.URL.Query().Get("meal_type"))
	q := r.URL.Query().Get("q")

	filters := store.ListFilters{Query: q}
	mealList, err := h.mealStore.List(r.Context(), filters)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load meals", "err", err)
		return
	}

	d := tmplplan.AssignModalData{
		Date:     date,
		MealType: mealType,
		Meals:    mealList,
		BasePath: h.cfg.BasePath,
	}

	// If triggered by the modal search input (hx-target="#modal-meal-list"),
	// return only the list; otherwise return the full modal.
	if r.Header.Get("HX-Target") == "modal-meal-list" {
		if err := tmplplan.ModalMealList(d).Render(r.Context(), w); err != nil {
			slog.Error("render modal meal list", "err", err)
		}
		return
	}

	if err := tmplplan.AssignModal(d).Render(r.Context(), w); err != nil {
		slog.Error("render assign modal", "err", err)
	}
}

// HandleAssign creates or replaces a meal plan entry.
// POST /plan  (body: date, meal_type, meal_id)
func (h *PlanHandler) HandleAssign(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}

	date := r.FormValue("date")
	mealType := models.MealType(r.FormValue("meal_type"))
	mealID := r.FormValue("meal_id")

	if date == "" || mealType == "" || mealID == "" {
		respondError(w, r, http.StatusBadRequest, "date, meal_type, and meal_id are required")
		return
	}

	entry := &models.MealPlanEntry{
		Date:     date,
		MealType: mealType,
		MealID:   &mealID,
	}

	if err := h.planStore.SetEntry(r.Context(), entry); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to save", "err", err)
		return
	}

	// Reload the entry with joined meal data for the cell render
	weekStart, _ := parseWeekParam("")
	_ = weekStart
	meal, err := h.mealStore.GetByID(r.Context(), mealID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "assigned but could not reload", "meal_id", mealID, "err", err)
		return
	}
	entry.Meal = meal

	cellData := tmplplan.DayCellData{
		Date:     date,
		MealType: mealType,
		Entry:    entry,
		BasePath: h.cfg.BasePath,
	}
	if err := tmplplan.DayCell(cellData).Render(r.Context(), w); err != nil {
		slog.Error("render day cell after assign", "err", err)
	}
}

// HandleRemove deletes a plan entry and returns an empty cell.
// DELETE /plan/{id}?date=...&meal_type=...
func (h *PlanHandler) HandleRemove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	date := r.URL.Query().Get("date")
	mealType := models.MealType(r.URL.Query().Get("meal_type"))

	if err := h.planStore.RemoveEntry(r.Context(), id); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to remove", "id", id, "err", err)
		return
	}

	// Return an empty cell to replace the removed one
	cellData := tmplplan.DayCellData{
		Date:     date,
		MealType: mealType,
		Entry:    nil,
		BasePath: h.cfg.BasePath,
	}
	if err := tmplplan.DayCell(cellData).Render(r.Context(), w); err != nil {
		slog.Error("render empty cell after remove", "err", err)
	}
}

// --- helpers ---

// parseWeekParam converts "2025-W15" to the Monday of that ISO week.
// Returns current week on empty or invalid input.
func parseWeekParam(s string) (time.Time, error) {
	if s == "" {
		return currentWeekStart(), nil
	}
	var year, week int
	if _, err := fmt.Sscanf(s, "%d-W%d", &year, &week); err != nil {
		return time.Time{}, fmt.Errorf("invalid week %q", s)
	}
	return isoWeekStart(year, week), nil
}

// currentWeekStart returns the Monday of the current week (local time).
func currentWeekStart() time.Time {
	return isoWeekStart(time.Now().ISOWeek())
}

// isoWeekStart returns the Monday of the given ISO year + week.
func isoWeekStart(year, week int) time.Time {
	// Jan 4 is always in week 1
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	_, w := jan4.ISOWeek()
	// Offset to Monday of week 1
	monday := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
	// Advance to the target week
	return monday.AddDate(0, 0, (week-w)*7)
}

// indexEntries converts a flat slice of entries into a nested map
// date → mealType → entry, for easy lookup in the template.
func indexEntries(entries []models.MealPlanEntry) map[string]map[models.MealType]*models.MealPlanEntry {
	idx := make(map[string]map[models.MealType]*models.MealPlanEntry)
	for i := range entries {
		e := &entries[i]
		if idx[e.Date] == nil {
			idx[e.Date] = make(map[models.MealType]*models.MealPlanEntry)
		}
		idx[e.Date][e.MealType] = e
	}
	return idx
}
