-- Per-entry servings override (NULL means "use the meal's default servings")
-- and a leftover flag so extra-day entries don't need new shopping ingredients.
ALTER TABLE meal_plan ADD COLUMN servings INTEGER;
ALTER TABLE meal_plan ADD COLUMN is_leftover INTEGER NOT NULL DEFAULT 0;
