package auth

import (
	"net/http"
	"strings"

	"github.com/craicoverflow/beili/internal/config"
)

func Middleware(cfg config.Config, exemptPrefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.IsHA {
				next.ServeHTTP(w, r)
				return
			}

			for _, prefix := range exemptPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			id := r.Header.Get("X-Remote-User-Id")
			if id == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			u := &User{
				ID:          id,
				Name:        r.Header.Get("X-Remote-User-Name"),
				DisplayName: r.Header.Get("X-Remote-User-Display-Name"),
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}
