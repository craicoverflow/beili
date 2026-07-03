package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/craicoverflow/beili/internal/config"
	appdb "github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
)

func TestBuildShoppingGroups_ScalesToEntryServings(t *testing.T) {
	db, err := appdb.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	ms := store.NewMealStore(db)
	ctx := context.Background()

	base := 4
	meal := &models.Meal{
		Name:        "Carbonara",
		MealTypes:   models.MealTypes{models.MealTypeDinner},
		Servings:    &base,
		Ingredients: models.StringList{"320g spaghetti"},
	}
	if err := ms.Create(ctx, meal, nil); err != nil {
		t.Fatalf("create meal: %v", err)
	}

	h := NewShoppingHandler(nil, ms, nil, nil, config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/shopping", nil)

	entrySrv := 5
	entries := []models.MealPlanEntry{
		{MealID: &meal.ID, Date: "2025-04-14", MealType: models.MealTypeDinner, Servings: &entrySrv},
	}

	groups, err := h.buildShoppingGroups(req, entries)
	if err != nil {
		t.Fatalf("buildShoppingGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	g := groups[0]
	if g.Servings != 5 {
		t.Errorf("Servings: got %d, want 5", g.Servings)
	}
	if len(g.Ingredients) != 1 || g.Ingredients[0] != "400g spaghetti" {
		t.Errorf("Ingredients: got %v, want [400g spaghetti]", g.Ingredients)
	}
}

func TestBuildShoppingGroups_SkipsLeftovers(t *testing.T) {
	db, err := appdb.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	ms := store.NewMealStore(db)
	ctx := context.Background()

	base := 4
	meal := &models.Meal{
		Name:        "Chilli",
		MealTypes:   models.MealTypes{models.MealTypeDinner},
		Servings:    &base,
		Ingredients: models.StringList{"500g beef mince"},
	}
	if err := ms.Create(ctx, meal, nil); err != nil {
		t.Fatalf("create meal: %v", err)
	}

	h := NewShoppingHandler(nil, ms, nil, nil, config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/shopping", nil)

	entries := []models.MealPlanEntry{
		{MealID: &meal.ID, Date: "2025-04-14", MealType: models.MealTypeDinner},
		{MealID: &meal.ID, Date: "2025-04-15", MealType: models.MealTypeDinner, IsLeftover: true},
	}

	groups, err := h.buildShoppingGroups(req, entries)
	if err != nil {
		t.Fatalf("buildShoppingGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0].Slots) != 1 {
		t.Errorf("Slots: got %d, want 1 (leftover entry should not add a slot)", len(groups[0].Slots))
	}
}
