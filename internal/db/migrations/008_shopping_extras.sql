-- Ad-hoc shopping list items that aren't tied to a planned meal (e.g. "paper
-- towels"). Scoped to an ISO week string (e.g. "2025-W15") to match how the
-- shopping list itself is scoped.
CREATE TABLE shopping_extras (
    id         TEXT PRIMARY KEY,
    week       TEXT NOT NULL,
    text       TEXT NOT NULL,
    checked    INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_shopping_extras_week ON shopping_extras(week);
