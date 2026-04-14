package store

import (
	"context"
	"strings"
	"time"

	"github.com/craicoverflow/my-recipe-manager/internal/models"
	"github.com/google/uuid"
)

// LogCooked records today as a cook date for the given meal.
func (s *MealStore) LogCooked(ctx context.Context, mealID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cooked_log (id, meal_id, cooked_on, created_at) VALUES (?, ?, ?, ?)`,
		uuid.New().String(),
		mealID,
		time.Now().UTC().Format("2006-01-02"),
		time.Now().UTC(),
	)
	return err
}

// loadLastCooked fills in the LastCooked field for each meal in the slice
// using a single batch query against cooked_log.
func (s *MealStore) loadLastCooked(ctx context.Context, meals []models.Meal) error {
	if len(meals) == 0 {
		return nil
	}

	ids := make([]any, len(meals))
	idxByID := make(map[string]int, len(meals))
	for i, m := range meals {
		ids[i] = m.ID
		idxByID[m.ID] = i
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := s.db.QueryContext(ctx,
		`SELECT meal_id, MAX(cooked_on) FROM cooked_log WHERE meal_id IN (`+placeholders+`) GROUP BY meal_id`,
		ids...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mealID, cookedOn string
		if err := rows.Scan(&mealID, &cookedOn); err != nil {
			return err
		}
		if idx, ok := idxByID[mealID]; ok {
			c := cookedOn
			meals[idx].LastCooked = &c
		}
	}
	return rows.Err()
}
