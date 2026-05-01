package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/scraper"
	"github.com/craicoverflow/beili/internal/templates/components"
)

// ScrapeHandler handles recipe URL scraping requests.
type ScrapeHandler struct {
	scraper scraper.Scraper
	cfg     config.Config
}

// NewScrapeHandler creates a ScrapeHandler with a SchemaOrgScraper.
func NewScrapeHandler(cfg config.Config) *ScrapeHandler {
	return &ScrapeHandler{
		scraper: scraper.NewSchemaOrgScraper(),
		cfg:     cfg,
	}
}

// HandleScrape accepts a URL via POST form and returns an HTMX partial
// containing OOB swaps that populate the meal form fields.
func (h *ScrapeHandler) HandleScrape(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.FormValue("import_url"))
	if rawURL == "" {
		rawURL = strings.TrimSpace(r.FormValue("source_url_0"))
	}
	if rawURL == "" {
		// Try any source URL field (source_url_N)
		if err := r.ParseForm(); err == nil {
			for key, vals := range r.Form {
				if strings.HasPrefix(key, "source_url_") && len(vals) > 0 && vals[0] != "" {
					rawURL = vals[0]
					break
				}
			}
		}
	}

	if rawURL == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// YouTube URLs don't have schema.org recipe data — handle them directly.
	if models.IsYouTubeURL(rawURL) {
		data := &scraper.RecipeData{IsYouTube: true}
		if title, err := fetchYouTubeTitle(r.Context(), rawURL); err == nil {
			data.Name = title
		}
		if err := components.ScrapedPreview(data, rawURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render scraped preview", "err", err)
		}
		return
	}

	data, err := h.scraper.Scrape(r.Context(), rawURL)
	if err != nil {
		if errors.Is(err, scraper.ErrNoRecipeFound) {
			slog.Info("no recipe schema found", "url", rawURL)
			if renderErr := components.ScrapeError("No recipe data found on this page. You can still fill in the fields manually.").Render(r.Context(), w); renderErr != nil {
				slog.Error("render scrape error", "err", renderErr)
			}
			return
		}
		slog.Warn("scrape failed", "url", rawURL, "err", err)
		if renderErr := components.ScrapeError("Could not fetch the page. Check the URL and try again.").Render(r.Context(), w); renderErr != nil {
			slog.Error("render scrape error", "err", renderErr)
		}
		return
	}

	if err := components.ScrapedPreview(data, rawURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render scraped preview", "err", err)
	}
}

func fetchYouTubeTitle(ctx context.Context, videoURL string) (string, error) {
	endpoint := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(videoURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("oembed request failed")
	}
	var result struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Title, nil
}
