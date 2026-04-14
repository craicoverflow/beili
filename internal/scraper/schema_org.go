package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	gorecipe "github.com/kkyr/go-recipe/pkg/recipe"
)

// SchemaOrgScraper implements Scraper using go-recipe (primary) with a
// manual goquery JSON-LD fallback for sites go-recipe can't handle.
type SchemaOrgScraper struct {
	client *http.Client
}

// NewSchemaOrgScraper creates a scraper with a sensible HTTP client timeout.
func NewSchemaOrgScraper() *SchemaOrgScraper {
	return &SchemaOrgScraper{
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

// Scrape fetches the URL and attempts to extract schema.org/Recipe data.
// Returns ErrNoRecipeFound if the page exists but has no recipe schema.
func (s *SchemaOrgScraper) Scrape(ctx context.Context, rawURL string) (*RecipeData, error) {
	// --- Primary: go-recipe library ---
	result, err := s.scrapeWithGoRecipe(rawURL)
	if err == nil && result != nil {
		slog.Info("scraped recipe via go-recipe", "url", rawURL, "name", result.Name)
		return result, nil
	}
	if err != nil {
		slog.Debug("go-recipe failed, trying fallback", "url", rawURL, "err", err)
	}

	// --- Fallback: manual goquery JSON-LD extraction ---
	result, err = s.scrapeWithGoQuery(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrNoRecipeFound
	}

	slog.Info("scraped recipe via goquery fallback", "url", rawURL, "name", result.Name)
	return result, nil
}

// scrapeWithGoRecipe uses the go-recipe library which handles many sites
// natively and parses JSON-LD schema.org/Recipe data.
func (s *SchemaOrgScraper) scrapeWithGoRecipe(rawURL string) (*RecipeData, error) {
	r, err := gorecipe.ScrapeURL(rawURL)
	if err != nil {
		return nil, err
	}

	data := &RecipeData{}

	if name, ok := r.Name(); ok {
		data.Name = strings.TrimSpace(name)
	}
	if desc, ok := r.Description(); ok {
		data.Description = strings.TrimSpace(desc)
	}
	if ings, ok := r.Ingredients(); ok {
		data.Ingredients = cleanStringSlice(ings)
	}
	if cuisines, ok := r.Cuisine(); ok && len(cuisines) > 0 {
		data.Cuisine = cuisines[0]
	}
	if d, ok := r.PrepTime(); ok && d > 0 {
		mins := int(d.Minutes())
		data.PrepTime = &mins
	}
	if d, ok := r.CookTime(); ok && d > 0 {
		mins := int(d.Minutes())
		data.CookTime = &mins
	}
	if yields, ok := r.Yields(); ok {
		if n := parseServings(yields); n > 0 {
			data.Servings = &n
		}
	}
	if steps, ok := r.Instructions(); ok {
		data.Instructions = cleanStringSlice(steps)
	}
	if img, ok := r.ImageURL(); ok {
		data.ImageURL = strings.TrimSpace(img)
	}

	// Require at least a name or ingredients to consider this a success
	if data.Name == "" && len(data.Ingredients) == 0 {
		return nil, fmt.Errorf("go-recipe returned no usable data")
	}

	return data, nil
}

// scrapeWithGoQuery manually fetches the page and extracts
// <script type="application/ld+json"> blocks looking for @type: "Recipe".
func (s *SchemaOrgScraper) scrapeWithGoQuery(ctx context.Context, rawURL string) (*RecipeData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RecipeManager/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("page returned HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var found *RecipeData
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		raw := strings.TrimSpace(sel.Text())
		if raw == "" {
			return true
		}

		// Handle both a single object and an array of objects
		var candidates []map[string]any

		var single map[string]any
		if err := json.Unmarshal([]byte(raw), &single); err == nil {
			candidates = append(candidates, single)
		} else {
			var arr []map[string]any
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				candidates = append(candidates, arr...)
			}
		}

		for _, obj := range candidates {
			if r := extractRecipeFromLD(obj); r != nil {
				found = r
				return false // stop iteration
			}
		}
		return true
	})

	// If JSON-LD didn't provide an image, fall back to og:image / twitter:image meta tags.
	if found != nil && found.ImageURL == "" {
		if img := doc.Find(`meta[property="og:image"]`).AttrOr("content", ""); img != "" {
			found.ImageURL = img
		} else if img := doc.Find(`meta[name="twitter:image"]`).AttrOr("content", ""); img != "" {
			found.ImageURL = img
		}
	}

	return found, nil
}

