package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
	"github.com/craicoverflow/beili/internal/templates/layout"
	tmplmeals "github.com/craicoverflow/beili/internal/templates/meals"
)

// ExportHandler handles meal export and import.
type ExportHandler struct {
	store *store.MealStore
	cfg   config.Config
}

// NewExportHandler creates an ExportHandler.
func NewExportHandler(s *store.MealStore, cfg config.Config) *ExportHandler {
	return &ExportHandler{store: s, cfg: cfg}
}

// --- export ---

type exportSource struct {
	Type          string `json:"type"`
	Title         string `json:"title,omitempty"`
	URL           string `json:"url,omitempty"`
	PageReference string `json:"page_reference,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

type exportMeal struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	MealTypes    []string       `json:"meal_types,omitempty"`
	Cuisine      string         `json:"cuisine,omitempty"`
	PrepTime     *int           `json:"prep_time,omitempty"`
	CookTime     *int           `json:"cook_time,omitempty"`
	Servings     *int           `json:"servings,omitempty"`
	Ingredients  []string       `json:"ingredients,omitempty"`
	Instructions []string       `json:"instructions,omitempty"`
	ImageURL     string         `json:"image_url,omitempty"`
	Rating       *int           `json:"rating,omitempty"`
	Notes        string         `json:"notes,omitempty"`
	Sources      []exportSource `json:"sources,omitempty"`
}

type exportData struct {
	Version    int          `json:"version"`
	ExportedAt time.Time    `json:"exported_at"`
	Meals      []exportMeal `json:"meals"`
}

// HandleExport streams all meals as a downloadable JSON file.
// GET /meals/export
func (h *ExportHandler) HandleExport(w http.ResponseWriter, r *http.Request) {
	mealList, err := h.store.List(r.Context(), store.ListFilters{})
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load meals", "err", err)
		return
	}

	exportMeals := make([]exportMeal, len(mealList))
	for i, m := range mealList {
		em := exportMeal{
			Name:         m.Name,
			Description:  m.Description,
			Cuisine:      m.Cuisine,
			PrepTime:     m.PrepTime,
			CookTime:     m.CookTime,
			Servings:     m.Servings,
			Ingredients:  []string(m.Ingredients),
			Instructions: []string(m.Instructions),
			ImageURL:     m.ImageURL,
			Rating:       m.Rating,
			Notes:        m.Notes,
		}
		for _, mt := range m.MealTypes {
			em.MealTypes = append(em.MealTypes, string(mt))
		}
		for _, src := range m.Sources {
			em.Sources = append(em.Sources, exportSource{
				Type:          string(src.Type),
				Title:         src.Title,
				URL:           src.URL,
				PageReference: src.PageReference,
				Notes:         src.Notes,
			})
		}
		exportMeals[i] = em
	}

	payload := exportData{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Meals:      exportMeals,
	}

	filename := fmt.Sprintf("meals-%s.json", time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		slog.Error("encode export", "err", err)
	}
}

// --- import ---

// HandleImportPage renders the import upload form.
// GET /meals/import
func (h *ExportHandler) HandleImportPage(w http.ResponseWriter, r *http.Request) {
	page := tmplmeals.ImportPage(nil, h.cfg.BasePath)
	if err := layout.Base("Import Meals", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render import page", "err", err)
	}
}

// HandleImport processes an uploaded JSON file.
// POST /meals/import
func (h *ExportHandler) HandleImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		respondError(w, r, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "no file uploaded")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "failed to read file")
		return
	}

	var payload exportData
	if err := json.Unmarshal(data, &payload); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON file")
		return
	}

	// Build a set of existing meal names for duplicate detection.
	existing, err := h.store.List(r.Context(), store.ListFilters{})
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "failed to load meals", "err", err)
		return
	}
	nameSet := make(map[string]bool, len(existing))
	for _, m := range existing {
		nameSet[strings.ToLower(m.Name)] = true
	}

	result := &models.ImportResult{}
	for _, em := range payload.Meals {
		if nameSet[strings.ToLower(em.Name)] {
			result.Skipped = append(result.Skipped, em.Name)
			continue
		}

		mealTypes := make(models.MealTypes, len(em.MealTypes))
		for i, mt := range em.MealTypes {
			mealTypes[i] = models.MealType(mt)
		}

		meal := &models.Meal{
			Name:         em.Name,
			Description:  em.Description,
			MealTypes:    mealTypes,
			Cuisine:      em.Cuisine,
			PrepTime:     em.PrepTime,
			CookTime:     em.CookTime,
			Servings:     em.Servings,
			Ingredients:  em.Ingredients,
			Instructions: em.Instructions,
			ImageURL:     em.ImageURL,
			Rating:       em.Rating,
			Notes:        em.Notes,
		}

		sources := make([]models.Source, len(em.Sources))
		for i, s := range em.Sources {
			sources[i] = models.Source{
				Type:          models.SourceType(s.Type),
				Title:         s.Title,
				URL:           s.URL,
				PageReference: s.PageReference,
				Notes:         s.Notes,
			}
		}

		if err := h.store.Create(r.Context(), meal, sources); err != nil {
			slog.Error("import meal", "name", em.Name, "err", err)
			result.Errors = append(result.Errors, em.Name)
			continue
		}
		result.Imported++
		// Register in nameSet so duplicate names within the same file are caught.
		nameSet[strings.ToLower(em.Name)] = true
	}

	page := tmplmeals.ImportPage(result, h.cfg.BasePath)
	if err := layout.Base("Import Meals", h.cfg.BasePath, auth.UserFromContext(r.Context()), h.cfg.ShoppingList, page).Render(r.Context(), w); err != nil {
		slog.Error("render import results", "err", err)
	}
}
