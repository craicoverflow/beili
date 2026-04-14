-- Track when each meal was cooked
CREATE TABLE cooked_log (
    id         TEXT PRIMARY KEY,
    meal_id    TEXT NOT NULL REFERENCES meals(id) ON DELETE CASCADE,
    cooked_on  TEXT NOT NULL,    -- YYYY-MM-DD
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_cooked_log_meal_id ON cooked_log(meal_id);
