-- Free-text feedback on a meal ("that was good, but too sweet"), kept whether
-- or not it is ever sent to the AI.
CREATE TABLE meal_feedback (
    id         TEXT PRIMARY KEY,
    meal_id    TEXT NOT NULL REFERENCES meals(id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_meal_feedback_meal_id ON meal_feedback(meal_id);

-- A fork is a revised copy of a meal. Whether a meal is superseded is derived
-- from "has at least one fork", never stored, so deleting the last fork
-- restores the parent. Deleting a parent leaves its forks as standalone meals.
ALTER TABLE meals ADD COLUMN parent_meal_id TEXT REFERENCES meals(id) ON DELETE SET NULL;

CREATE INDEX idx_meals_parent_meal_id ON meals(parent_meal_id);
