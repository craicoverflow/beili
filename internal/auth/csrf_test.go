package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/craicoverflow/beili/internal/config"
)

func TestCSRFMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		method     string
		secFetch   string
		origin     string
		host       string
		isHA       bool
		wantStatus int
	}{
		{"GET always allowed", http.MethodGet, "cross-site", "https://evil.example.com", "beili.local", false, http.StatusOK},
		{"POST same-site allowed", http.MethodPost, "same-origin", "", "beili.local", false, http.StatusOK},
		{"POST cross-site rejected", http.MethodPost, "cross-site", "", "beili.local", false, http.StatusForbidden},
		{"DELETE cross-site rejected", http.MethodDelete, "cross-site", "", "beili.local", false, http.StatusForbidden},
		{"POST matching origin allowed", http.MethodPost, "", "http://beili.local", "beili.local", false, http.StatusOK},
		{"POST mismatched origin rejected", http.MethodPost, "", "https://evil.example.com", "beili.local", false, http.StatusForbidden},
		{"POST no headers allowed (non-browser client)", http.MethodPost, "", "", "beili.local", false, http.StatusOK},
		{"POST cross-site allowed in HA mode", http.MethodPost, "cross-site", "", "beili.local", true, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := CSRFMiddleware(config.Config{IsHA: tt.isHA})(ok)

			req := httptest.NewRequest(tt.method, "/plan", nil)
			req.Host = tt.host
			if tt.secFetch != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetch)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
