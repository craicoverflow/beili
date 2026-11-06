// Package main re-imports every existing recipe through the scraper and AI
// standardiser, updating each meal in place. Meal IDs are kept, so ratings,
// the cooked log and plan entries (all keyed by meal_id) are untouched.
//
// Name, category, meal types and notes are user-curated and are kept; the
// scraped description, ingredients, instructions, times, servings, cuisine and
// image are replaced. Meals without a scrapeable URL source (or whose page no
// longer scrapes) have their stored text standardised instead.
//
// Usage (dry run by default):
//
//	go run ./cmd/reimport -data ./data
//	go run ./cmd/reimport -data ./data -apply
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/scraper"
	"github.com/craicoverflow/beili/internal/store"
)

func main() {
	dataDir := flag.String("data", "", "path to data directory (default: DATA_DIR / config default)")
	apply := flag.Bool("apply", false, "write changes (default is a dry run)")
	only := flag.String("id", "", "re-import a single meal ID")
	flag.Parse()

	cfg := config.Load()
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}

	var provider ai.Provider
	switch {
	case cfg.AIProvider == "anthropic" && cfg.AnthropicAPIKey != "":
		provider = ai.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicModel)
	case cfg.AIProvider == "gemini" && cfg.GeminiAPIKey != "":
		p, err := ai.NewGeminiProvider(context.Background(), cfg.GeminiAPIKey, cfg.GeminiModel)
		if err != nil {
			fatal("gemini init", err)
		}
		provider = p
	default:
		fatal("no AI provider configured", fmt.Errorf("set ANTHROPIC_API_KEY or GEMINI_API_KEY"))
	}

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		fatal("open database", err)
	}
	defer database.Close()

	ctx := context.Background()
	mealStore := store.NewMealStore(database)
	sc := scraper.NewSchemaOrgScraper()

	all, err := mealStore.List(ctx, store.ListFilters{Category: "all"})
	if err != nil {
		fatal("list meals", err)
	}

	var updated, fallback, failed int
	for _, summary := range all {
		if *only != "" && summary.ID != *only {
			continue
		}
		meal, err := mealStore.GetByID(ctx, summary.ID, "")
		if err != nil {
			slog.Error("load meal", "id", summary.ID, "err", err)
			failed++
			continue
		}

		how, err := reimport(ctx, sc, provider, meal)
		if err != nil {
			slog.Error("skipped", "meal", meal.Name, "id", meal.ID, "err", err)
			failed++
			continue
		}
		if how == "stored" {
			fallback++
		}

		if *apply {
			if err := mealStore.Update(ctx, meal, meal.Sources); err != nil {
				slog.Error("update", "meal", meal.Name, "id", meal.ID, "err", err)
				failed++
				continue
			}
		}
		updated++
		slog.Info("re-imported", "meal", meal.Name, "from", how, "ingredients", len(meal.Ingredients), "applied", *apply)
	}

	fmt.Printf("\n%d re-imported (%d from stored text), %d failed", updated, fallback, failed)
	if !*apply {
		fmt.Print(" — dry run, nothing written; pass -apply")
	}
	fmt.Println()
	if failed > 0 {
		os.Exit(1)
	}
}

// reimport rewrites meal in memory and reports where the text came from:
// "scrape" or "stored".
func reimport(ctx context.Context, sc scraper.Scraper, p ai.Provider, meal *models.Meal) (string, error) {
	how := "stored"
	ingredients, instructions := []string(meal.Ingredients), []string(meal.Instructions)

	if src := scrapeable(meal); src != nil {
		if data, err := sc.Scrape(ctx, src.URL); err == nil && len(data.Ingredients) > 0 {
			how = "scrape"
			ingredients, instructions = data.Ingredients, data.Instructions
			meal.Description = data.Description
			meal.ImageURL = data.ImageURL
			meal.PrepTime, meal.CookTime, meal.Servings = data.PrepTime, data.CookTime, data.Servings
			meal.Cuisine = data.Cuisine
		} else {
			slog.Warn("scrape failed, standardising stored text", "meal", meal.Name, "url", src.URL, "err", err)
		}
	}
	if len(ingredients) == 0 {
		return how, fmt.Errorf("no ingredients to standardise")
	}

	ings, steps, err := ai.Standardize(ctx, p, ingredients, instructions)
	if err != nil {
		return how, err
	}
	meal.Ingredients = ings
	meal.Instructions = steps
	// Names were derived from the old ingredient lines; empty means "regenerate".
	meal.ShoppingNames = models.StringList{}
	return how, nil
}

// scrapeable returns the first plain URL source (not a video).
func scrapeable(meal *models.Meal) *models.Source {
	for i := range meal.Sources {
		s := &meal.Sources[i]
		if s.Type == models.SourceTypeURL && !s.IsVideo() && strings.HasPrefix(s.URL, "http") {
			return s
		}
	}
	return nil
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
