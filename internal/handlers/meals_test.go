package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/craicoverflow/beili/internal/config"
	appdb "github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/store"
)

func listWithHeaders(t *testing.T, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	db, err := appdb.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	h := NewMealsHandler(store.NewMealStore(db), config.Config{BasePath: "/ingress"})
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.HandleList(rec, req)
	return rec
}

// htmx.ajax() ignores a pushUrl option, so the server has to tell htmx to put
// the search URL in the address bar or chips are lost on reload.
func TestHandleList_ChipSearchPushesURL(t *testing.T) {
	rec := listWithHeaders(t, "/meals?q=chicken+AND+rice&category=baking", map[string]string{
		"HX-Request": "true",
		"HX-Target":  "meal-grid",
	})
	want := "/ingress/meals?category=baking&q=chicken+AND+rice"
	if got := rec.Header().Get("HX-Push-Url"); got != want {
		t.Errorf("HX-Push-Url = %q, want %q", got, want)
	}
}

func TestHandleList_InfiniteScrollDoesNotPushURL(t *testing.T) {
	rec := listWithHeaders(t, "/meals?q=chicken&offset=12", map[string]string{
		"HX-Request": "true",
		"HX-Target":  "meal-grid",
	})
	if got := rec.Header().Get("HX-Push-Url"); got != "" {
		t.Errorf("HX-Push-Url = %q, want none for infinite-scroll append", got)
	}
}
