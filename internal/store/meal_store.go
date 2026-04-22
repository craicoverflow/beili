package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/search"
	"github.com/google/uuid"
)

// loadLastCooked is defined in cooked.go (same package).

// MealStore handles all database operations for meals.
type MealStore struct {
	db *sql.DB
}

// NewMealStore creates a new MealStore backed by the given database.
func NewMealStore(db *sql.DB) *MealStore {
	return &MealStore{db: db}
}

// ListFilters controls optional filtering on the meal list.
type ListFilters struct {
	Query     string // full-text search query
	MealType  string // filter by a single meal type
	MinRating int    // 0 = no filter (filters by average rating)
	UserID    string // for loading per-user rating
	Limit     int    // 0 = no limit (used by Page())
	Offset    int    // used by Page()
}

// List returns all meals (unpaginated) ordered by most recently updated, with
// sources and last-cooked dates loaded. Use Page() for paginated access.
func (s *MealStore) List(ctx context.Context, filters ListFilters) ([]models.Meal, error) {
	var (
		rows *sql.Rows
		err  error
	)

	// Ensure no pagination leaks into the all-results path.
	f := filters
	f.Limit = 0
	f.Offset = 0

	if f.Query != "" {
		rows, err = s.search(ctx, f)
	} else {
		rows, err = s.listFiltered(ctx, f)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meals, err := scanMeals(rows)
	if err != nil {
		return nil, err
	}
	if err := s.loadSources(ctx, meals); err != nil {
		return nil, err
	}
	if err := s.loadLastCooked(ctx, meals); err != nil {
		return nil, err
	}
	if err := s.loadRatings(ctx, meals, f.UserID); err != nil {
		return nil, err
	}
	return meals, nil
}

// Page returns a paginated slice of meals. hasMore is true when additional
// pages exist beyond filters.Offset + filters.Limit.
func (s *MealStore) Page(ctx context.Context, filters ListFilters) ([]models.Meal, bool, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 24
	}

	// Fetch one extra row to detect whether a next page exists.
	f := filters
	f.Limit = limit + 1

	var (
		rows *sql.Rows
		err  error
	)
	if f.Query != "" {
		rows, err = s.search(ctx, f)
	} else {
		rows, err = s.listFiltered(ctx, f)
	}
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	meals, err := scanMeals(rows)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(meals) > limit
	if hasMore {
		meals = meals[:limit]
	}

	if err := s.loadSources(ctx, meals); err != nil {
		return nil, false, err
	}
	if err := s.loadLastCooked(ctx, meals); err != nil {
		return nil, false, err
	}
	if err := s.loadRatings(ctx, meals, f.UserID); err != nil {
		return nil, false, err
	}
	return meals, hasMore, nil
}

// Random returns a single random meal matching the given filters, or
// sql.ErrNoRows if no meals match.
func (s *MealStore) Random(ctx context.Context, f ListFilters) (*models.Meal, error) {
	query := `SELECT id, name, description, meal_types, cuisine,
	                 prep_time, cook_time, servings, ingredients, instructions, image_url, notes,
	                 created_at, updated_at
	          FROM meals`
	var conds []string
	var args []any

	if f.MealType != "" {
		conds = append(conds, `meal_types LIKE ?`)
		args = append(args, "%\""+f.MealType+"\"%")
	}
	if f.MinRating > 0 {
		conds = append(conds, `(SELECT AVG(rating) FROM meal_ratings WHERE meal_id = id) >= ?`)
		args = append(args, f.MinRating)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY RANDOM() LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)
	meal, err := scanMeal(row)
	if err != nil {
		return nil, err
	}

	meals := []models.Meal{*meal}
	if err := s.loadSources(ctx, meals); err != nil {
		return nil, err
	}
	if err := s.loadLastCooked(ctx, meals); err != nil {
		return nil, err
	}
	if err := s.loadRatings(ctx, meals, f.UserID); err != nil {
		return nil, err
	}
	meal.Sources = meals[0].Sources
	meal.LastCooked = meals[0].LastCooked
	meal.AverageRating = meals[0].AverageRating
	meal.RatingCount = meals[0].RatingCount
	meal.UserRating = meals[0].UserRating
	return meal, nil
}

