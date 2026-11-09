package store_test

import (
	"context"
	"testing"

	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
)

func newMeal(t *testing.T, s *store.MealStore, name string) *models.Meal {
	t.Helper()
	m := &models.Meal{Name: name, MealTypes: models.MealTypes{models.MealTypeDinner}, Ingredients: []string{"1 onion"}}
	if err := s.Create(context.Background(), m, nil); err != nil {
		t.Fatalf("Create %s: %v", name, err)
	}
	return m
}

func TestFeedback_AddAndList(t *testing.T) {
	s := store.NewMealStore(openTestDB(t))
	ctx := context.Background()
	m := newMeal(t, s, "Curry")

	for _, body := range []string{"too sweet", "needs more heat"} {
		if _, err := s.AddFeedback(ctx, m.ID, body); err != nil {
			t.Fatalf("AddFeedback: %v", err)
		}
	}
	got, err := s.ListFeedback(ctx, m.ID)
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(got) != 2 || got[0].Body != "needs more heat" {
		t.Errorf("want newest first, got %+v", got)
	}
}

func TestFork_LineageAndSuperseded(t *testing.T) {
	s := store.NewMealStore(openTestDB(t))
	ctx := context.Background()
	parent := newMeal(t, s, "Curry")

	fork := &models.Meal{Name: "Curry v2", MealTypes: models.MealTypes{models.MealTypeDinner}, Ingredients: []string{"1 onion"}}
	if err := s.CreateFork(ctx, parent.ID, fork, nil); err != nil {
		t.Fatalf("CreateFork: %v", err)
	}

	gotParent, _ := s.GetByID(ctx, parent.ID, "")
	if gotParent.ForkCount != 1 || gotParent.ParentMealID != nil {
		t.Errorf("parent: want 1 fork and no parent, got %d / %v", gotParent.ForkCount, gotParent.ParentMealID)
	}
	gotFork, _ := s.GetByID(ctx, fork.ID, "")
	if gotFork.ParentMealID == nil || *gotFork.ParentMealID != parent.ID || gotFork.ParentName != "Curry" {
		t.Errorf("fork: want parent link to Curry, got %v / %q", gotFork.ParentMealID, gotFork.ParentName)
	}

	// Superseded parents are hidden from filters that ask for it, but not from
	// the default list.
	visible, _ := s.List(ctx, store.ListFilters{HideSuperseded: true})
	if len(visible) != 1 || visible[0].ID != fork.ID {
		t.Errorf("HideSuperseded: want only the fork, got %+v", names(visible))
	}
	all, _ := s.List(ctx, store.ListFilters{})
	if len(all) != 2 {
		t.Errorf("default list: want both meals, got %+v", names(all))
	}
	searched, _ := s.List(ctx, store.ListFilters{Query: "curry", HideSuperseded: true})
	if len(searched) != 1 || searched[0].ID != fork.ID {
		t.Errorf("HideSuperseded search: want only the fork, got %+v", names(searched))
	}
	for i := 0; i < 10; i++ {
		r, err := s.Random(ctx, store.ListFilters{HideSuperseded: true})
		if err != nil || r.ID != fork.ID {
			t.Fatalf("Random with HideSuperseded: want the fork, got %v / %v", r, err)
		}
	}

	// Deleting the last fork restores the parent.
	if err := s.Delete(ctx, fork.ID); err != nil {
		t.Fatalf("Delete fork: %v", err)
	}
	visible, _ = s.List(ctx, store.ListFilters{HideSuperseded: true})
	if len(visible) != 1 || visible[0].ID != parent.ID {
		t.Errorf("after deleting the fork: want the parent back, got %+v", names(visible))
	}
}

func TestFork_DeletingParentKeepsFork(t *testing.T) {
	s := store.NewMealStore(openTestDB(t))
	ctx := context.Background()
	parent := newMeal(t, s, "Curry")
	fork := &models.Meal{Name: "Curry v2", MealTypes: models.MealTypes{models.MealTypeDinner}, Ingredients: []string{"1 onion"}}
	if err := s.CreateFork(ctx, parent.ID, fork, nil); err != nil {
		t.Fatalf("CreateFork: %v", err)
	}
	if err := s.Delete(ctx, parent.ID); err != nil {
		t.Fatalf("Delete parent: %v", err)
	}
	got, err := s.GetByID(ctx, fork.ID, "")
	if err != nil || got.ParentMealID != nil {
		t.Errorf("fork should survive as a standalone meal, got %v / %v", got, err)
	}
}

func TestLineageFeedback_WalksAncestors(t *testing.T) {
	s := store.NewMealStore(openTestDB(t))
	ctx := context.Background()
	a := newMeal(t, s, "A")
	b := &models.Meal{Name: "B", MealTypes: models.MealTypes{models.MealTypeDinner}, Ingredients: []string{"1 onion"}}
	c := &models.Meal{Name: "C", MealTypes: models.MealTypes{models.MealTypeDinner}, Ingredients: []string{"1 onion"}}
	other := newMeal(t, s, "Other")
	if err := s.CreateFork(ctx, a.ID, b, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateFork(ctx, b.ID, c, nil); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ id, body string }{
		{a.ID, "too sweet"}, {b.ID, "too salty"}, {c.ID, "too dry"}, {other.ID, "unrelated"},
	} {
		if _, err := s.AddFeedback(ctx, f.id, f.body); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.LineageFeedback(ctx, c.ID)
	if err != nil {
		t.Fatalf("LineageFeedback: %v", err)
	}
	var bodies []string
	for _, f := range got {
		bodies = append(bodies, f.Body)
	}
	want := []string{"too sweet", "too salty", "too dry"}
	if len(bodies) != 3 || bodies[0] != want[0] || bodies[1] != want[1] || bodies[2] != want[2] {
		t.Errorf("lineage feedback = %v, want %v", bodies, want)
	}
}

func names(ms []models.Meal) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}
