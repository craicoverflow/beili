package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/craicoverflow/beili/internal/models"
	"github.com/google/uuid"
)

// ShoppingExtrasStore handles ad-hoc (non-meal) shopping list items.
type ShoppingExtrasStore struct {
	db *sql.DB
}

// NewShoppingExtrasStore creates a new ShoppingExtrasStore.
func NewShoppingExtrasStore(db *sql.DB) *ShoppingExtrasStore {
	return &ShoppingExtrasStore{db: db}
}

// ListForWeek returns all extras for a given ISO week, oldest first.
func (s *ShoppingExtrasStore) ListForWeek(ctx context.Context, week string) ([]models.ShoppingExtra, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, week, text, checked, created_at
		FROM shopping_extras
		WHERE week = ?
		ORDER BY created_at`,
		week,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var extras []models.ShoppingExtra
	for rows.Next() {
		var e models.ShoppingExtra
		if err := rows.Scan(&e.ID, &e.Week, &e.Text, &e.Checked, &e.CreatedAt); err != nil {
			return nil, err
		}
		extras = append(extras, e)
	}
	return extras, rows.Err()
}

// Add inserts a new extra item for the given week.
func (s *ShoppingExtrasStore) Add(ctx context.Context, week, text string) (*models.ShoppingExtra, error) {
	e := &models.ShoppingExtra{
		ID:        uuid.New().String(),
		Week:      week,
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO shopping_extras (id, week, text, checked, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.Week, e.Text, e.Checked, e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// SetChecked updates the checked state of a single extra item.
func (s *ShoppingExtrasStore) SetChecked(ctx context.Context, id string, checked bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE shopping_extras SET checked = ? WHERE id = ?`, checked, id)
	return err
}

// Remove deletes a single extra item by ID.
func (s *ShoppingExtrasStore) Remove(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM shopping_extras WHERE id = ?`, id)
	return err
}

// ClearWeek deletes all extras for a given week (used by "Clear all checks").
func (s *ShoppingExtrasStore) ClearChecked(ctx context.Context, week string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM shopping_extras WHERE week = ? AND checked = 1`, week)
	return err
}
