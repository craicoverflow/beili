-- AI-derived, shopping-list-friendly item names, one per ingredient (same
-- length/order as ingredients when populated). Generated once at recipe
-- import/save time so "Add to shopping list" doesn't need to parse prep
-- notes out of the raw ingredient line on every push. Empty/short arrays
-- mean "not generated yet" -- callers fall back to regex parsing of the raw
-- ingredient string.
ALTER TABLE meals ADD COLUMN shopping_names TEXT NOT NULL DEFAULT '[]';
