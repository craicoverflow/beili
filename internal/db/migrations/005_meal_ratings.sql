CREATE TABLE IF NOT EXISTS meal_ratings (
    meal_id    TEXT NOT NULL REFERENCES meals(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL,
    rating     INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (meal_id, user_id)
);
