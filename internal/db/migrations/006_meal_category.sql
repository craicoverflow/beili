-- Distinguishes "main" recipes from secondary collections like "toddler".
-- Toddler recipes are excluded from the default meals list, the plan calendar,
-- and the random picker unless explicitly requested.
ALTER TABLE meals ADD COLUMN category TEXT NOT NULL DEFAULT 'main';

CREATE INDEX idx_meals_category ON meals(category);
