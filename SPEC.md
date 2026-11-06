# Spec: Recipe feedback & fork

Source idea: `docs/ideas/recipe-feedback-fork.md`

## Objective

After cooking a meal, the user writes what was off ("too sweet") in a free-text
box. Béilí asks Claude (Sonnet) to revise the recipe, shows a **diff** of the
proposed changes, and on acceptance saves a **new meal** (a "fork") linked to the
original. Over time a household's go-to recipes get tuned to their taste.

**Users:** the household using the web UI (and HA add-on). Occasional use, a
handful of recipes, one person usually giving the feedback.

**Success looks like:** "too sweet" on a stir fry produces a reviewable, correctly
scaled fork in under ~20 s, the original drops out of the picker and calendar, and
feedback is never lost even when the AI is unavailable.

### Behaviour

1. **Feedback is always saved.** The detail page has an "Improve this recipe"
   section with a textarea. Submitting stores a `meal_feedback` row against that
   meal. No AI is involved at this step.
2. **Revise on request.** A "Suggest a fix" action sends the recipe, the new
   feedback, and feedback from the meal's whole lineage (parent, grandparent...)
   to `Provider.Revise`. The AI sees earlier feedback so it doesn't undo earlier fixes.
3. **Diff review.** The response is a complete revised ingredient list and
   instruction list, a suggested name, and a one-line reason per change. The
   **server computes the diff** by comparing revised lists to the original; the
   model does not describe its own edits. The user can edit the result and the
   name before saving.
4. **Save as fork.** Creates a new meal with `parent_meal_id` set to the
   original. The fork starts fresh: **no ratings, cooked-log entries, or feedback
   are copied.** Everything else (description, meal types, category, cuisine,
   times, servings, notes, image, sources) is carried over, along with the revised
   ingredients and instructions.
5. **Superseding.** A meal with at least one fork is *superseded*. This is
   **derived** (`EXISTS fork`), never stored. Superseded meals are excluded from
   the meal list, search, random picker and plan calendar. They remain reachable by
   direct URL and via the "forked from" link. Deleting the last fork makes the
   parent live again automatically. There is no undo or override in v1.
6. **Lineage UI.** A fork shows "Forked from <parent>". A parent shows its forks.
7. **Servings and units.** The revised recipe stays at the original's serving
   count and in metric units. Quantity scaling remains the job of
   `internal/scaling`; no client-side quantity parsing.

### Acceptance criteria

- [ ] Submitting empty or whitespace-only feedback is rejected with a visible message.
- [ ] Feedback is persisted even if the AI call fails or no provider is configured.
- [ ] With no Anthropic key configured, the "Suggest a fix" action is hidden or
      disabled with an explanation; feedback capture still works.
- [ ] `Revise` output that is malformed, empty, or whose lists are shorter than
      a sane minimum is rejected with an error banner; nothing is saved.
- [ ] The diff marks added, removed and changed lines; unchanged lines are shown dimmed.
- [ ] A saved fork has `parent_meal_id` = original, zero ratings, zero cooked-log
      rows, and its own `shopping_names` (regenerated or cleared; never stale).
- [ ] After forking, the original no longer appears in list, search, random, or
      plan-calendar selection, but `GET /meals/{originalID}` still renders.
- [ ] Deleting the only fork makes the original appear in those views again.
- [ ] Deleting the *parent* does not delete forks (they become roots: `parent_meal_id`
      set to NULL).
- [ ] Revising a fork of a fork passes feedback from every ancestor to the prompt.
- [ ] Scaling a fork on the detail page via `GET /meals/{id}/scale` still works.
- [ ] Revise and fork endpoints require the same auth/CSRF as other write endpoints.

## Tech stack

Existing; no new dependencies.

- Go, chi router, templ templates, HTMX, Tailwind (standalone CLI)
- SQLite via `modernc.org/sqlite` (pure Go, **no CGO**)
- AI: Anthropic Messages API over plain `net/http` (as in `internal/ai/anthropic.go`;
  no SDK). Default model for `Revise`: Sonnet, configured separately from the
  Haiku import model.

