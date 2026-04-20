package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the server.
type Config struct {
	Port         int
	DataDir      string
	BasePath     string // URL prefix when running behind HA ingress proxy
	IsHA         bool
	ShoppingList       bool   // enable the weekly shopping list feature
	ShoppingWebhookURL string // webhook URL for adding ingredients to HA shopping list
}

// Load reads configuration from environment variables, applying defaults based
// on whether the process is running inside a Home Assistant addon container
// (detected via the SUPERVISOR_TOKEN env var).
func Load() Config {
	isHA := os.Getenv("SUPERVISOR_TOKEN") != ""

	port := 8080
	dataDir := "./data"
	if isHA {
		port = 8099
		dataDir = "/data"
	}

	if v := os.Getenv("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		dataDir = v
	}

	basePath := os.Getenv("INGRESS_PATH") // injected by HA supervisor
	shoppingList := os.Getenv("FEATURE_SHOPPING_LIST") == "true"

	// Shopping webhook: prefer SHOPPING_WEBHOOK_URL (full URL); otherwise combine
	// SHOPPING_WEBHOOK_BASE (default http://localhost:8123) + SHOPPING_WEBHOOK_SLUG.
	shoppingWebhookURL := os.Getenv("SHOPPING_WEBHOOK_URL")
	if shoppingWebhookURL == "" {
		if slug := os.Getenv("SHOPPING_WEBHOOK_SLUG"); slug != "" {
			base := os.Getenv("SHOPPING_WEBHOOK_BASE")
			if base == "" {
				base = "http://localhost:8123"
			}
			shoppingWebhookURL = strings.TrimRight(base, "/") + "/" + strings.TrimLeft(slug, "/")
		}
	}

	cfg := Config{
		Port:               port,
		DataDir:            dataDir,
		BasePath:           basePath,
		IsHA:               isHA,
		ShoppingList:       shoppingList,
		ShoppingWebhookURL: shoppingWebhookURL,
	}

	slog.Info("config loaded",
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"base_path", cfg.BasePath,
		"ha_mode", cfg.IsHA,
		"shopping_list", cfg.ShoppingList,
		"shopping_webhook", shoppingWebhookURL != "",
	)

	return cfg
}
