package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	appdb "github.com/craicoverflow/beili/internal/db"
	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/store"
)

// openTestDB opens a real SQLite database in a temp directory with migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := appdb.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func intPtr(n int) *int { return &n }

// --- MealStore ---

func TestMealStore_CreateAndGetByID(t *testing.T) {
	db := openTestDB(t)
	s := store.NewMealStore(db)
	ctx := context.Background()

	meal := &models.Meal{
		Name:        "Spaghetti Carbonara",
		Description: "Classic Roman pasta",
		MealTypes:   models.MealTypes{models.MealTypeDinner},
		Cuisine:     "Italian",
		PrepTime:    intPtr(10),
		CookTime:    intPtr(20),
		Servings:    intPtr(2),
		Ingredients: []string{"pasta", "eggs", "pancetta", "pecorino"},
		Notes:       "Use guanciale if available",
	}
	sources := []models.Source{
		{Type: models.SourceTypeURL, Title: "Recipe site", URL: "https://example.com/carbonara"},
	}

	if err := s.Create(ctx, meal, sources); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if meal.ID == "" {
		t.Fatal("expected ID to be set after Create")
	}

	got, err := s.GetByID(ctx, meal.ID, "")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.Name != meal.Name {
		t.Errorf("Name: got %q, want %q", got.Name, meal.Name)
	}
	if got.Cuisine != meal.Cuisine {
		t.Errorf("Cuisine: got %q, want %q", got.Cuisine, meal.Cuisine)
	}
	if len(got.Ingredients) != 4 {
		t.Errorf("Ingredients: got %d, want 4", len(got.Ingredients))
	}
	if len(got.Sources) != 1 {
		t.Fatalf("Sources: got %d, want 1", len(got.Sources))
	}
	if got.Sources[0].URL != "https://example.com/carbonara" {
		t.Errorf("Source URL: got %q", got.Sources[0].URL)
	}
}

func TestMealStore_GetByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.NewMealStore(db)

	_, err := s.GetByID(context.Background(), "nonexistent-id", "")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMealStore_Update(t *testing.T) {
	db := openTestDB(t)
	s := store.NewMealStore(db)
	ctx := context.Background()

	meal := &models.Meal{Name: "Toast", MealTypes: models.MealTypes{models.MealTypeBreakfast}}
	if err := s.Create(ctx, meal, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	meal.Name = "French Toast"
	meal.Cuisine = "French"

	newSources := []models.Source{
		{Type: models.SourceTypeBook, Title: "Cookbook", PageReference: "p.42"},
	}
	if err := s.Update(ctx, meal, newSources); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.GetByID(ctx, meal.ID, "")
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Name != "French Toast" {
		t.Errorf("Name: got %q, want %q", got.Name, "French Toast")
	}
	if len(got.Sources) != 1 || got.Sources[0].PageReference != "p.42" {
		t.Errorf("Sources after update: %+v", got.Sources)
	}
}

func TestMealStore_Delete(t *testing.T) {
	db := openTestDB(t)
	s := store.NewMealStore(db)
	ctx := context.Background()

	meal := &models.Meal{Name: "Porridge", MealTypes: models.MealTypes{models.MealTypeBreakfast}}
	if err := s.Create(ctx, meal, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Delete(ctx, meal.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.GetByID(ctx, meal.ID, "")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestMealStore_List_FiltersAndSearch(t *testing.T) {
	db := openTestDB(t)
	s := store.NewMealStore(db)
	ctx := context.Background()

	meals := []models.Meal{
		{Name: "Pancakes", MealTypes: models.MealTypes{models.MealTypeBreakfast}, Ingredients: []string{"flour", "eggs", "milk"}},
		{Name: "Beef Stew", MealTypes: models.MealTypes{models.MealTypeDinner}, Cuisine: "Irish", Ingredients: []string{"beef", "carrots", "potatoes"}},
		{Name: "Caesar Salad", MealTypes: models.MealTypes{models.MealTypeLunch}, Ingredients: []string{"romaine", "parmesan", "croutons"}},
	}
	for i := range meals {
		if err := s.Create(ctx, &meals[i], nil); err != nil {
			t.Fatalf("Create meal %d: %v", i, err)
		}
	}
	// Seed per-user ratings: Pancakes=4, Beef Stew=5, Caesar Salad=3
	if err := s.UpsertUserRating(ctx, meals[0].ID, "user1", 4); err != nil {
		t.Fatalf("UpsertUserRating: %v", err)
	}
	if err := s.UpsertUserRating(ctx, meals[1].ID, "user1", 5); err != nil {
		t.Fatalf("UpsertUserRating: %v", err)
	}
	if err := s.UpsertUserRating(ctx, meals[2].ID, "user1", 3); err != nil {
		t.Fatalf("UpsertUserRating: %v", err)
	}

	t.Run("all", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilters{})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 3 {
			t.Errorf("got %d meals, want 3", len(list))
		}
	})

	t.Run("filter by meal type", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilters{MealType: "dinner"})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Name != "Beef Stew" {
			t.Errorf("unexpected results: %+v", list)
		}
	})

	t.Run("filter by min rating", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilters{MinRating: 4})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 {
			t.Errorf("got %d meals, want 2 (rating >= 4)", len(list))
		}
	})

	t.Run("full text search by ingredient", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilters{Query: "potatoes"})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Name != "Beef Stew" {
			t.Errorf("FTS ingredient search: %+v", list)
		}
	})

	t.Run("full text search by name", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilters{Query: "pancake"})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Name != "Pancakes" {
			t.Errorf("FTS name search: %+v", list)
		}
	})

	t.Run("full text search by cuisine", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilters{Query: "Irish"})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Name != "Beef Stew" {
			t.Errorf("FTS cuisine search: %+v", list)
		}
	})
}

