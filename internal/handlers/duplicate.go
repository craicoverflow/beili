package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
)

// DuplicateHandler handles meal duplication.
type DuplicateHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewDuplicateHandler creates a DuplicateHandler.
func NewDuplicateHandler(s *store.MealStore, cfg config.Config) *DuplicateHandler {
	return &DuplicateHandler{store: s, cfg: cfg}
}

// HandleDuplicate copies an existing meal (and its sources) then redirects to
// the edit form so the user can rename and adjust the copy before saving.
// POST /meals/{id}/duplicate
func (h *DuplicateHandler) HandleDuplicate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	original, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		respondError(w, r, http.StatusInternalServerError, "failed to load meal", "id", id, "err", err)
		return
	}

	// Build the copy — clear IDs/timestamps so Create assigns fresh ones.
	copy := &models.Meal{
		Name:        "Copy of " + original.Name,
		Description: original.Description,
		MealTypes:   original.MealTypes,
		Cuisine:     original.Cuisine,
		PrepTime:    original.PrepTime,
		CookTime:    original.CookTime,
		Servings:    original.Servings,
		Ingredients: append([]string(nil), original.Ingredients...),
		Rating:      original.Rating,
		Notes:       original.Notes,
	}

	// Strip source IDs/meal IDs so insertSource treats them as new rows.
	sources := make([]models.Source, len(original.Sources))
	for i, src := range original.Sources {
		sources[i] = models.Source{
			Type:          src.Type,
			Title:         src.Title,
			URL:           src.URL,
			PageReference: src.PageReference,
			Notes:         src.Notes,
		}
	}

	if err := h.store.Create(r.Context(), copy, sources); err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to duplicate meal", "err", err)
		return
	}

	http.Redirect(w, r, h.cfg.BasePath+"/meals/"+copy.ID+"/edit", http.StatusSeeOther)
}
