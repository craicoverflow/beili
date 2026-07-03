package models

import "time"

// ShoppingExtra is an ad-hoc shopping list item not tied to a planned meal,
// scoped to a week (e.g. "2025-W15").
type ShoppingExtra struct {
	ID        string
	Week      string
	Text      string
	Checked   bool
	CreatedAt time.Time
}

// MealPlanEntry represents a single meal assigned to a day and meal-type slot.
type MealPlanEntry struct {
	ID         string
	Date       string   // YYYY-MM-DD
	MealType   MealType
	MealID     *string  // nullable FK to meals.id
	Meal       *Meal    // populated by join when MealID is set
	CustomMeal string   // free-text if no MealID
	Notes      string
	Servings   *int // nil means "use the meal's default servings"
	IsLeftover bool // true for extra-day entries that need no new shopping ingredients
	CreatedAt  time.Time
}