func TestMealStore_Update_ReplacesAllSources(t *testing.T) {
	db := openTestDB(t)
	s := store.NewMealStore(db)
	ctx := context.Background()

	meal := &models.Meal{Name: "Tacos", MealTypes: models.MealTypes{models.MealTypeDinner}}
	originalSources := []models.Source{
		{Type: models.SourceTypeURL, Title: "Site A", URL: "https://a.example.com"},
		{Type: models.SourceTypeURL, Title: "Site B", URL: "https://b.example.com"},
	}
	if err := s.Create(ctx, meal, originalSources); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update with a single new source — both originals should be gone
	newSources := []models.Source{
		{Type: models.SourceTypeBook, Title: "Taco Book", PageReference: "p.1"},
	}
	if err := s.Update(ctx, meal, newSources); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.GetByID(ctx, meal.ID, "")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Sources) != 1 {
		t.Errorf("expected 1 source after update, got %d", len(got.Sources))
	}
	if got.Sources[0].Type != models.SourceTypeBook {
		t.Errorf("expected book source, got %s", got.Sources[0].Type)
	}
}

// --- PlanStore ---

func TestPlanStore_SetAndGetWeek(t *testing.T) {
	db := openTestDB(t)
	ms := store.NewMealStore(db)
	ps := store.NewPlanStore(db)
	ctx := context.Background()

	meal := &models.Meal{Name: "Omelette", MealTypes: models.MealTypes{models.MealTypeBreakfast}}
	if err := ms.Create(ctx, meal, nil); err != nil {
		t.Fatalf("Create meal: %v", err)
	}

	weekStart := time.Date(2025, time.April, 14, 0, 0, 0, 0, time.UTC) // Monday
	date := "2025-04-14"

	entry := &models.MealPlanEntry{
		Date:     date,
		MealType: models.MealTypeBreakfast,
		MealID:   &meal.ID,
	}
	if err := ps.SetEntry(ctx, entry); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected entry ID to be set after SetEntry")
	}

	entries, err := ps.GetWeek(ctx, weekStart)
	if err != nil {
		t.Fatalf("GetWeek: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("GetWeek: got %d entries, want 1", len(entries))
	}
	if entries[0].Date != date {
		t.Errorf("Date: got %q, want %q", entries[0].Date, date)
	}
	if entries[0].Meal == nil || entries[0].Meal.Name != "Omelette" {
		t.Errorf("Meal not populated: %+v", entries[0].Meal)
	}
}

