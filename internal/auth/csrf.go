package auth

import (
	"net/http"

	"github.com/craicoverflow/beili/internal/config"
)

// mutatingMethods are the HTTP methods a cross-site request could use to
// change state; GET/HEAD/OPTIONS never mutate and are left alone.
var mutatingMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodDelete: true,
	http.MethodPatch:  true,
}

// CSRFMiddleware rejects cross-site mutating requests using Sec-Fetch-Site
// and Origin — no token needed, which fits the no-JS-framework HTMX stack.
// Requests with neither header (non-browser clients hitting the app directly,
// e.g. curl) are allowed through; that's an accepted gap for this threat model
// since the app has no cross-origin credentialed API surface otherwise.
//
// HA ingress mode is exempt: Home Assistant's reverse proxy can rewrite Origin
// and stopped headers can not be relied on, and the ingress boundary itself is
// the trust boundary there (see auth.Middleware's X-Remote-User-Id check).
func CSRFMiddleware(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.IsHA || !mutatingMethods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}

			if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
				http.Error(w, "Forbidden (cross-site request)", http.StatusForbidden)
				return
			}

			if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r) {
				http.Error(w, "Forbidden (origin mismatch)", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// sameOrigin reports whether the Origin header's host matches the request's
// own host (preferring X-Forwarded-Host when the app is behind a proxy).
func sameOrigin(origin string, r *http.Request) bool {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return originHost(origin) == host
}

// originHost extracts the host[:port] portion of an Origin header value
// (e.g. "https://example.com:8080" -> "example.com:8080").
func originHost(origin string) string {
	s := origin
	if i := indexAfterScheme(s); i >= 0 {
		s = s[i:]
	}
	return s
}

// indexAfterScheme returns the index just past "://" in a URL, or -1 if absent.
func indexAfterScheme(s string) int {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == ':' && s[i+1] == '/' && s[i+2] == '/' {
			return i + 3
		}
	}
	return -1
}