// extractRecipeFromLD attempts to pull RecipeData out of a JSON-LD object.
// Handles both top-level Recipe objects and @graph arrays.
func extractRecipeFromLD(obj map[string]any) *RecipeData {
	// Check for @graph wrapper
	if graph, ok := obj["@graph"]; ok {
		if items, ok := graph.([]any); ok {
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					if r := extractRecipeFromLD(m); r != nil {
						return r
					}
				}
			}
		}
		return nil
	}

	// Check @type is "Recipe"
	typeVal, _ := obj["@type"].(string)
	if !strings.EqualFold(typeVal, "recipe") {
		return nil
	}

	data := &RecipeData{}

	if v, ok := obj["name"].(string); ok {
		data.Name = strings.TrimSpace(v)
	}
	if v, ok := obj["description"].(string); ok {
		data.Description = strings.TrimSpace(v)
	}
	if v, ok := obj["recipeCuisine"].(string); ok {
		data.Cuisine = v
	}

	// recipeIngredient: string array
	if raw, ok := obj["recipeIngredient"]; ok {
		if arr, ok := raw.([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					data.Ingredients = append(data.Ingredients, strings.TrimSpace(s))
				}
			}
		}
	}

	// recipeInstructions: string, []string, or []HowToStep{text}
	if raw, ok := obj["recipeInstructions"]; ok {
		data.Instructions = extractInstructions(raw)
	}

	// image: string URL or ImageObject{url} or []either
	if raw, ok := obj["image"]; ok {
		data.ImageURL = extractImageURL(raw)
	}

	// Durations stored as ISO 8601 strings (e.g. "PT30M", "PT1H15M")
	if v, ok := obj["prepTime"].(string); ok {
		if mins := parseISO8601Duration(v); mins > 0 {
			data.PrepTime = &mins
		}
	}
	if v, ok := obj["cookTime"].(string); ok {
		if mins := parseISO8601Duration(v); mins > 0 {
			data.CookTime = &mins
		}
	}

	// recipeYield: can be a string or an array
	switch y := obj["recipeYield"].(type) {
	case string:
		if n := parseServings(y); n > 0 {
			data.Servings = &n
		}
	case []any:
		if len(y) > 0 {
			if s, ok := y[0].(string); ok {
				if n := parseServings(s); n > 0 {
					data.Servings = &n
				}
			}
		}
	}

	if data.Name == "" && len(data.Ingredients) == 0 {
		return nil
	}
	return data
}

// parseISO8601Duration converts an ISO 8601 duration string like "PT1H30M"
// or "PT45M" to a whole number of minutes.
var iso8601Re = regexp.MustCompile(`(?i)PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)

func parseISO8601Duration(s string) int {
	m := iso8601Re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	hours, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	// ignore seconds for cooking purposes
	return hours*60 + mins
}

// parseServings extracts a leading integer from strings like "4 servings",
// "serves 4", "4-6", or just "4".
var servingsRe = regexp.MustCompile(`\d+`)

func parseServings(s string) int {
	m := servingsRe.FindString(s)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

// extractImageURL normalises the schema.org image property into a single URL string.
// The value can be a plain string, an ImageObject map with a "url" key, or a
// []any of either of the above — we take the first usable value.
func extractImageURL(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if u, ok := v["url"].(string); ok {
			return strings.TrimSpace(u)
		}
	case []any:
		for _, item := range v {
			if s := extractImageURL(item); s != "" {
				return s
			}
		}
	}
	return ""
}

// extractInstructions normalises the various schema.org recipeInstructions
// formats into a flat []string of step texts.
func extractInstructions(raw any) []string {
	var steps []string
	switch v := raw.(type) {
	case string:
		// Plain text — split on newlines and/or ". " boundaries
		for _, line := range strings.Split(v, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				steps = append(steps, line)
			}
		}
	case []any:
		for _, item := range v {
			switch step := item.(type) {
			case string:
				if s := strings.TrimSpace(step); s != "" {
					steps = append(steps, s)
				}
			case map[string]any:
				// HowToStep or HowToSection
				if t, _ := step["@type"].(string); strings.EqualFold(t, "HowToSection") {
					// Recurse into itemListElement
					if sub, ok := step["itemListElement"]; ok {
						steps = append(steps, extractInstructions(sub)...)
					}
				} else {
					// HowToStep — prefer "text", fall back to "name"
					text, _ := step["text"].(string)
					if text == "" {
						text, _ = step["name"].(string)
					}
					if s := strings.TrimSpace(text); s != "" {
						steps = append(steps, s)
					}
				}
			}
		}
	}
	return steps
}

func cleanStringSlice(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
