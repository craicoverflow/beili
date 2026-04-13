-- Core meal record
CREATE TABLE meals (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    meal_types  TEXT NOT NULL DEFAULT '[]',  -- JSON array of MealType strings
    cuisine     TEXT NOT NULL DEFAULT '',
    prep_time   INTEGER,                      -- minutes, nullable
    cook_time   INTEGER,                      -- minutes, nullable
    servings    INTEGER,                      -- nullable
    ingredients TEXT NOT NULL DEFAULT '[]',  -- JSON array of strings
    rating      INTEGER,                      -- 1-5, nullable
    notes       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- Sources: one or more per meal (URL, book, YouTube, other)
CREATE TABLE sources (
    id             TEXT PRIMARY KEY,
    meal_id        TEXT NOT NULL REFERENCES meals(id) ON DELETE CASCADE,
    type           TEXT NOT NULL,             -- url | book | youtube | other
    title          TEXT NOT NULL DEFAULT '',
    url            TEXT NOT NULL DEFAULT '',
    page_reference TEXT NOT NULL DEFAULT '',  -- e.g. "p.47, The Ottolenghi Cookbook"
    notes          TEXT NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL
);

-- Weekly meal plan entries
CREATE TABLE meal_plan (
    id          TEXT PRIMARY KEY,
    date        TEXT NOT NULL,               -- YYYY-MM-DD
    meal_type   TEXT NOT NULL,               -- breakfast | lunch | dinner | snack | side | other
    meal_id     TEXT REFERENCES meals(id) ON DELETE SET NULL,
    custom_meal TEXT NOT NULL DEFAULT '',    -- free-text if no meal_id
    notes       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL
);

CREATE INDEX idx_meal_plan_date ON meal_plan(date);
CREATE INDEX idx_sources_meal_id ON sources(meal_id);

-- FTS5 virtual table for full-text search across meals
CREATE VIRTUAL TABLE meals_fts USING fts5(
    id UNINDEXED,
    name,
    description,
    ingredients,
    cuisine,
    content='meals',
    content_rowid='rowid'
);

-- Keep FTS in sync with meals table
CREATE TRIGGER meals_fts_insert AFTER INSERT ON meals BEGIN
    INSERT INTO meals_fts(rowid, id, name, description, ingredients, cuisine)
    VALUES (new.rowid, new.id, new.name, new.description, new.ingredients, new.cuisine);
END;

CREATE TRIGGER meals_fts_update AFTER UPDATE ON meals BEGIN
    DELETE FROM meals_fts WHERE rowid = old.rowid;
    INSERT INTO meals_fts(rowid, id, name, description, ingredients, cuisine)
    VALUES (new.rowid, new.id, new.name, new.description, new.ingredients, new.cuisine);
END;

CREATE TRIGGER meals_fts_delete AFTER DELETE ON meals BEGIN
    DELETE FROM meals_fts WHERE rowid = old.rowid;
END;
