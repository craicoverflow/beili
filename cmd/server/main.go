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
	// Configure structured logging
	cfg := config.Load()

	var logHandler slog.Handler
	if cfg.IsHA {
		logHandler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(logHandler))

	// Open database
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	// Wire up stores and handlers
	mealStore := store.NewMealStore(database)
	mealsHandler := handlers.NewMealsHandler(mealStore, cfg)

	// Build router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// HA ingress path middleware: reads X-Ingress-Path on first request
	// if BasePath wasn't already set via env var.
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

	base := cfg.BasePath

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, base+"/meals", http.StatusFound)
	})

	r.Route(base+"/meals", func(r chi.Router) {
		r.Get("/", mealsHandler.HandleList)
	})

	r.Get(base+"/search", func(w http.ResponseWriter, r *http.Request) {
		// Placeholder — will be implemented in Phase 4
		http.Redirect(w, r, base+"/meals", http.StatusFound)
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
