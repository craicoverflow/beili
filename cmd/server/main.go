package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/craicoverflow/my-recipe-manager/internal/config"
	"github.com/craicoverflow/my-recipe-manager/internal/db"
	"github.com/craicoverflow/my-recipe-manager/internal/handlers"
	"github.com/craicoverflow/my-recipe-manager/internal/store"
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
	mealsHandler := handlers.NewMealsHandler(mealStore, cfg)
	scrapeHandler := handlers.NewScrapeHandler()
	searchHandler := handlers.NewSearchHandler(mealStore, cfg)
	planHandler := handlers.NewPlanHandler(planStore, mealStore, cfg)
	shoppingHandler := handlers.NewShoppingHandler(planStore, mealStore, cfg)
	duplicateHandler := handlers.NewDuplicateHandler(mealStore, cfg)

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
			next.ServeHTTP(w, r)
		})
	})

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

	base := cfg.BasePath

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, base+"/meals", http.StatusFound)
	})

	// Meals
	r.Get(base+"/meals", mealsHandler.HandleList)
	r.Get(base+"/meals/new", mealsHandler.HandleNew)
	r.Post(base+"/meals", mealsHandler.HandleCreate)
	r.Get(base+"/meals/{id}", mealsHandler.HandleDetail)
	r.Get(base+"/meals/{id}/edit", mealsHandler.HandleEdit)
	r.Put(base+"/meals/{id}", mealsHandler.HandleUpdate)
	r.Post(base+"/meals/{id}", mealsHandler.HandleUpdate) // fallback for non-HTMX browsers
	r.Delete(base+"/meals/{id}", mealsHandler.HandleDelete)
	r.Post(base+"/meals/{id}/duplicate", duplicateHandler.HandleDuplicate)

	// HTMX component partials
	r.Get(base+"/components/ingredient-row", mealsHandler.HandleIngredientRow)
	r.Get(base+"/components/source-row", mealsHandler.HandleSourceRow)
	r.Post(base+"/components/source-type-fields", mealsHandler.HandleSourceTypeFields)

	// Search
	r.Get(base+"/search", searchHandler.HandleSearch)

	// Recipe URL scraping
	r.Post(base+"/scrape", scrapeHandler.HandleScrape)

	// Meal plan calendar
	r.Get(base+"/plan", planHandler.HandleWeek)
	r.Get(base+"/plan/assign", planHandler.HandleAssignModal)
	r.Post(base+"/plan", planHandler.HandleAssign)
	r.Delete(base+"/plan/{id}", planHandler.HandleRemove)

	// Shopping list
	r.Get(base+"/shopping", shoppingHandler.HandleList)

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
