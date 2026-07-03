package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/scaling"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/components"
	"github.com/craicoverflow/beili/internal/templates/layout"
	"github.com/craicoverflow/beili/internal/templates/meals"
)

const defaultPageSize = 24

// MealsHandler handles all meal-related HTTP routes.
type MealsHandler struct {
	store    *store.MealStore
	cfg      config.Config
	aiProvider ai.Provider // nil when no AI key is configured
}

// NewMealsHandler creates a new MealsHandler.
func NewMealsHandler(s *store.MealStore, cfg config.Config, aiProvider ai.Provider) *MealsHandler {
	return &MealsHandler{store: s, cfg: cfg, aiProvider: aiProvider}
}

// HandleList renders the meal list page with infinite-scroll pagination.
func (h *MealsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	minRating, _ := strconv.Atoi(r.URL.Query().Get("min_rating"))
	filters := store.ListFilters{
		Query:     r.URL.Query().Get("q"),
		Category:  r.URL.Query().Get("category"),
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
		if r.Header.Get("HX-Target") == "meal-grid" {
			// Filter/search chip swap — return grid plus an OOB refresh of the
			// filter bar so the active-chip state stays in sync with the URL.
			if err := meals.FilterBarOOB(filters, h.cfg.BasePath).Render(r.Context(), w); err != nil {
				slog.Error("render filter bar oob", "err", err)
			}
			if err := components.MealGrid(mealList, nextURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
				slog.Error("render meal grid", "err", err)
			}
			return
		}
		// Nav-link swap into #main-content — return full page content without outer layout
		if err := meals.List(mealList, nextURL, filters, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render meals list htmx", "err", err)
		}
		return
	}

	page := meals.List(mealList, nextURL, filters, h.cfg.BasePath)
	if err := layout.Base("Meals", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
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
	if f.Query != "" {
		params.Set("q", f.Query)
	}
	if f.Category != "" {
		params.Set("category", f.Category)
	}
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
	if r.Header.Get("HX-Request") == "true" {
		if err := page.Render(r.Context(), w); err != nil {
			slog.Error("render new meal form", "err", err)
		}
		return
	}
	if err := layout.Base("Add Meal", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
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
		if err := layout.Base("Add Meal", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
			slog.Error("render form with errors", "err", err)
		}
		return
	}

	// Reject duplicate imports: check all source URLs against existing records.
	for _, src := range sources {
		if src.URL == "" {
			continue
		}
		existing, err := h.store.GetBySourceURL(r.Context(), src.URL)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			respondError(w, r, http.StatusInternalServerError, "failed to check for duplicates", "err", err)
			return
		}
		if existing != nil {
			validationErrs["duplicate_url"] = existing.ID
			page := meals.Form(&meal, sources, h.cfg.BasePath, validationErrs)
			w.WriteHeader(http.StatusUnprocessableEntity)
			if err := layout.Base("Add Meal", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
				slog.Error("render form with errors", "err", err)
			}
			return
		}
	}

	h.normalizeServings(r.Context(), &meal)

	if err := h.store.Create(r.Context(), &meal, sources); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to save meal", "err", err)
		return
	}

	http.Redirect(w, r, h.cfg.BasePath+"/meals/"+meal.ID, http.StatusSeeOther)
}

// userIDFromRequest returns the HA user ID from context, or "local" in dev mode.
func userIDFromRequest(r *http.Request) string {
	if u := auth.UserFromContext(r.Context()); u != nil {
		return u.ID
	}
	return "local"
}

// HandleDetail renders the read-only meal view.
func (h *MealsHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meal, err := h.store.GetByID(r.Context(), id, userIDFromRequest(r))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(w, r, h.cfg.BasePath)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return
	}

	page := meals.Detail(meal, h.cfg.BasePath, h.cfg.ShoppingPushEnabled(), h.cfg.IsHA)
	if r.Header.Get("HX-Request") == "true" {
		if err := page.Render(r.Context(), w); err != nil {
			slog.Error("render meal detail", "err", err)
		}
		return
	}
	if err := layout.Base(meal.Name, h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render meal detail", "err", err)
	}
}

// HandleScale re-renders the servings control plus scaled ingredient and
// instruction lists for the requested serving count. Scaling is deterministic
// (see internal/scaling) and always computed from the stored base quantities,
// so repeated adjustments never compound rounding errors.
func (h *MealsHandler) HandleScale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meal, err := h.store.GetByID(r.Context(), id, userIDFromRequest(r))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(w, r, h.cfg.BasePath)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return
	}
	if meal.Servings == nil || *meal.Servings < 1 {
		respondError(w, r, http.StatusBadRequest, "meal has no serving size to scale from", "id", id)
		return
	}

	servings, _ := strconv.Atoi(r.URL.Query().Get("servings"))
	if servings < 1 {
		servings = 1
	}
	if servings > 100 {
		servings = 100
	}

	ingredients := meal.Ingredients
	instructions := meal.Instructions
	if servings != *meal.Servings {
		ratio := float64(servings) / float64(*meal.Servings)
		ingredients = scaling.ScaleIngredients(meal.Ingredients, ratio)
		instructions = scaling.ScaleInstructions(meal.Instructions, ratio)
	}

	ctx := r.Context()
	if err := meals.ServingsControl(meal, servings, h.cfg.BasePath).Render(ctx, w); err != nil {
		slog.Error("render servings control", "err", err)
		return
	}
	if len(ingredients) > 0 {
		if err := meals.IngredientsSection(meal, ingredients, h.cfg.ShoppingPushEnabled(), h.cfg.BasePath, true).Render(ctx, w); err != nil {
			slog.Error("render scaled ingredients", "err", err)
			return
		}
	}
	if len(instructions) > 0 {
		if err := meals.InstructionsSection(instructions, true).Render(ctx, w); err != nil {
			slog.Error("render scaled instructions", "err", err)
		}
	}
}