## Commands

```
Generate templates:  make generate
Build:               make build
Dev (live reload):   make dev
Test (all):          go test ./...
Test (focused):      go test ./internal/store/... ./internal/ai/... ./internal/handlers/...
Lint:                make lint
CSS (after template changes that add classes): make css
Release:             make release VERSION=x.y.z   (bumps addon/config.yaml, tags, pushes)
```

New config (read in `internal/config/config.go`, alongside the existing AI settings):

```
ANTHROPIC_REVISE_MODEL   default: the current Sonnet model ID from the claude-api skill
```

## Project structure

```
internal/db/migrations/010_recipe_feedback_fork.sql  → NEW migration (never edit 001-009)
internal/models/meal.go        → add ParentMealID *string, Forks/ParentName display fields
internal/models/feedback.go    → NEW Feedback struct
internal/store/meal_store.go   → superseded filter in listFiltered/search/Random; fork helpers
internal/store/feedback.go     → NEW feedback store methods (CRUD, lineage query)
internal/ai/provider.go        → add Revise to the interface (or a separate Reviser interface)
internal/ai/revise.go          → NEW prompt, request/response types, parsing, validation
internal/ai/anthropic.go       → Anthropic Revise implementation (HTTP)
internal/diff/                 → NEW pure line-diff of two string lists (no deps)
internal/handlers/feedback.go  → NEW: POST feedback, POST revise, POST fork
internal/templates/meals/      → detail.templ section; NEW revise_review.templ (diff UI)
cmd/server/main.go             → wire handlers and routes
docs/ideas/recipe-feedback-fork.md → source idea (already written)
```

Design decision on the interface: `Standardize` and `Revise` do different jobs and
Gemini will not implement `Revise`. Prefer a separate `Reviser` interface so the
Gemini provider needn't stub a method. Handlers depend on structs/interfaces as the
code does today; no new store abstraction layer.

## Data model (migration 010)

```sql
-- Free-text feedback from the household, kept even if never turned into a fork.
CREATE TABLE meal_feedback (
    id         TEXT PRIMARY KEY,
    meal_id    TEXT NOT NULL REFERENCES meals(id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX idx_meal_feedback_meal_id ON meal_feedback(meal_id);

-- Lineage. SET NULL so deleting a parent leaves its forks as standalone recipes.
ALTER TABLE meals ADD COLUMN parent_meal_id TEXT REFERENCES meals(id) ON DELETE SET NULL;
CREATE INDEX idx_meals_parent_meal_id ON meals(parent_meal_id);
```

SQLite `ALTER TABLE ADD COLUMN` with a foreign key requires a NULL default, which
this has. Verify FTS5 triggers/tables are unaffected (the new column is not indexed
for search).

Superseded predicate used in every "live meals" query:

```sql
NOT EXISTS (SELECT 1 FROM meals f WHERE f.parent_meal_id = meals.id)
```

Careful: `listFiltered`, `search` and `Random` build SQL independently, so the
predicate must be added to **all three** plus the plan calendar's meal source. A
shared helper is preferred over four copies. `GetByID` must **not** apply it.

## Code style

Match the surrounding Go: doc comments on exported identifiers, errors wrapped with
`fmt.Errorf("...: %w", err)`, handlers use `respondError`/`respondNotFound`, store
methods take `ctx` first, and structs rather than interfaces for the store.

```go
// LogCooked records today as a cook date for the meal.
func (s *MealStore) LogCooked(ctx context.Context, mealID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cooked_log (id, meal_id, cooked_on, created_at) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), mealID, time.Now().UTC().Format("2006-01-02"), time.Now().UTC())
	return err
}
```

- Edit `.templ` sources only; never hand-edit `*_templ.go`. Run `make generate`.
- Interactions use HTMX partial swaps. Do not add `hx-boost` to new routes.
- Ingredient quantities in AI output are metric (g, ml, kg, l); convert imperial.
- Keep `MealTypes` / `StringList` as the custom Valuer/Scanner types.

