package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/my-recipe-manager/internal/config"
	"github.com/craicoverflow/my-recipe-manager/internal/models"
	"github.com/craicoverflow/my-recipe-manager/internal/store"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/components"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/layout"
	"github.com/craicoverflow/my-recipe-manager/internal/templates/meals"
)

const defaultPageSize = 24

// MealsHandler handles all meal-related HTTP routes.
type MealsHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewMealsHandler creates a new MealsHandler.
func NewMealsHandler(s *store.MealStore, cfg config.Config) *MealsHandler {
	return &MealsHandler{store: s, cfg: cfg}
}

// HandleList renders the meal list page with infinite-scroll pagination.
func (h *MealsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	minRating, _ := strconv.Atoi(r.URL.Query().Get("min_rating"))
	filters := store.ListFilters{
		MealType:  r.URL.Query().Get("meal_type"),
		MinRating: minRating,
		Offset:    offset,
		Limit:     defaultPageSize,
	}

	mealList, hasMore, err := h.store.Page(r.Context(), filters)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load meals", "err", err)
		return
	}

	nextURL := listNextURL(h.cfg.BasePath+"/meals", filters, hasMore)

	if r.Header.Get("HX-Request") == "true" {
		if offset > 0 {
			// Infinite scroll append — return only cards + optional sentinel
			if err := components.MealGridAppend(mealList, nextURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
				slog.Error("render meal grid append", "err", err)
			}
			return
		}
		if err := components.MealGrid(mealList, nextURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render meal grid", "err", err)
		}
		return
	}

	page := meals.List(mealList, nextURL, filters, h.cfg.BasePath)
	if err := layout.Base("Meals", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render meals list", "err", err)
	}
}

// listNextURL builds the next-page URL for infinite scroll, or returns "" if
// there are no more pages.
func listNextURL(base string, f store.ListFilters, hasMore bool) string {
	if !hasMore {
		return ""
	}
	params := url.Values{}
	if f.MealType != "" {
		params.Set("meal_type", f.MealType)
	}
	if f.MinRating > 0 {
		params.Set("min_rating", strconv.Itoa(f.MinRating))
	}
	params.Set("offset", strconv.Itoa(f.Offset+f.Limit))
	return base + "?" + params.Encode()
}

// HandleNew renders the empty create form.
func (h *MealsHandler) HandleNew(w http.ResponseWriter, r *http.Request) {
	page := meals.Form(nil, nil, h.cfg.BasePath, nil)
	if err := layout.Base("Add Meal", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render new meal form", "err", err)
	}
}

// HandleCreate processes the create form submission.
func (h *MealsHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}

	meal, sources, validationErrs := parseMealForm(r)
	if len(validationErrs) > 0 {
		page := meals.Form(&meal, sources, h.cfg.BasePath, validationErrs)
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := layout.Base("Add Meal", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
			slog.Error("render form with errors", "err", err)
		}
		return
	}

	if err := h.store.Create(r.Context(), &meal, sources); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to save meal", "err", err)
		return
	}

	http.Redirect(w, r, h.cfg.BasePath+"/meals/"+meal.ID, http.StatusSeeOther)
}

// HandleDetail renders the read-only meal view.
func (h *MealsHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meal, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return
	}

	page := meals.Detail(meal, h.cfg.BasePath)
	if err := layout.Base(meal.Name, h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render meal detail", "err", err)
	}
}

// HandleEdit renders the pre-populated edit form.
func (h *MealsHandler) HandleEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meal, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return
	}

	page := meals.Form(meal, meal.Sources, h.cfg.BasePath, nil)
	if err := layout.Base("Edit — "+meal.Name, h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
		slog.Error("render edit form", "err", err)
	}
}

// HandleUpdate processes the edit form submission.
func (h *MealsHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}

	meal, sources, validationErrs := parseMealForm(r)
	if len(validationErrs) > 0 {
		meal.ID = id
		page := meals.Form(&meal, sources, h.cfg.BasePath, validationErrs)
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := layout.Base("Edit Meal", h.cfg.BasePath, page).Render(r.Context(), w); err != nil {
			slog.Error("render form with errors", "err", err)
		}
		return
	}

	meal.ID = id
	if err := h.store.Update(r.Context(), &meal, sources); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to update meal", "id", id, "err", err)
		return
	}

	http.Redirect(w, r, h.cfg.BasePath+"/meals/"+id, http.StatusSeeOther)
}

// HandleDelete deletes a meal and redirects to the list.
func (h *MealsHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to delete meal", "id", id, "err", err)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", h.cfg.BasePath+"/meals")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, h.cfg.BasePath+"/meals", http.StatusSeeOther)
}

// HandleIngredientRow returns a single ingredient input row partial (HTMX target).
func (h *MealsHandler) HandleIngredientRow(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Query().Get("idx")
	idx, _ := strconv.Atoi(idxStr)

	// Render inline — simple enough to not need a separate templ file
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="flex items-center gap-2" id="ingredient-row-%d">
		<input type="text" name="ingredients" placeholder="Ingredient %d"
			class="ingredient-input flex-1 bg-surface-3 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-accent focus:border-accent" />
		<button type="button" class="text-zinc-600 hover:text-red-400 transition-colors shrink-0"
			onclick="document.getElementById('ingredient-row-%d').remove()" aria-label="Remove ingredient">
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
			</svg>
		</button>
	</div>`, idx, idx+1, idx)
}

// HandleInstructionRow returns a single instruction textarea row partial (HTMX target).
func (h *MealsHandler) HandleInstructionRow(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Query().Get("idx")
	idx, _ := strconv.Atoi(idxStr)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="flex items-start gap-2" id="instruction-row-%d">
		<span class="mt-2.5 text-xs font-medium text-zinc-500 w-6 text-right shrink-0">%d.</span>
		<textarea name="instructions" rows="2" placeholder="Step %d..."
			class="instruction-input flex-1 bg-surface-3 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-accent focus:border-accent resize-none"></textarea>
		<button type="button" class="mt-2 text-zinc-600 hover:text-red-400 transition-colors shrink-0"
			onclick="document.getElementById('instruction-row-%d').remove()" aria-label="Remove step">
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
			</svg>
		</button>
	</div>`, idx, idx+1, idx+1, idx)
}

