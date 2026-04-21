package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/handlers"
	"github.com/craicoverflow/beili/internal/store"
)

func main() {
	cfg := config.Load()

	var logHandler slog.Handler
	if cfg.IsHA {
		logHandler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(logHandler))

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	mealStore := store.NewMealStore(database)
	planStore := store.NewPlanStore(database)

	var aiProvider ai.Provider
	if cfg.AIProvider == "gemini" && cfg.GeminiAPIKey != "" {
		p, err := ai.NewGeminiProvider(context.Background(), cfg.GeminiAPIKey, cfg.GeminiModel)
		if err != nil {
			slog.Warn("gemini provider init failed, AI normalisation disabled", "err", err)
		} else {
			aiProvider = p
			slog.Info("AI recipe normalisation enabled", "provider", "gemini", "base_servings", cfg.BaseServings)
		}
	}

	mealsHandler := handlers.NewMealsHandler(mealStore, cfg, aiProvider)
	scrapeHandler := handlers.NewScrapeHandler(cfg)
	searchHandler := handlers.NewSearchHandler(mealStore, cfg)
	planHandler := handlers.NewPlanHandler(planStore, mealStore, cfg)
	shoppingHandler := handlers.NewShoppingHandler(planStore, mealStore, cfg)
	duplicateHandler := handlers.NewDuplicateHandler(mealStore, cfg)
	cookedHandler := handlers.NewCookedHandler(mealStore, cfg)
	randomHandler := handlers.NewRandomHandler(mealStore, cfg)
	exportHandler := handlers.NewExportHandler(mealStore, cfg)
	apiHandler := handlers.NewAPIHandler(planStore, mealStore, cfg)
	shoppingWebhookHandler := handlers.NewShoppingWebhookHandler(cfg)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// HA ingress: pick up base path from proxy header on first request
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.BasePath == "" {
				if p := r.Header.Get("X-Ingress-Path"); p != "" {
					cfg.BasePath = p
				}
			}
			// Strip the ingress prefix if HA forwards the full path (some versions don't strip it)
			if cfg.BasePath != "" && strings.HasPrefix(r.URL.Path, cfg.BasePath) {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, cfg.BasePath)
				if r.URL.RawPath != "" {
					r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, cfg.BasePath)
				}
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}
			next.ServeHTTP(w, r)
		})
	})

	// HA auth: validate X-Remote-User-Id header in HA mode, extract user into context
	r.Use(auth.Middleware(cfg))

	// Method override: HTML forms can only POST; check _method field for PUT/DELETE
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				if m := r.FormValue("_method"); m == "PUT" || m == "DELETE" {
					r.Method = m
				}
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, cfg.BasePath+"/meals", http.StatusFound)
	})

	// Meals
	r.Get("/meals", mealsHandler.HandleList)
	r.Get("/meals/new", mealsHandler.HandleNew)
	r.Post("/meals", mealsHandler.HandleCreate)
	r.Get("/meals/{id}", mealsHandler.HandleDetail)
	r.Get("/meals/{id}/edit", mealsHandler.HandleEdit)
	r.Put("/meals/{id}", mealsHandler.HandleUpdate)
	r.Post("/meals/{id}", mealsHandler.HandleUpdate) // fallback for non-HTMX browsers
	r.Delete("/meals/{id}", mealsHandler.HandleDelete)
	r.Post("/meals/{id}/duplicate", duplicateHandler.HandleDuplicate)
	r.Post("/meals/{id}/add-to-shopping", shoppingWebhookHandler.HandleAddToShoppingList)

	// HTMX component partials
	r.Get("/components/ingredient-row", mealsHandler.HandleIngredientRow)
	r.Get("/components/instruction-row", mealsHandler.HandleInstructionRow)
	r.Get("/components/source-row", mealsHandler.HandleSourceRow)
	r.Post("/components/source-type-fields", mealsHandler.HandleSourceTypeFields)

	// Search
	r.Get("/search", searchHandler.HandleSearch)

	// Recipe URL scraping
	r.Post("/scrape", scrapeHandler.HandleScrape)

	// Random meal (must be before /meals/{id})
	r.Get("/meals/random", randomHandler.HandleRandom)

	// Export / import (must be before /meals/{id})
	r.Get("/meals/export", exportHandler.HandleExport)
	r.Get("/meals/import", exportHandler.HandleImportPage)
	r.Post("/meals/import", exportHandler.HandleImport)

	// Meal plan calendar
	r.Get("/plan", planHandler.HandleWeek)
	r.Get("/plan/assign", planHandler.HandleAssignModal)
	r.Post("/plan", planHandler.HandleAssign)
	r.Delete("/plan/{id}", planHandler.HandleRemove)

	// JSON API (for Home Assistant integration)
	r.Get("/api/plan/week", apiHandler.HandlePlanWeek)
	r.Get("/api/meals", apiHandler.HandleMeals)

	// Shopping list
	if cfg.ShoppingList {
		r.Get("/shopping", shoppingHandler.HandleList)
	}

	// Cook log
	r.Post("/meals/{id}/cooked", cookedHandler.HandleMarkCooked)

	// Inline rating
	r.Post("/meals/{id}/rating", mealsHandler.HandleRating)

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