## Testing strategy

Framework: standard `testing`. Store tests use a real temp SQLite
(`openTestDB`, as in `internal/store/store_test.go`); AI tests use `httptest`
servers (as in `TestAnthropicStandardize`). No test calls the real API.

| Concern | Level | Location |
|---|---|---|
| Line diff of two string lists (add/remove/change/unchanged/reorder) | unit, table-driven | `internal/diff` |
| `Revise` prompt includes feedback from all ancestors, in order | unit | `internal/ai` |
| `Revise` response parsing and rejection of malformed output | unit | `internal/ai` |
| Anthropic `Revise` request shape, model, error statuses | unit w/ httptest | `internal/ai` |
| Superseded exclusion in list, search, random, plan | integration (real SQLite) | `internal/store` |
| Deleting last fork un-supersedes parent; deleting parent nulls fork parent | integration | `internal/store` |
| Fork creation copies the right fields and none of ratings/cooked/feedback | integration | `internal/store` |
| Feedback saved when AI errors or is unconfigured | handler test | `internal/handlers` |
| Fork scales correctly at another serving size | integration | `internal/scaling` / handlers |

Follow TDD: write the failing test first, and for the exclusion bug class use the
Prove-It pattern (a test that fails for each of the four query paths).

Non-automated check before relying on it: run the **quality probe**, a throwaway
script (not committed) that sends ~10 real complaints against ~5 of your recipes
and records how many revisions you'd accept unedited. Target: 7 of 10. If it
misses, fix the prompt before building the review UI.

## Boundaries

**Always**
- Run `go test ./...` and `make generate` before committing.
- Add new schema as `010_*.sql`; keep migrations embedded and in filename order.
- Bump `addon/config.yaml` and tag `vX.Y.Z` before pushing.
- Gate revise/fork endpoints behind the existing auth and CSRF middleware.
- Validate and size-limit feedback text and AI output before storing.
- Keep scaling in `internal/scaling`; AI output stays metric at the original servings.

**Ask first**
- Adding any dependency (diff library, SDK, etc.). Default: hand-write the diff.
- Changing existing migrations or the `Provider` / `Standardize` contract.
- Adding an undo/override for superseded meals, or a stored `superseded` column.
- Copying ratings, cooked history or feedback into forks.
- Changing which fields a fork inherits.

**Never**
- Hand-edit `*_templ.go` or modify migrations 001-009.
- Introduce CGO or client-side quantity parsing.
- Expose an unauthenticated write endpoint (the app is public via the Cloudflare tunnel).
- Log the Anthropic key or send other users' data to the model.
- Let a failed or malformed AI response lose the user's feedback text.
- Delete forks when their parent is deleted.

## Success criteria

- All acceptance criteria above pass as automated tests, except the quality probe.
- `go test ./...` and `make lint` are green; `make generate` leaves no diff.
- Manual run (`make dev`): give feedback on a seeded stir fry, review the diff,
  save the fork, confirm the original disappears from list/random/plan, delete the
  fork, confirm the original returns.
- Quality probe reaches 7 of 10 acceptable revisions before the review UI is built.

## Open questions

- Should "Suggest a fix" call the model with the lineage feedback truncated if it
  grows long? Proposed: cap at the most recent 10 feedback items.
- Regenerate `shopping_names` for the fork at save time (extra AI call), or clear
  them and let callers fall back to regex parsing? Proposed: clear, to keep the
  fork flow to one AI call.
- Sonnet model ID default: confirm against the claude-api skill before coding.
- Resolved: the plan calendar picker reuses `MealStore.List` (`handlers/plan.go`), so
  the default exclusion covers it. `plan_store.GetRange` is not filtered, so meals
  already scheduled still show.
- Resolved: export and import also call `List`; `ListFilters.IncludeSuperseded`
  (default false) is set true there so backups keep superseded recipes.
