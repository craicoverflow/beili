package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/craicoverflow/beili/internal/ai"
	"github.com/craicoverflow/beili/internal/config"
	appdb "github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/ingredient"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
)

type fakeReviser struct {
	req  ai.ReviseRequest
	resp ai.ReviseResponse
	err  error
}

func (f *fakeReviser) Revise(_ context.Context, req ai.ReviseRequest) (ai.ReviseResponse, error) {
	f.req = req
	return f.resp, f.err
}

func newFeedbackRouter(t *testing.T, reviser ai.Reviser) (http.Handler, *store.MealStore, *models.Meal) {
	t.Helper()
	db, err := appdb.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	s := store.NewMealStore(db)
	meal := &models.Meal{
		Name:         "Curry",
		MealTypes:    models.MealTypes{models.MealTypeDinner},
		Ingredients:  []string{"500 g chicken thighs", "60 ml soy sauce"},
		Instructions: []string{"Fry.", "Simmer 5 minutes."},
	}
	if err := s.Create(context.Background(), meal, nil); err != nil {
		t.Fatal(err)
	}

	h := NewFeedbackHandler(s, reviser, config.Config{BaseServings: 5})
	r := chi.NewRouter()
	r.Get("/meals/{id}/feedback", h.HandleSection)
	r.Post("/meals/{id}/feedback", h.HandleSubmit)
	r.Post("/meals/{id}/fork", h.HandleFork)
	return r, s, meal
}

func post(r http.Handler, target string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestFeedback_SaveNoteWithoutAI(t *testing.T) {
	r, s, meal := newFeedbackRouter(t, nil)

	rec := post(r, "/meals/"+meal.ID+"/feedback", url.Values{"body": {"too salty"}, "action": {"revise"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Note saved.") || !strings.Contains(body, "too salty") {
		t.Errorf("expected saved confirmation and the note, got: %s", body)
	}
	if strings.Contains(body, "suggest a revision") {
		t.Error("revise button must be hidden when no reviser is configured")
	}
	if notes, _ := s.ListFeedback(context.Background(), meal.ID); len(notes) != 1 {
		t.Errorf("want 1 saved note, got %d", len(notes))
	}
}

func TestFeedback_BlankNoteRejected(t *testing.T) {
	r, _, meal := newFeedbackRouter(t, nil)
	if rec := post(r, "/meals/"+meal.ID+"/feedback", url.Values{"body": {"   "}}); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestFeedback_ReviseShowsDiffAndSendsLineage(t *testing.T) {
	rv := &fakeReviser{resp: ai.ReviseResponse{
		Ingredients: []ingredient.Parsed{
			{Qty: 500, Unit: "g", Name: "chicken thighs"},
			{Qty: 40, Unit: "ml", Name: "soy sauce"},
		},
		Instructions: []string{"Fry.", "Simmer 10 minutes."},
		Reasons:      []ai.Reason{{Ref: "soy sauce", Text: "less salty"}, {Ref: "step 2", Text: "thicker sauce"}},
	}}
	r, s, meal := newFeedbackRouter(t, rv)
	if _, err := s.AddFeedback(context.Background(), meal.ID, "too sweet"); err != nil {
		t.Fatal(err)
	}

	rec := post(r, "/meals/"+meal.ID+"/feedback", url.Values{"body": {"too salty"}, "action": {"revise"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, want := range []string{"60 ml soy sauce", "40 ml soy sauce", "less salty", "Simmer 10 minutes.", "thicker sauce", "Save as new version"} {
		if !strings.Contains(body, want) {
			t.Errorf("review page missing %q", want)
		}
	}
	if got := rv.req.Feedback; len(got) != 2 || got[0] != "too sweet" || got[1] != "too salty" {
		t.Errorf("reviser feedback = %v, want lineage oldest first ending with the new note", got)
	}
	if rv.req.Servings != 5 {
		t.Errorf("servings = %d, want the base servings", rv.req.Servings)
	}
}

func TestFeedback_ReviseFailureKeepsNote(t *testing.T) {
	r, s, meal := newFeedbackRouter(t, &fakeReviser{err: errors.New("boom")})

	rec := post(r, "/meals/"+meal.ID+"/feedback", url.Values{"body": {"too salty"}, "action": {"revise"}})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "produce a revision this time") {
		t.Errorf("want a friendly error page, got %d: %s", rec.Code, rec.Body)
	}
	if notes, _ := s.ListFeedback(context.Background(), meal.ID); len(notes) != 1 {
		t.Errorf("note must be saved even when the AI fails, got %d", len(notes))
	}
}

func TestFork_CreatesLinkedMealWithFreshHistory(t *testing.T) {
	r, s, meal := newFeedbackRouter(t, &fakeReviser{})
	ctx := context.Background()
	if err := s.LogCooked(ctx, meal.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertUserRating(ctx, meal.ID, "u", 5); err != nil {
		t.Fatal(err)
	}

	rec := post(r, "/meals/"+meal.ID+"/fork", url.Values{
		"name":         {"Curry v2"},
		"ingredients":  {"500 g chicken thighs\n\n40 ml soy sauce\n"},
		"instructions": {"Fry.\nSimmer 10 minutes."},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	forkID := strings.TrimPrefix(rec.Header().Get("Location"), "/meals/")

	fork, err := s.GetByID(ctx, forkID, "u")
	if err != nil {
		t.Fatalf("GetByID fork: %v", err)
	}
	if fork.ParentMealID == nil || *fork.ParentMealID != meal.ID {
		t.Errorf("fork should link to the parent, got %v", fork.ParentMealID)
	}
	if len(fork.Ingredients) != 2 || fork.Ingredients[1] != "40 ml soy sauce" {
		t.Errorf("ingredients = %v (blank lines should be dropped)", fork.Ingredients)
	}
	if fork.LastCooked != nil || fork.RatingCount != 0 {
		t.Errorf("fork must start fresh, got cooked=%v ratings=%d", fork.LastCooked, fork.RatingCount)
	}
	if parent, _ := s.GetByID(ctx, meal.ID, ""); parent.ForkCount != 1 {
		t.Errorf("parent fork count = %d, want 1", parent.ForkCount)
	}
}

func TestFork_RequiresNameAndIngredients(t *testing.T) {
	r, _, meal := newFeedbackRouter(t, &fakeReviser{})
	rec := post(r, "/meals/"+meal.ID+"/fork", url.Values{"name": {"x"}, "ingredients": {"  \n "}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