// HandleEdit renders the pre-populated edit form.
func (h *MealsHandler) HandleEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meal, err := h.store.GetByID(r.Context(), id, "")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(w, r, h.cfg.BasePath)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return
	}

	slog.Info("HandleEdit loaded meal",
		"id", meal.ID,
		"ingredients_count", len(meal.Ingredients),
		"instructions_count", len(meal.Instructions),
		"sources_count", len(meal.Sources),
	)

	page := meals.Form(meal, meal.Sources, h.cfg.BasePath, nil)
	if r.Header.Get("HX-Request") == "true" {
		if err := page.Render(r.Context(), w); err != nil {
			slog.Error("render edit form", "err", err)
		}
		return
	}
	if err := layout.Base("Edit — "+meal.Name, h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
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

	slog.Info("HandleUpdate form data",
		"id", id,
		"ingredients_count", len(r.Form["ingredients"]),
		"instructions_count", len(r.Form["instructions"]),
		"ingredients", r.Form["ingredients"],
		"instructions", r.Form["instructions"],
	)

	meal, sources, validationErrs := parseMealForm(r)
	if len(validationErrs) > 0 {
		meal.ID = id
		page := meals.Form(&meal, sources, h.cfg.BasePath, validationErrs)
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := layout.Base("Edit Meal", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
			slog.Error("render form with errors", "err", err)
		}
		return
	}

	meal.ID = id
	h.normalizeServings(r.Context(), &meal)

	if err := h.store.Update(r.Context(), &meal, sources); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(w, r, h.cfg.BasePath)
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

// HandleRating updates the current user's rating for a meal and returns the updated widget.
func (h *MealsHandler) HandleRating(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}
	rating, _ := strconv.Atoi(r.FormValue("rating"))
	if rating < 0 || rating > 5 {
		respondError(w, r, http.StatusBadRequest, "rating must be 0–5")
		return
	}
	userID := userIDFromRequest(r)
	if err := h.store.UpsertUserRating(r.Context(), id, userID, rating); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to update rating", "id", id, "err", err)
		return
	}
	meal, err := h.store.GetByID(r.Context(), id, userID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load rating", "id", id, "err", err)
		return
	}
	if err := components.StarRatingInline(id, meal.UserRating, meal.AverageRating, meal.RatingCount, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render star rating inline", "err", err)
	}
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
	if err := components.SourceRow(idx, src, h.cfg.BasePath).Render(r.Context(), w); err != nil {
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

	if err := components.SourceTypeFields(idx, src, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render source type fields", "err", err)
	}
}

// nonMetricRe spots imperial units, fractions and unicode vulgar fractions —
// quantities the deterministic display scaler handles less gracefully than
// clean metric values, so they're worth an AI normalisation pass.
var nonMetricRe = regexp.MustCompile(`(?i)\b(tsp|tbsp|teaspoons?|tablespoons?|cups?|oz|ounces?|lbs?|pounds?|pints?|quarts?|gallons?)\b|[¼½¾⅓⅔⅕⅖⅗⅘⅙⅚⅛⅜⅝⅞]|\d\s*/\s*\d`)

func hasNonMetricQuantities(lines []string) bool {
	for _, l := range lines {
		if nonMetricRe.MatchString(l) {
			return true
		}
	}
	return false
}

// normalizeServings calls the AI provider to scale ingredients and instructions
// to the configured base serving size and convert quantities to metric. It
// mutates the meal in place. On any error it logs a warning and leaves the
// meal unchanged — in particular meal.Servings keeps its original value so the
// stored serving count always matches the stored quantities.
func (h *MealsHandler) normalizeServings(ctx context.Context, meal *models.Meal) {
	if h.aiProvider == nil || len(meal.Ingredients) == 0 {
		return
	}
	fromServings := 0
	if meal.Servings != nil {
		fromServings = *meal.Servings
	}
	needsScaling := fromServings > 0 && fromServings != h.cfg.BaseServings
	if !needsScaling && !hasNonMetricQuantities(meal.Ingredients) {
		return // already at target servings and already metric, skip AI call
	}

	req := ai.NormalizeRequest{
		Ingredients:  meal.Ingredients,
		Instructions: meal.Instructions,
		FromServings: fromServings,
		ToServings:   h.cfg.BaseServings,
	}

	var resp ai.NormalizeResponse
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		aiCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		resp, err = h.aiProvider.NormalizeRecipe(aiCtx, req)
		cancel()
		if err == nil {
			break
		}
		slog.Warn("ai normalisation attempt failed", "attempt", attempt, "err", err)
	}
	if err != nil {
		slog.Warn("ai normalisation failed, saving original values", "err", err)
		return
	}

	meal.Ingredients = resp.Ingredients
	meal.Instructions = resp.Instructions
	if needsScaling {
		base := h.cfg.BaseServings
		meal.Servings = &base
	}
	// when the serving count was unknown (fromServings == 0) the quantities
	// were only converted to metric, so the count deliberately stays unset
}

// --- form parsing ---

func parseMealForm(r *http.Request) (models.Meal, []models.Source, map[string]string) {
	errs := map[string]string{}

	category := models.Category(strings.TrimSpace(r.FormValue("category")))
	if category == "" {
		category = models.CategoryMain
	}

	meal := models.Meal{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Category:    category,
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
