package scraper

import (
	"context"
	"errors"
)

// ErrNoRecipeFound is returned when a URL is fetched successfully but no
// schema.org/Recipe data can be extracted from it.
var ErrNoRecipeFound = errors.New("no recipe schema found on page")

// RecipeData holds the structured data extracted from a recipe page.
// All fields are optional — only populate what was found.
type RecipeData struct {
	Name        string
	Description string
	Ingredients []string
	PrepTime    *int // minutes
	CookTime    *int // minutes
	Servings    *int
	Cuisine     string
}

// Scraper fetches and parses recipe data from a URL.
type Scraper interface {
	Scrape(ctx context.Context, url string) (*RecipeData, error)
}
