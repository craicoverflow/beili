package scraper

import (
	"context"
	"errors"

	"github.com/craicoverflow/beili/internal/models"
)

// ErrNoRecipeFound is returned when a URL is fetched successfully but no
// schema.org/Recipe data can be extracted from it.
var ErrNoRecipeFound = errors.New("no recipe schema found on page")

// RecipeData holds the structured data extracted from a recipe page.
// All fields are optional — only populate what was found.
type RecipeData struct {
	Name         string
	Description  string
	Ingredients  []string
	Instructions []string
	ImageURL     string
	PrepTime     *int // minutes
	CookTime     *int // minutes
	Servings     *int
	Cuisine      string
	// MealTypes is derived from schema.org recipeCategory (e.g. "Dinner",
	// "Dessert") via a best-effort keyword mapping — see categoryToMealTypes.
	MealTypes []models.MealType
	// VideoKind identifies the platform when the source URL is an embeddable
	// video (e.g. "youtube", "instagram"). Empty for non-video sources.
	VideoKind string
}

// IsVideo reports whether the source URL is an embeddable video.
func (r *RecipeData) IsVideo() bool { return r.VideoKind != "" }

// Scraper fetches and parses recipe data from a URL.
type Scraper interface {
	Scrape(ctx context.Context, url string) (*RecipeData, error)
}
