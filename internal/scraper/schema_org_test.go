package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// sampleRecipePage is a minimal HTML page with a valid schema.org/Recipe JSON-LD block.
const sampleRecipePage = `<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Classic Spaghetti Bolognese",
  "description": "A rich Italian meat sauce served over pasta.",
  "recipeCuisine": "Italian",
  "prepTime": "PT15M",
  "cookTime": "PT1H",
  "recipeYield": "4 servings",
  "recipeIngredient": [
    "400g spaghetti",
    "500g beef mince",
    "1 onion, finely chopped",
    "2 cloves garlic",
    "400g chopped tomatoes",
    "2 tbsp tomato paste",
    "1 tsp dried oregano"
  ]
}
</script>
</head>
<body><h1>Classic Spaghetti Bolognese</h1></body>
</html>`

// sampleGraphPage tests the @graph wrapper pattern used by some sites (e.g. WordPress with Yoast SEO).
const sampleGraphPage = `<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "WebPage",
      "name": "My Blog"
    },
    {
      "@type": "Recipe",
      "name": "Chicken Tikka Masala",
      "prepTime": "PT20M",
      "cookTime": "PT35M",
      "recipeYield": "4",
      "recipeIngredient": ["500g chicken", "400ml coconut milk", "2 tbsp tikka paste"],
      "recipeCuisine": "Indian"
    }
  ]
}
</script>
</head>
<body></body>
</html>`

func TestParseISO8601Duration(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"PT30M", 30},
		{"PT1H", 60},
		{"PT1H30M", 90},
		{"PT2H15M", 135},
		{"PT45S", 0}, // seconds ignored
		{"P1D", 0},   // days not handled (uncommon in recipes)
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := parseISO8601Duration(tt.input)
		if got != tt.want {
			t.Errorf("parseISO8601Duration(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseServings(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"4 servings", 4},
		{"serves 6", 6},
		{"4-6", 4},
		{"12", 12},
		{"Makes 8 cookies", 8},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseServings(tt.input)
		if got != tt.want {
			t.Errorf("parseServings(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestGoQueryScraper_StandardPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sampleRecipePage))
	}))
	defer srv.Close()

	s := NewSchemaOrgScraper()
	data, err := s.Scrape(context.Background(), srv.URL+"/recipe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Name != "Classic Spaghetti Bolognese" {
		t.Errorf("Name = %q, want %q", data.Name, "Classic Spaghetti Bolognese")
	}
	if data.Cuisine != "Italian" {
		t.Errorf("Cuisine = %q, want %q", data.Cuisine, "Italian")
	}
	if len(data.Ingredients) != 7 {
		t.Errorf("Ingredients count = %d, want 7", len(data.Ingredients))
	}
	if data.PrepTime == nil || *data.PrepTime != 15 {
		t.Errorf("PrepTime = %v, want 15", data.PrepTime)
	}
	if data.CookTime == nil || *data.CookTime != 60 {
		t.Errorf("CookTime = %v, want 60", data.CookTime)
	}
	if data.Servings == nil || *data.Servings != 4 {
		t.Errorf("Servings = %v, want 4", data.Servings)
	}
}

func TestGoQueryScraper_GraphPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sampleGraphPage))
	}))
	defer srv.Close()

	s := NewSchemaOrgScraper()
	data, err := s.Scrape(context.Background(), srv.URL+"/recipe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Name != "Chicken Tikka Masala" {
		t.Errorf("Name = %q, want %q", data.Name, "Chicken Tikka Masala")
	}
	if len(data.Ingredients) != 3 {
		t.Errorf("Ingredients count = %d, want 3", len(data.Ingredients))
	}
	if data.Servings == nil || *data.Servings != 4 {
		t.Errorf("Servings = %v, want 4", data.Servings)
	}
}

func TestGoQueryScraper_NoRecipe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><p>Just a regular page</p></body></html>`))
	}))
	defer srv.Close()

	s := NewSchemaOrgScraper()
	_, err := s.Scrape(context.Background(), srv.URL+"/page")
	if err != ErrNoRecipeFound {
		t.Errorf("expected ErrNoRecipeFound, got %v", err)
	}
}

func TestGoQueryScraper_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewSchemaOrgScraper()
	_, err := s.Scrape(context.Background(), srv.URL+"/missing")
	if err == nil {
		t.Error("expected error for 404 response, got nil")
	}
}
