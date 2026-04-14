package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// MealType represents the category of a meal.
type MealType string

const (
	MealTypeBreakfast MealType = "breakfast"
	MealTypeLunch     MealType = "lunch"
	MealTypeDinner    MealType = "dinner"
	MealTypeSnack     MealType = "snack"
	MealTypeSide      MealType = "side"
	MealTypeOther     MealType = "other"
)

// AllMealTypes is the ordered list used in UI rendering.
var AllMealTypes = []MealType{
	MealTypeBreakfast,
	MealTypeLunch,
	MealTypeDinner,
	MealTypeSnack,
	MealTypeSide,
	MealTypeOther,
}

// MealTypeLabel returns a display-friendly label for a MealType.
func (m MealType) Label() string {
	switch m {
	case MealTypeBreakfast:
		return "Breakfast"
	case MealTypeLunch:
		return "Lunch"
	case MealTypeDinner:
		return "Dinner"
	case MealTypeSnack:
		return "Snack"
	case MealTypeSide:
		return "Side"
	default:
		return "Other"
	}
}

// MealTypes is a []MealType that transparently marshals to/from a JSON TEXT
// column in SQLite.
type MealTypes []MealType

func (mt MealTypes) Value() (driver.Value, error) {
	b, err := json.Marshal(mt)
	return string(b), err
}

func (mt *MealTypes) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	case nil:
		*mt = MealTypes{}
		return nil
	default:
		return fmt.Errorf("MealTypes.Scan: unsupported type %T", src)
	}
	return json.Unmarshal([]byte(s), mt)
}

// StringList is a []string that transparently marshals to/from a JSON TEXT
// column in SQLite (used for ingredients).
type StringList []string

func (sl StringList) Value() (driver.Value, error) {
	b, err := json.Marshal(sl)
	return string(b), err
}

func (sl *StringList) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	case nil:
		*sl = StringList{}
		return nil
	default:
		return fmt.Errorf("StringList.Scan: unsupported type %T", src)
	}
	return json.Unmarshal([]byte(s), sl)
}

// Meal is the core domain object.
type Meal struct {
	ID          string
	Name        string
	Description string
	MealTypes   MealTypes
	Cuisine     string
	PrepTime    *int // minutes, nullable
	CookTime    *int // minutes, nullable
	Servings    *int // nullable
	Ingredients  StringList
	Instructions StringList
	Rating       *int // 1-5, nullable
	Notes       string
	Sources     []Source // populated by join, not stored in meals table
	LastCooked  *string  // YYYY-MM-DD, populated from cooked_log
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TotalTime returns the sum of prep and cook time in minutes, or nil if both
// are unset.
func (m *Meal) TotalTime() *int {
	if m.PrepTime == nil && m.CookTime == nil {
		return nil
	}
	total := 0
	if m.PrepTime != nil {
		total += *m.PrepTime
	}
	if m.CookTime != nil {
		total += *m.CookTime
	}
	return &total
}

// ImportResult holds the outcome of a meal import operation.
type ImportResult struct {
	Imported int
	Skipped  []string // names skipped due to duplicate
	Errors   []string // per-meal errors
}

// FormatMinutes converts a minute count to a human-readable string like
// "1h 30m" or "45m".
func FormatMinutes(mins int) string {
	if mins <= 0 {
		return "0m"
	}
	h := mins / 60
	m := mins % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
