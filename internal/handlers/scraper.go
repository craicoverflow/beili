package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/craicoverflow/beili/internal/scraper"
	"github.com/craicoverflow/beili/internal/templates/components"
)

// ScrapeHandler handles recipe URL scraping requests.
type ScrapeHandler struct {
	scraper scraper.Scraper
}

// NewScrapeHandler creates a ScrapeHandler with a SchemaOrgScraper.
func NewScrapeHandler() *ScrapeHandler {
	return &ScrapeHandler{
		scraper: scraper.NewSchemaOrgScraper(),
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

	if err := components.ScrapedPreview(data, rawURL).Render(r.Context(), w); err != nil {
		slog.Error("render scraped preview", "err", err)
	}
}
