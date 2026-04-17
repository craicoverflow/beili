package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the server.
type Config struct {
	Port         int
	DataDir      string
	BasePath     string // URL prefix when running behind HA ingress proxy
	IsHA         bool
	ShoppingList bool // enable the weekly shopping list feature
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

	cfg := Config{
		Port:         port,
		DataDir:      dataDir,
		BasePath:     basePath,
		IsHA:         isHA,
		ShoppingList: shoppingList,
	}

	slog.Info("config loaded",
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"base_path", cfg.BasePath,
		"ha_mode", cfg.IsHA,
		"shopping_list", cfg.ShoppingList,
	)

	return cfg
}
