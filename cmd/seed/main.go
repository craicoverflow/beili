// Package main is a development utility that loads seed data from
// seeds/seed.json into the local SQLite database.
//
// Usage:
//
//	go run ./cmd/seed
//	go run ./cmd/seed -data ./data -file ./seeds/seed.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
)

func main() {
	dataDir := flag.String("data", "./data", "path to data directory")
	seedFile := flag.String("file", "./seeds/seed.json", "path to seed JSON file")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	database, err := db.Open(*dataDir)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	data, err := os.ReadFile(*seedFile)
	if err != nil {
		slog.Error("read seed file", "file", *seedFile, "err", err)
		os.Exit(1)
	}

	var payload seedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		slog.Error("parse seed file", "err", err)
		os.Exit(1)
	}

	mealStore := store.NewMealStore(database)
	ctx := context.Background()

	// Build set of existing meal names to skip duplicates.
	existing, err := mealStore.List(ctx, store.ListFilters{})
	if err != nil {
		slog.Error("load existing meals", "err", err)
		os.Exit(1)
	}
	nameSet := make(map[string]bool, len(existing))
	for _, m := range existing {
		nameSet[strings.ToLower(m.Name)] = true
	}

	var inserted, skipped int
	for _, sm := range payload.Meals {
		if nameSet[strings.ToLower(sm.Name)] {
			slog.Info("skipped (already exists)", "meal", sm.Name)
			skipped++
			continue
		}

		mealTypes := make(models.MealTypes, len(sm.MealTypes))
		for i, mt := range sm.MealTypes {
			mealTypes[i] = models.MealType(mt)
		}

		meal := &models.Meal{
			Name:         sm.Name,
			Description:  sm.Description,
			MealTypes:    mealTypes,
			Cuisine:      sm.Cuisine,
			PrepTime:     sm.PrepTime,
			CookTime:     sm.CookTime,
			Servings:     sm.Servings,
			Ingredients:  sm.Ingredients,
			Instructions: sm.Instructions,
			ImageURL:     sm.ImageURL,
			Rating:       sm.Rating,
			Notes:        sm.Notes,
		}

		sources := make([]models.Source, len(sm.Sources))
		for i, s := range sm.Sources {
			sources[i] = models.Source{
				Type:          models.SourceType(s.Type),
				Title:         s.Title,
				URL:           s.URL,
				PageReference: s.PageReference,
				Notes:         s.Notes,
			}
		}

		if err := mealStore.Create(ctx, meal, sources); err != nil {
			slog.Error("insert meal", "meal", sm.Name, "err", err)
			os.Exit(1)
		}

		slog.Info("inserted", "meal", sm.Name)
		nameSet[strings.ToLower(sm.Name)] = true
		inserted++
	}

	slog.Info("seed complete", "inserted", inserted, "skipped", skipped, "elapsed", time.Since(time.Now()))
}

// seedPayload mirrors the export format so seed data can be copy-pasted from exports.
type seedPayload struct {
	Version int        `json:"version"`
	Meals   []seedMeal `json:"meals"`
}

type seedMeal struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	MealTypes    []string     `json:"meal_types"`
	Cuisine      string       `json:"cuisine"`
	PrepTime     *int         `json:"prep_time"`
	CookTime     *int         `json:"cook_time"`
	Servings     *int         `json:"servings"`
	Ingredients  []string     `json:"ingredients"`
	Instructions []string     `json:"instructions"`
	ImageURL     string       `json:"image_url"`
	Rating       *int         `json:"rating"`
	Notes        string       `json:"notes"`
	Sources      []seedSource `json:"sources"`
}

type seedSource struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	PageReference string `json:"page_reference"`
	Notes         string `json:"notes"`
}