// GetByID returns a single meal with its sources, or sql.ErrNoRows if not found.
// Pass userID to populate UserRating; pass "" to skip.
func (s *MealStore) GetByID(ctx context.Context, id string, userID string) (*models.Meal, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, meal_types, cuisine,
		       prep_time, cook_time, servings, ingredients, instructions, image_url, notes,
		       created_at, updated_at
		FROM meals WHERE id = ?`, id)

	meal, err := scanMeal(row)
	if err != nil {
		return nil, err
	}

	meals := []models.Meal{*meal}
	if err := s.loadSources(ctx, meals); err != nil {
		return nil, err
	}
	if err := s.loadLastCooked(ctx, meals); err != nil {
		return nil, err
	}
	if err := s.loadRatings(ctx, meals, userID); err != nil {
		return nil, err
	}
	meal.Sources = meals[0].Sources
	meal.LastCooked = meals[0].LastCooked
	meal.AverageRating = meals[0].AverageRating
	meal.RatingCount = meals[0].RatingCount
	meal.UserRating = meals[0].UserRating
	return meal, nil
}

// Create inserts a new meal and its sources in a single transaction.
func (s *MealStore) Create(ctx context.Context, meal *models.Meal, sources []models.Source) error {
	meal.ID = uuid.New().String()
	now := time.Now().UTC()
	meal.CreatedAt = now
	meal.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO meals (id, name, description, meal_types, cuisine,
		                   prep_time, cook_time, servings, ingredients, instructions, image_url, notes,
		                   created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		meal.ID, meal.Name, meal.Description, meal.MealTypes, meal.Cuisine,
		meal.PrepTime, meal.CookTime, meal.Servings, meal.Ingredients, meal.Instructions, meal.ImageURL, meal.Notes,
		meal.CreatedAt, meal.UpdatedAt,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("insert meal: %w", err)
	}

	for i := range sources {
		sources[i].ID = uuid.New().String()
		sources[i].MealID = meal.ID
		sources[i].CreatedAt = now
		if err := insertSource(ctx, tx, &sources[i]); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// Update replaces a meal's fields and re-syncs its sources in a transaction.
func (s *MealStore) Update(ctx context.Context, meal *models.Meal, sources []models.Source) error {
	meal.UpdatedAt = time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE meals SET
			name = ?, description = ?, meal_types = ?, cuisine = ?,
			prep_time = ?, cook_time = ?, servings = ?, ingredients = ?,
			instructions = ?, image_url = ?, notes = ?, updated_at = ?
		WHERE id = ?`,
		meal.Name, meal.Description, meal.MealTypes, meal.Cuisine,
		meal.PrepTime, meal.CookTime, meal.Servings, meal.Ingredients,
		meal.Instructions, meal.ImageURL, meal.Notes, meal.UpdatedAt,
		meal.ID,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update meal: %w", err)
	}

	// Replace all sources
	if _, err := tx.ExecContext(ctx, `DELETE FROM sources WHERE meal_id = ?`, meal.ID); err != nil {
		tx.Rollback()
		return fmt.Errorf("delete sources: %w", err)
	}

	now := time.Now().UTC()
	for i := range sources {
		sources[i].ID = uuid.New().String()
		sources[i].MealID = meal.ID
		sources[i].CreatedAt = now
		if err := insertSource(ctx, tx, &sources[i]); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// UpsertUserRating sets or clears a user's rating for a meal (rating=0 deletes).
func (s *MealStore) UpsertUserRating(ctx context.Context, mealID, userID string, rating int) error {
	if rating == 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM meal_ratings WHERE meal_id = ? AND user_id = ?`, mealID, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO meal_ratings (meal_id, user_id, rating, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(meal_id, user_id) DO UPDATE SET rating = excluded.rating, updated_at = excluded.updated_at`,
		mealID, userID, rating, time.Now().UTC(),
	)
	return err
}

// GetBySourceURL returns the meal that has a source with the given URL, or
// sql.ErrNoRows if no match is found.
func (s *MealStore) GetBySourceURL(ctx context.Context, rawURL string) (*models.Meal, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT m.id, m.name, m.description, m.meal_types, m.cuisine,
		       m.prep_time, m.cook_time, m.servings, m.ingredients, m.instructions, m.image_url, m.notes,
		       m.created_at, m.updated_at
		FROM meals m
		JOIN sources s ON s.meal_id = m.id
		WHERE s.url = ?
		LIMIT 1`, rawURL)
	return scanMeal(row)
}

// Delete removes a meal (sources are cascade-deleted by the DB).
func (s *MealStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM meals WHERE id = ?`, id)
	return err
}

// --- internal helpers ---

