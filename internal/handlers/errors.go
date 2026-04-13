package handlers

import (
	"log/slog"
	"net/http"

	"github.com/craicoverflow/my-recipe-manager/internal/templates/components"
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