// HandleSourceRow returns a blank source row partial (HTMX target).
func (h *MealsHandler) HandleSourceRow(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Query().Get("idx")
	idx, _ := strconv.Atoi(idxStr)

	src := models.Source{Type: models.SourceTypeURL}
	if err := components.SourceRow(idx, src).Render(r.Context(), w); err != nil {
		slog.Error("render source row", "err", err)
	}
}

// HandleSourceTypeFields returns the sub-fields for a given source type (HTMX swap).
func (h *MealsHandler) HandleSourceTypeFields(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Query().Get("idx")
	if idxStr == "" {
		idxStr = r.FormValue("idx")
	}
	idx, _ := strconv.Atoi(idxStr)

	srcType := models.SourceType(r.FormValue(fmt.Sprintf("source_type_%d", idx)))
	src := models.Source{Type: srcType}

	if err := components.SourceTypeFields(idx, src).Render(r.Context(), w); err != nil {
		slog.Error("render source type fields", "err", err)
	}
}

// --- form parsing ---

func parseMealForm(r *http.Request) (models.Meal, []models.Source, map[string]string) {
	errs := map[string]string{}

	meal := models.Meal{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Cuisine:     strings.TrimSpace(r.FormValue("cuisine")),
		Notes:       strings.TrimSpace(r.FormValue("notes")),
		ImageURL:    strings.TrimSpace(r.FormValue("image_url")),
	}

	if meal.Name == "" {
		errs["name"] = "Name is required"
	} else if len(meal.Name) > 200 {
		errs["name"] = "Name must be 200 characters or less"
	}

	// Meal types (multi-value checkbox)
	for _, mt := range r.Form["meal_types"] {
		meal.MealTypes = append(meal.MealTypes, models.MealType(mt))
	}

	// Numeric fields
	if v := r.FormValue("prep_time"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			errs["prep_time"] = "Must be a positive number"
		} else {
			meal.PrepTime = &n
		}
	}
	if v := r.FormValue("cook_time"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			errs["cook_time"] = "Must be a positive number"
		} else {
			meal.CookTime = &n
		}
	}
	if v := r.FormValue("servings"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs["servings"] = "Must be a positive number"
		} else {
			meal.Servings = &n
		}
	}

	// Rating
	if v := r.FormValue("rating"); v != "" && v != "0" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 5 {
			errs["rating"] = "Rating must be between 1 and 5"
		} else {
			meal.Rating = &n
		}
	}

	// Ingredients (multi-value, filter empties)
	for _, ing := range r.Form["ingredients"] {
		ing = strings.TrimSpace(ing)
		if ing != "" {
			meal.Ingredients = append(meal.Ingredients, ing)
		}
	}

	// Instructions (multi-value, filter empties)
	for _, step := range r.Form["instructions"] {
		step = strings.TrimSpace(step)
		if step != "" {
			meal.Instructions = append(meal.Instructions, step)
		}
	}

	// Sources: discover by scanning indexed form keys
	sources := parseSources(r)

	// Auto-add import_url as a URL source if provided and not already present
	if importURL := strings.TrimSpace(r.FormValue("import_url")); importURL != "" {
		already := false
		for _, s := range sources {
			if s.URL == importURL {
				already = true
				break
			}
		}
		if !already {
			sources = append([]models.Source{{Type: models.SourceTypeURL, URL: importURL}}, sources...)
		}
	}

	return meal, sources, errs
}

func parseSources(r *http.Request) []models.Source {
	var sources []models.Source
	// Find the highest source index present in the form
	maxIdx := -1
	for key := range r.Form {
		if strings.HasPrefix(key, "source_type_") {
			idxStr := strings.TrimPrefix(key, "source_type_")
			if n, err := strconv.Atoi(idxStr); err == nil && n > maxIdx {
				maxIdx = n
			}
		}
	}
	for i := 0; i <= maxIdx; i++ {
		srcType := models.SourceType(r.FormValue(fmt.Sprintf("source_type_%d", i)))
		if srcType == "" {
			continue
		}
		src := models.Source{
			Type:          srcType,
			Title:         strings.TrimSpace(r.FormValue(fmt.Sprintf("source_title_%d", i))),
			URL:           strings.TrimSpace(r.FormValue(fmt.Sprintf("source_url_%d", i))),
			PageReference: strings.TrimSpace(r.FormValue(fmt.Sprintf("source_page_%d", i))),
			Notes:         strings.TrimSpace(r.FormValue(fmt.Sprintf("source_notes_%d", i))),
		}
		// Skip completely empty sources
		if src.Title == "" && src.URL == "" && src.PageReference == "" && src.Notes == "" {
			continue
		}
		sources = append(sources, src)
	}
	return sources
}
