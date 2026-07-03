package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/layout"
	tmplplan "github.com/craicoverflow/beili/internal/templates/plan"
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

// HandleWeek renders the meal plan.
// Desktop view: ISO week (Mon–Sun) containing the anchor.
// Mobile view: rolling 4-day window starting at the anchor.
// GET /plan?start=2025-04-14   (preferred — anchors mobile rolling view)
// GET /plan?week=2025-W15      (legacy — anchored to that week's Monday)
// GET /plan                    (defaults to today)
func (h *PlanHandler) HandleWeek(w http.ResponseWriter, r *http.Request) {
	anchor := parseAnchor(r.URL.Query().Get("start"), r.URL.Query().Get("week"))
	mobileStart := startOfDay(anchor)
	weekStart := mondayOf(mobileStart)

	rangeStart := minDate(weekStart, mobileStart)
	rangeEnd := maxDate(weekStart.AddDate(0, 0, 7), mobileStart.AddDate(0, 0, 4))

	entries, err := h.planStore.GetRange(r.Context(), rangeStart, rangeEnd)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load plan", "err", err)
		return
	}

	data := tmplplan.WeekData{
		WeekStart:   weekStart,
		MobileStart: mobileStart,
		Entries:     indexEntries(entries),
		BasePath:    h.cfg.BasePath,
	}

	calendarComponent := tmplplan.Week(data)

	if r.Header.Get("HX-Request") == "true" {
		if err := calendarComponent.Render(r.Context(), w); err != nil {
			slog.Error("render week partial", "err", err)
		}
		return
	}

	if r.URL.Query().Get("embed") == "true" {
		if err := layout.BaseEmbed("Meal Plan", h.cfg.BasePath, calendarComponent).Render(r.Context(), w); err != nil {
			slog.Error("render week embed", "err", err)
		}
		return
	}

	if err := layout.Base("Meal Plan", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, calendarComponent).Render(r.Context(), w); err != nil {
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

// HandleAddToPlanModal returns the modal for adding a specific meal to the plan.
// GET /plan/add-to-plan?meal_id=...
func (h *PlanHandler) HandleAddToPlanModal(w http.ResponseWriter, r *http.Request) {
	mealID := r.URL.Query().Get("meal_id")
	if mealID == "" {
		respondError(w, r, http.StatusBadRequest, "meal_id is required")
		return
	}

	meal, err := h.mealStore.GetByID(r.Context(), mealID, "")
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "err", err)
		return
	}

	servings := 4
	if meal.Servings != nil {
		servings = *meal.Servings
	}
	// The detail page passes the currently-scaled serving count (via the
	// servings-control's data-current attribute) so the modal opens prefilled
	// to what's on screen, not the meal's unscaled base.
	if q := r.URL.Query().Get("servings"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			servings = v
		}
	}

	d := tmplplan.AddToPlanModalData{
		MealID:   mealID,
		MealName: meal.Name,
		Today:    time.Now().Format("2006-01-02"),
		Servings: servings,
		BasePath: h.cfg.BasePath,
	}

	if err := tmplplan.AddToPlanModal(d).Render(r.Context(), w); err != nil {
		slog.Error("render add-to-plan modal", "err", err)
	}
}

// HandleAssign creates or replaces one or more meal plan entries.
// POST /plan  (body: date, meal_type, meal_id, extra_date (repeated, optional))
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

	extraDays, _ := strconv.Atoi(r.FormValue("extra_days"))
	if extraDays < 0 || extraDays > 2 {
		extraDays = 0
	}

	var servings *int
	if sv := r.FormValue("servings"); sv != "" {
		if v, err := strconv.Atoi(sv); err == nil && v > 0 {
			servings = &v
		}
	}
	// Only used by the calendar's remove-undo restore, where the primary
	// entry itself needs to come back as a leftover; the normal add-to-plan
	// flow leaves this unset and relies on the extra_days i>0 rule below.
	primaryIsLeftover := r.FormValue("is_leftover") == "true"

	primaryDate, parseErr := time.Parse("2006-01-02", date)
	if parseErr != nil {
		respondError(w, r, http.StatusBadRequest, "invalid date format")
		return
	}
	allDates := make([]string, 1+extraDays)
	allDates[0] = date
	for i := 1; i <= extraDays; i++ {
		allDates[i] = primaryDate.AddDate(0, 0, i).Format("2006-01-02")
	}

	entryIDs := make([]string, len(allDates))
	for i, d := range allDates {
		entry := &models.MealPlanEntry{
			Date:     d,
			MealType: mealType,
			MealID:   &mealID,
			Servings: servings,
			// Extra days beyond the primary date are leftovers of the same
			// cooked batch — they need no new shopping ingredients.
			IsLeftover: i > 0 || primaryIsLeftover,
		}
		if err := h.planStore.SetEntry(r.Context(), entry); err != nil {
			respondError(w, r, http.StatusInternalServerError, "failed to save", "date", d, "err", err)
			return
		}
		entryIDs[i] = entry.ID
	}

	meal, err := h.mealStore.GetByID(r.Context(), mealID, "")
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "assigned but could not reload", "meal_id", mealID, "err", err)
		return
	}

	if len(allDates) == 1 {
		entry := &models.MealPlanEntry{ID: entryIDs[0], Date: date, MealType: mealType, Meal: meal, MealID: &mealID, Servings: servings, IsLeftover: primaryIsLeftover}
		cellData := tmplplan.DayCellData{Date: date, MealType: mealType, Entry: entry, BasePath: h.cfg.BasePath}
		if err := tmplplan.DayCell(cellData).Render(r.Context(), w); err != nil {
			slog.Error("render day cell after assign", "err", err)
		}
		return
	}

	cells := make([]tmplplan.DayCellData, len(allDates))
	for i, d := range allDates {
		entry := &models.MealPlanEntry{ID: entryIDs[i], Date: d, MealType: mealType, Meal: meal, MealID: &mealID, Servings: servings, IsLeftover: i > 0}
		cells[i] = tmplplan.DayCellData{Date: d, MealType: mealType, Entry: entry, BasePath: h.cfg.BasePath}
	}
	if err := tmplplan.MultiDayCellResponse(cells).Render(r.Context(), w); err != nil {
		slog.Error("render multi day cells after assign", "err", err)
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

// parseAnchor resolves the date anchor from the query params, in priority order:
// ?start=YYYY-MM-DD, then legacy ?week=YYYY-Www, then today.
func parseAnchor(start, week string) time.Time {
	if start != "" {
		if t, err := time.ParseInLocation("2006-01-02", start, time.Local); err == nil {
			return t
		}
	}
	if week != "" {
		if t, err := parseWeekParam(week); err == nil {
			return t
		}
	}
	return time.Now()
}

// mondayOf returns the Monday of the ISO week containing t (00:00 local).
func mondayOf(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	d := t.AddDate(0, 0, -(wd - 1))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func minDate(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxDate(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

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
	// ISO weekday: Mon=1 ... Sat=6, Sun=7 (Go's Sunday=0 must become 7)
	isoWd := int(jan4.Weekday())
	if isoWd == 0 {
		isoWd = 7
	}
	monday := jan4.AddDate(0, 0, -(isoWd - 1))
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
