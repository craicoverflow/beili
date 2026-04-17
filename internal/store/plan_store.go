package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/craicoverflow/beili/internal/models"
	"github.com/google/uuid"
)

// PlanStore handles all database operations for the meal plan calendar.
type PlanStore struct {
	db *sql.DB
}

// NewPlanStore creates a new PlanStore.
func NewPlanStore(db *sql.DB) *PlanStore {
	return &PlanStore{db: db}
}

// GetWeek returns all meal plan entries for the 7-day window starting at
// weekStart (inclusive), with the associated Meal populated where set.
func (s *PlanStore) GetWeek(ctx context.Context, weekStart time.Time) ([]models.MealPlanEntry, error) {
	weekEnd := weekStart.AddDate(0, 0, 7)

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			mp.id, mp.date, mp.meal_type, mp.meal_id, mp.custom_meal, mp.notes, mp.created_at,
			m.id, m.name, m.rating, m.meal_types, m.prep_time, m.cook_time
		FROM meal_plan mp
		LEFT JOIN meals m ON mp.meal_id = m.id
		WHERE mp.date >= ? AND mp.date < ?
		ORDER BY mp.date, mp.meal_type`,
		weekStart.Format("2006-01-02"),
		weekEnd.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.MealPlanEntry
	for rows.Next() {
		var e models.MealPlanEntry
		var mealID, mealName sql.NullString
		var mealRating sql.NullInt64
		var mealTypes models.MealTypes
		var prepTime, cookTime sql.NullInt64

		err := rows.Scan(
			&e.ID, &e.Date, &e.MealType, &e.MealID, &e.CustomMeal, &e.Notes, &e.CreatedAt,
			&mealID, &mealName, &mealRating, &mealTypes, &prepTime, &cookTime,
		)
		if err != nil {
			return nil, fmt.Errorf("scan plan entry: %w", err)
		}

		if mealID.Valid {
			meal := &models.Meal{
				ID:        mealID.String,
				Name:      mealName.String,
				MealTypes: mealTypes,
			}
			if mealRating.Valid {
				r := int(mealRating.Int64)
				meal.Rating = &r
			}
			if prepTime.Valid {
				p := int(prepTime.Int64)
				meal.PrepTime = &p
			}
			if cookTime.Valid {
				c := int(cookTime.Int64)
				meal.CookTime = &c
			}
			e.Meal = meal
		}

		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SetEntry upserts a meal plan entry. If an entry already exists for the
// same date + meal_type combination, it is replaced.
func (s *PlanStore) SetEntry(ctx context.Context, entry *models.MealPlanEntry) error {
	// Remove any existing entry for this date+type slot first
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM meal_plan WHERE date = ? AND meal_type = ?`,
		entry.Date, entry.MealType,
	)
	if err != nil {
		return fmt.Errorf("clear slot: %w", err)
	}

	entry.ID = uuid.New().String()
	entry.CreatedAt = time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO meal_plan (id, date, meal_type, meal_id, custom_meal, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Date, entry.MealType, entry.MealID,
		entry.CustomMeal, entry.Notes, entry.CreatedAt,
	)
	return err
}

// RemoveEntry deletes a single meal plan entry by ID.
func (s *PlanStore) RemoveEntry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM meal_plan WHERE id = ?`, id)
	return err
}
