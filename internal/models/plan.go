package models

import "time"

// MealPlanEntry represents a single meal assigned to a day and meal-type slot.
type MealPlanEntry struct {
	ID         string
	Date       string   // YYYY-MM-DD
	MealType   MealType
	MealID     *string  // nullable FK to meals.id
	Meal       *Meal    // populated by join when MealID is set
	CustomMeal string   // free-text if no MealID
	Notes      string
	CreatedAt  time.Time
}
