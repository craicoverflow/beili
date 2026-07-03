package handlers

import (
	"log/slog"
	"net/http"

	"github.com/craicoverflow/beili/internal/auth"
	"github.com/craicoverflow/beili/internal/templates/components"
	"github.com/craicoverflow/beili/internal/templates/layout"
)

// respondError writes an error response. For HTMX requests it renders an inline
// error banner partial so the page layout is preserved; for full-page requests it
// falls back to a plain HTTP error.
func respondError(w http.ResponseWriter, r *http.Request, status int, userMsg string, logFields ...any) {
	if len(logFields) > 0 {
		slog.Error(userMsg, logFields...)
	}

	w.WriteHeader(status)

	if r.Header.Get("HX-Request") == "true" {
		if err := components.ErrorBanner(userMsg).Render(r.Context(), w); err != nil {
			slog.Error("render error banner", "err", err)
		}
		return
	}

	http.Error(w, userMsg, status)
}

// RespondNotFound is the exported entry point for chi's catch-all r.NotFound,
// which has no handler struct (and thus no h.cfg.BasePath) to call through.
func RespondNotFound(w http.ResponseWriter, r *http.Request, basePath string) {
	respondNotFound(w, r, basePath)
}

// respondNotFound renders the styled 404 page for full-page requests, or the
// inline error banner for HTMX partial requests. basePath is needed to link
// back to the meals list from a bare Go handler with no app-shell wrapper.
func respondNotFound(w http.ResponseWriter, r *http.Request, basePath string) {
	w.WriteHeader(http.StatusNotFound)

	if r.Header.Get("HX-Request") == "true" {
		if err := components.ErrorBanner("Not found").Render(r.Context(), w); err != nil {
			slog.Error("render error banner", "err", err)
		}
		return
	}

	page := layout.NotFoundContent(basePath)
	if err := layout.Base("Not found", basePath, auth.UserFromContext(r.Context()), false, page).Render(r.Context(), w); err != nil {
		slog.Error("render not found page", "err", err)
	}
}
