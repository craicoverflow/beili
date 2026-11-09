package handlers

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/revision"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/layout"
	"github.com/craicoverflow/beili/internal/templates/meals"
)

const (
	maxFeedbackLen = 2000
	maxFormBytes   = 1 << 20
)

// FeedbackHandler records cooking feedback on a meal and, when a reviser is
// configured, turns it into a reviewed, linked fork of the meal.
type FeedbackHandler struct {
	store   *store.MealStore
	reviser ai.Reviser // nil disables AI revision; notes can still be saved
	cfg     config.Config
}

// NewFeedbackHandler creates a FeedbackHandler. reviser may be nil.
func NewFeedbackHandler(s *store.MealStore, reviser ai.Reviser, cfg config.Config) *FeedbackHandler {
	return &FeedbackHandler{store: s, reviser: reviser, cfg: cfg}
}

// loadMeal fetches the meal for the {id} route param, writing the error
// response itself and returning false when it cannot.
func (h *FeedbackHandler) loadMeal(w http.ResponseWriter, r *http.Request) (*models.Meal, bool) {
	id := chi.URLParam(r, "id")
	meal, err := h.store.GetByID(r.Context(), id, "")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(w, r, h.cfg.BasePath)
			return nil, false
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return nil, false
	}
	return meal, true
}

func (h *FeedbackHandler) renderSection(w http.ResponseWriter, r *http.Request, mealID string, saved bool) {
	notes, err := h.store.ListFeedback(r.Context(), mealID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load feedback", "id", mealID, "err", err)
		return
	}
	forks, err := h.store.ListForks(r.Context(), mealID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load forks", "id", mealID, "err", err)
		return
	}
	data := meals.FeedbackData{MealID: mealID, Notes: notes, Forks: forks, CanRevise: h.reviser != nil, Saved: saved}
	if err := meals.FeedbackSection(data, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render feedback section", "err", err)
	}
}

// HandleSection renders the feedback box, past notes and forks. The meal page
// loads it with HTMX so it also works inside the random-pick view.
// GET /meals/{id}/feedback
func (h *FeedbackHandler) HandleSection(w http.ResponseWriter, r *http.Request) {
	meal, ok := h.loadMeal(w, r)
	if !ok {
		return
	}
	h.renderSection(w, r, meal.ID, false)
}

// HandleSubmit saves a feedback note and, for action=revise, asks the AI for a
// revised recipe and shows it for review. The note is saved first so it is
// never lost if the AI call fails.
// POST /meals/{id}/feedback
func (h *FeedbackHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}
	meal, ok := h.loadMeal(w, r)
	if !ok {
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" || len([]rune(body)) > maxFeedbackLen {
		respondError(w, r, http.StatusBadRequest, "feedback must be between 1 and 2000 characters")
		return
	}
	if _, err := h.store.AddFeedback(r.Context(), meal.ID, body); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to save feedback", "id", meal.ID, "err", err)
		return
	}

	if r.FormValue("action") != "revise" || h.reviser == nil {
		h.renderSection(w, r, meal.ID, true)
		return
	}

	data := meals.ReviseData{Meal: meal, Feedback: body}
	h.revise(r, meal, &data)
	h.renderPage(w, r, "Suggested revision", meals.RevisePage(data, h.cfg.BasePath))
}

// revise asks the AI for a revision and fills data with it and its diff.
func (h *FeedbackHandler) revise(r *http.Request, meal *models.Meal, data *meals.ReviseData) {
	lineage, err := h.store.LineageFeedback(r.Context(), meal.ID)
	if err != nil {
		slog.Error("load lineage feedback", "id", meal.ID, "err", err)
		data.Error = "Couldn't load the feedback history for this recipe."
		return
	}
	// The new note was just saved, so it is the last entry.
	notes := make([]string, len(lineage))
	for i, f := range lineage {
		notes[i] = f.Body
	}

	servings := h.cfg.BaseServings
	if meal.Servings != nil {
		servings = *meal.Servings
	}
	ingredients, instructions, reasons, err := ai.Revise(r.Context(), h.reviser, ai.ReviseRequest{
		Name:         meal.Name,
		Servings:     servings,
		Ingredients:  meal.Ingredients,
		Instructions: meal.Instructions,
		Feedback:     notes,
	})
	if err != nil {
		slog.Error("ai revision failed", "id", meal.ID, "err", err)
		data.Error = "The AI couldn't produce a revision this time. Your note was saved; you can try again."
		return
	}
	instructions = cleanLines(instructions)

	rs := make([]revision.Reason, len(reasons))
	for i, re := range reasons {
		rs[i] = revision.Reason{Ref: re.Ref, Text: re.Text}
	}
	data.Ingredients = ingredients
	data.Instructions = instructions
	data.IngredientChanges = revision.Ingredients(meal.Ingredients, ingredients, rs)
	data.InstructionChanges = revision.Instructions(meal.Instructions, instructions, rs)
}

// HandleFork saves the reviewed revision as a new meal linked to the original.
// The fork starts fresh: no ratings, cooked history or notes are copied.
// POST /meals/{id}/fork
func (h *FeedbackHandler) HandleFork(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "bad request")
		return
	}
	parent, ok := h.loadMeal(w, r)
	if !ok {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	ingredients := cleanLines(strings.Split(r.FormValue("ingredients"), "\n"))
	instructions := cleanLines(strings.Split(r.FormValue("instructions"), "\n"))
	if name == "" || len(ingredients) == 0 {
		respondError(w, r, http.StatusBadRequest, "a name and at least one ingredient are required")
		return
	}

	fork := &models.Meal{
		Name:         name,
		Description:  parent.Description,
		Category:     parent.Category,
		MealTypes:    parent.MealTypes,
		Cuisine:      parent.Cuisine,
		PrepTime:     parent.PrepTime,
		CookTime:     parent.CookTime,
		Servings:     parent.Servings,
		Ingredients:  ingredients,
		Instructions: instructions,
		ImageURL:     parent.ImageURL,
		Notes:        parent.Notes,
	}
	// Source IDs are cleared so the store inserts them as new rows.
	sources := make([]models.Source, len(parent.Sources))
	for i, src := range parent.Sources {
		sources[i] = models.Source{Type: src.Type, Title: src.Title, URL: src.URL, PageReference: src.PageReference, Notes: src.Notes}
	}

	if err := h.store.CreateFork(r.Context(), parent.ID, fork, sources); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to save revised meal", "id", parent.ID, "err", err)
		return
	}
	http.Redirect(w, r, h.cfg.BasePath+"/meals/"+fork.ID, http.StatusSeeOther)
}

// renderPage renders page as a partial for HTMX requests and inside the full
// layout otherwise.
func (h *FeedbackHandler) renderPage(w http.ResponseWriter, r *http.Request, title string, page interface {
	Render(ctx context.Context, w io.Writer) error
}) {
	var err error
	if r.Header.Get("HX-Request") == "true" {
		err = page.Render(r.Context(), w)
	} else {
		err = layout.Base(title, h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w)
	}
	if err != nil {
		slog.Error("render page", "title", title, "err", err)
	}
}

// cleanLines trims each line and drops blanks.
func cleanLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