func (s *MealStore) listFiltered(ctx context.Context, f ListFilters) (*sql.Rows, error) {
	query := `SELECT id, name, description, meal_types, cuisine,
	                 prep_time, cook_time, servings, ingredients, instructions, image_url, notes,
	                 created_at, updated_at
	          FROM meals`
	var conds []string
	var args []any

	if f.MealType != "" {
		conds = append(conds, `meal_types LIKE ?`)
		args = append(args, "%\""+f.MealType+"\"%")
	}
	if f.MinRating > 0 {
		conds = append(conds, `(SELECT AVG(rating) FROM meal_ratings WHERE meal_id = id) >= ?`)
		args = append(args, f.MinRating)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY updated_at DESC"

	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, f.Offset)
	}

	return s.db.QueryContext(ctx, query, args...)
}

func (s *MealStore) search(ctx context.Context, f ListFilters) (*sql.Rows, error) {
	safe := search.ParseFTSQuery(f.Query)
	query := `SELECT m.id, m.name, m.description, m.meal_types, m.cuisine,
		       m.prep_time, m.cook_time, m.servings, m.ingredients, m.instructions, m.image_url, m.notes,
		       m.created_at, m.updated_at
		FROM meals m
		JOIN meals_fts ff ON m.id = ff.id
		WHERE meals_fts MATCH ?`
	args := []any{safe}

	if f.MealType != "" {
		query += ` AND m.meal_types LIKE ?`
		args = append(args, "%\""+f.MealType+"\"%")
	}
	if f.MinRating > 0 {
		query += ` AND (SELECT AVG(rating) FROM meal_ratings WHERE meal_id = m.id) >= ?`
		args = append(args, f.MinRating)
	}

	query += " ORDER BY rank"
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, f.Offset)
	}
	return s.db.QueryContext(ctx, query, args...)
}

func (s *MealStore) loadSources(ctx context.Context, meals []models.Meal) error {
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
		`SELECT id, meal_id, type, title, url, page_reference, notes, created_at
		 FROM sources WHERE meal_id IN (`+placeholders+`) ORDER BY created_at`,
		ids...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var src models.Source
		if err := rows.Scan(
			&src.ID, &src.MealID, &src.Type, &src.Title,
			&src.URL, &src.PageReference, &src.Notes, &src.CreatedAt,
		); err != nil {
			return err
		}
		idx := idxByID[src.MealID]
		meals[idx].Sources = append(meals[idx].Sources, src)
	}
	return rows.Err()
}

func insertSource(ctx context.Context, tx *sql.Tx, s *models.Source) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sources (id, meal_id, type, title, url, page_reference, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.MealID, s.Type, s.Title, s.URL, s.PageReference, s.Notes, s.CreatedAt,
	)
	return err
}

// loadRatings batch-loads average rating, count, and user's own rating for a
// slice of meals. userID may be "" to skip per-user rating loading.
func (s *MealStore) loadRatings(ctx context.Context, meals []models.Meal, userID string) error {
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

	query := `SELECT meal_id, AVG(rating), COUNT(*),
	                 COALESCE(MAX(CASE WHEN user_id = ? THEN rating END), 0)
	          FROM meal_ratings WHERE meal_id IN (` + placeholders + `) GROUP BY meal_id`

	args := make([]any, 0, 1+len(ids))
	args = append(args, userID)
	args = append(args, ids...)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mealID string
		var avg float64
		var count, userRating int
		if err := rows.Scan(&mealID, &avg, &count, &userRating); err != nil {
			return err
		}
		if idx, ok := idxByID[mealID]; ok {
			meals[idx].AverageRating = avg
			meals[idx].RatingCount = count
			meals[idx].UserRating = userRating
		}
	}
	return rows.Err()
}

// scanMeals reads all rows from a meals SELECT query.
func scanMeals(rows *sql.Rows) ([]models.Meal, error) {
	var meals []models.Meal
	for rows.Next() {
		var m models.Meal
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Description, &m.MealTypes, &m.Cuisine,
			&m.PrepTime, &m.CookTime, &m.Servings, &m.Ingredients, &m.Instructions, &m.ImageURL, &m.Notes,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		meals = append(meals, m)
	}
	return meals, rows.Err()
}

// scanMeal reads a single meal from a QueryRow result.
func scanMeal(row *sql.Row) (*models.Meal, error) {
	var m models.Meal
	err := row.Scan(
		&m.ID, &m.Name, &m.Description, &m.MealTypes, &m.Cuisine,
		&m.PrepTime, &m.CookTime, &m.Servings, &m.Ingredients, &m.Instructions, &m.ImageURL, &m.Notes,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
