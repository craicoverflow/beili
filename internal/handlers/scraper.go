package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/config"
	"github.com/craicoverflow/beili/internal/ingredient"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/scraper"
	"github.com/craicoverflow/beili/internal/templates/components"
)

// ScrapeHandler handles recipe URL scraping requests.
type ScrapeHandler struct {
	scraper    scraper.Scraper
	cfg        config.Config
	aiProvider ai.Provider // nil when no AI key is configured
}

// NewScrapeHandler creates a ScrapeHandler with a SchemaOrgScraper. aiProvider
// may be nil, in which case imported recipes are shown as scraped.
func NewScrapeHandler(cfg config.Config, aiProvider ai.Provider) *ScrapeHandler {
	return &ScrapeHandler{
		scraper:    scraper.NewSchemaOrgScraper(),
		cfg:        cfg,
		aiProvider: aiProvider,
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

	// Video URLs don't expose schema.org recipe data — handle them directly.
	if models.IsYouTubeURL(rawURL) {
		data := &scraper.RecipeData{VideoKind: "youtube"}
		if title, err := fetchYouTubeTitle(r.Context(), rawURL); err == nil {
			data.Name = title
		}
		if err := components.ScrapedPreview(data, rawURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
			slog.Error("render scraped preview", "err", err)
		}
		return
	}
	if models.IsInstagramURL(rawURL) {
		// Instagram doesn't expose an unauthenticated oEmbed endpoint, so we
		// just create the source row and let the user fill in the title.
		data := &scraper.RecipeData{VideoKind: "instagram"}
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

	h.standardize(r.Context(), data)

	if err := components.ScrapedPreview(data, rawURL, h.cfg.BasePath).Render(r.Context(), w); err != nil {
		slog.Error("render scraped preview", "err", err)
	}
}

// standardize rewrites the scraped ingredients into the canonical stored form
// (metric, one plain number, "<qty> <unit> <name>[, <prep>]") and converts
// imperial quantities in the steps, so internal/scaling can scale the recipe
// exactly. It runs once, at import; the user then reviews the result in the
// form and saving never calls AI. Best-effort: on any failure the scraped
// text is left as-is.
func (h *ScrapeHandler) standardize(ctx context.Context, data *scraper.RecipeData) {
	if h.aiProvider == nil || len(data.Ingredients) == 0 {
		return
	}
	req := ai.StandardizeRequest{Ingredients: data.Ingredients, Instructions: data.Instructions}

	var resp ai.StandardizeResponse
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		aiCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		resp, err = h.aiProvider.Standardize(aiCtx, req)
		cancel()
		if err == nil {
			break
		}
		slog.Warn("ai standardisation attempt failed", "attempt", attempt, "err", err)
	}
	if err != nil {
		slog.Warn("ai standardisation failed, using scraped values", "err", err)
		return
	}

	lines := make([]string, len(resp.Ingredients))
	for i, p := range resp.Ingredients {
		lines[i] = ingredient.Canonical(p)
		if lines[i] == "" { // model returned no name: keep the original line
			lines[i] = data.Ingredients[i]
		}
	}
	data.Ingredients = lines
	data.Instructions = resp.Instructions
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