func TestPlanStore_SetEntry_ReplacesExistingSlot(t *testing.T) {
	db := openTestDB(t)
	ms := store.NewMealStore(db)
	ps := store.NewPlanStore(db)
	ctx := context.Background()

	meal1 := &models.Meal{Name: "Meal A", MealTypes: models.MealTypes{models.MealTypeDinner}}
	meal2 := &models.Meal{Name: "Meal B", MealTypes: models.MealTypes{models.MealTypeDinner}}
	for _, m := range []*models.Meal{meal1, meal2} {
		if err := ms.Create(ctx, m, nil); err != nil {
			t.Fatalf("Create meal: %v", err)
		}
	}

	weekStart := time.Date(2025, time.April, 14, 0, 0, 0, 0, time.UTC)
	date := "2025-04-14"

	entry1 := &models.MealPlanEntry{Date: date, MealType: models.MealTypeDinner, MealID: &meal1.ID}
	if err := ps.SetEntry(ctx, entry1); err != nil {
		t.Fatalf("SetEntry 1: %v", err)
	}

	entry2 := &models.MealPlanEntry{Date: date, MealType: models.MealTypeDinner, MealID: &meal2.ID}
	if err := ps.SetEntry(ctx, entry2); err != nil {
		t.Fatalf("SetEntry 2: %v", err)
	}

	entries, err := ps.GetWeek(ctx, weekStart)
	if err != nil {
		t.Fatalf("GetWeek: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(entries))
	}
	if entries[0].Meal == nil || entries[0].Meal.Name != "Meal B" {
		t.Errorf("expected Meal B after upsert, got: %+v", entries[0].Meal)
	}
}

func TestPlanStore_RemoveEntry(t *testing.T) {
	db := openTestDB(t)
	ms := store.NewMealStore(db)
	ps := store.NewPlanStore(db)
	ctx := context.Background()

	meal := &models.Meal{Name: "Salad", MealTypes: models.MealTypes{models.MealTypeLunch}}
	if err := ms.Create(ctx, meal, nil); err != nil {
		t.Fatalf("Create meal: %v", err)
	}

	weekStart := time.Date(2025, time.April, 14, 0, 0, 0, 0, time.UTC)
	date := "2025-04-14"

	entry := &models.MealPlanEntry{Date: date, MealType: models.MealTypeLunch, MealID: &meal.ID}
	if err := ps.SetEntry(ctx, entry); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}

	if err := ps.RemoveEntry(ctx, entry.ID); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}

	entries, err := ps.GetWeek(ctx, weekStart)
	if err != nil {
		t.Fatalf("GetWeek: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(entries))
	}
}

func TestPlanStore_GetWeek_OnlyReturnsRequestedWeek(t *testing.T) {
	db := openTestDB(t)
	ms := store.NewMealStore(db)
	ps := store.NewPlanStore(db)
	ctx := context.Background()

	meal := &models.Meal{Name: "Soup", MealTypes: models.MealTypes{models.MealTypeLunch}}
	if err := ms.Create(ctx, meal, nil); err != nil {
		t.Fatalf("Create meal: %v", err)
	}

	// One entry in week of Apr 14, one in the following week
	e1 := &models.MealPlanEntry{Date: "2025-04-14", MealType: models.MealTypeLunch, MealID: &meal.ID}
	e2 := &models.MealPlanEntry{Date: "2025-04-21", MealType: models.MealTypeLunch, MealID: &meal.ID}
	for _, e := range []*models.MealPlanEntry{e1, e2} {
		if err := ps.SetEntry(ctx, e); err != nil {
			t.Fatalf("SetEntry: %v", err)
		}
	}

	weekStart := time.Date(2025, time.April, 14, 0, 0, 0, 0, time.UTC)
	entries, err := ps.GetWeek(ctx, weekStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for week, got %d", len(entries))
	}
	if entries[0].Date != "2025-04-14" {
		t.Errorf("wrong entry returned: %+v", entries[0])
	}
}
