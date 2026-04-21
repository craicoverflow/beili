# Feature: Tag/Ingredient Search with AND/OR Logic

## Problem
Users want to search meals by ingredient or freeform tag using complex boolean logic — e.g. find meals containing "chicken" AND "broccoli", or "chicken" OR "beef". This is useful for fridge-based planning or protein filtering.

## Current State
- FTS5 index already covers `ingredients` JSON column and all text fields
- `sanitizeFTSQuery()` in `internal/store/meal_store.go` strips FTS5 operators and adds prefix matching — intentionally simple
- The search bar is a single text input; no boolean UI exists
- Meal list filters (`meal_type`, `min_rating`) are separate from the search path

## Approach

### 1. Query parser (`internal/search/parser.go`)
Write a parser that converts user input into valid FTS5 boolean expressions:
- Whitespace-separated terms → AND by default (`chicken broccoli` → `chicken* AND broccoli*`)
- `OR` keyword → FTS5 OR (`chicken OR beef`)
- Quoted phrases → exact match (`"olive oil"`)
- Parentheses for grouping → pass through to FTS5 (`(chicken OR beef) AND broccoli`)
- Strip unsafe chars, preserve structure

### 2. Update `sanitizeFTSQuery()` → replace with new parser
`internal/store/meal_store.go:328` — swap call to new parser.

### 3. Search UI — chip-based query builder
In `internal/templates/meals/list.templ`:
- Replace plain search input with a chip builder
- Each chip = one term; click a chip border to toggle AND/OR with the next chip
- "Add term" input appends a chip
- Chips serialize to a query string the parser understands
- HTMX `hx-get` triggers on chip add/remove/toggle (same as existing filter chips)

### 4. No schema changes needed
FTS5 already indexes all relevant columns. No migrations required.

## Files to Modify
- `internal/store/meal_store.go` — replace `sanitizeFTSQuery()`
- `internal/handlers/search.go` — pass parsed query
- `internal/handlers/meals.go` — wire search into list handler if unified
- `internal/templates/meals/list.templ` — chip UI
- New: `internal/search/parser.go`

## Verification
- Search "chicken broccoli" returns only meals with both
- Search "chicken OR beef" returns meals with either
- Search `"olive oil"` matches exact phrase
- Existing meal type / rating filters still work alongside search
- FTS5 injection attempts (e.g. `" OR 1=1`) are sanitized
