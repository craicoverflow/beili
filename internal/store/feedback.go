package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/craicoverflow/beili/internal/models"
	"github.com/google/uuid"
)

// AddFeedback saves a free-text note against a meal.
func (s *MealStore) AddFeedback(ctx context.Context, mealID, body string) (*models.Feedback, error) {
	f := &models.Feedback{
		ID:        uuid.New().String(),
		MealID:    mealID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO meal_feedback (id, meal_id, body, created_at) VALUES (?, ?, ?, ?)`,
		f.ID, f.MealID, f.Body, f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// ListFeedback returns a meal's own feedback, newest first.
func (s *MealStore) ListFeedback(ctx context.Context, mealID string) ([]models.Feedback, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, meal_id, body, created_at FROM meal_feedback WHERE meal_id = ? ORDER BY created_at DESC, rowid DESC`,
		mealID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedback(rows)
}

// LineageFeedback returns the feedback left on a meal and on every ancestor it
// was forked from, oldest first, so a revision can see what earlier forks
// already fixed.
func (s *MealStore) LineageFeedback(ctx context.Context, mealID string) ([]models.Feedback, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE lineage(id, parent_meal_id) AS (
			SELECT id, parent_meal_id FROM meals WHERE id = ?
			UNION
			SELECT m.id, m.parent_meal_id FROM meals m JOIN lineage l ON m.id = l.parent_meal_id
		)
		SELECT id, meal_id, body, created_at FROM meal_feedback
		WHERE meal_id IN (SELECT id FROM lineage)
		ORDER BY created_at, rowid`, mealID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedback(rows)
}

func scanFeedback(rows *sql.Rows) ([]models.Feedback, error) {
	var out []models.Feedback
	for rows.Next() {
		var f models.Feedback
		if err := rows.Scan(&f.ID, &f.MealID, &f.Body, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// CreateFork inserts meal as a fork of parentID. The fork starts fresh: it has
// no ratings, cooked history or feedback of its own.
func (s *MealStore) CreateFork(ctx context.Context, parentID string, meal *models.Meal, sources []models.Source) error {
	if err := s.Create(ctx, meal, sources); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE meals SET parent_meal_id = ? WHERE id = ?`, parentID, meal.ID); err != nil {
		// Don't leave an orphaned, un-linked copy behind.
		_, _ = s.db.ExecContext(ctx, `DELETE FROM meals WHERE id = ?`, meal.ID)
		return err
	}
	meal.ParentMealID = &parentID
	return nil
}

// loadLineage fills ParentMealID, ParentName and ForkCount for each meal.
func (s *MealStore) loadLineage(ctx context.Context, meals []models.Meal) error {
	if len(meals) == 0 {
		return nil
	}
	ids := make([]any, len(meals))
	idx := make(map[string]int, len(meals))
	for i, m := range meals {
		ids[i] = m.ID
		idx[m.ID] = i
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.parent_meal_id, COALESCE(p.name, ''),
		       (SELECT COUNT(*) FROM meals f WHERE f.parent_meal_id = m.id)
		FROM meals m
		LEFT JOIN meals p ON p.id = m.parent_meal_id
		WHERE m.id IN (`+ph+`)`, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, parentName string
		var parentID sql.NullString
		var forks int
		if err := rows.Scan(&id, &parentID, &parentName, &forks); err != nil {
			return err
		}
		i := idx[id]
		if parentID.Valid {
			p := parentID.String
			meals[i].ParentMealID = &p
		}
		meals[i].ParentName = parentName
		meals[i].ForkCount = forks
	}
	return rows.Err()
}

// ListForks returns the id and name of each direct fork of a meal, oldest first.
func (s *MealStore) ListForks(ctx context.Context, parentID string) ([]models.Meal, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name FROM meals WHERE parent_meal_id = ? ORDER BY created_at, rowid`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Meal
	for rows.Next() {
		var m models.Meal
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
