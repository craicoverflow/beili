package meals

import (
	"strings"

	"github.com/craicoverflow/beili/internal/models"
	"github.com/craicoverflow/beili/internal/revision"
)

// FeedbackData is everything the feedback section on a meal page needs.
type FeedbackData struct {
	MealID string
	Notes  []models.Feedback // newest first
	Forks  []models.Meal     // revised versions of this meal
	// CanRevise is false when no AI reviser is configured; notes can still be
	// saved.
	CanRevise bool
	// Saved is true right after a note was saved, to confirm it.
	Saved bool
}

// ReviseData is the review screen shown after asking the AI to revise a meal.
type ReviseData struct {
	Meal               *models.Meal
	Feedback           string
	Ingredients        []string
	Instructions       []string
	IngredientChanges  []revision.Change
	InstructionChanges []revision.Change
	Error              string
}

func joinLines(lines []string) string { return strings.Join(lines, "\n") }
