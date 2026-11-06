# Implementation Plan: Recipe feedback & fork

Source: `SPEC.md` (idea: `docs/ideas/recipe-feedback-fork.md`). Task list: `tasks/recipe-fork/todo.md`.
(Kept in a subfolder: `tasks/plan.md` and `tasks/todo.md` belong to the usability-report work.)

## Overview

Let the household describe what was wrong with a meal, have Sonnet propose a revised
recipe, review a server-computed diff, and save the result as a linked fork. Parents
with forks drop out of list/search/random/calendar. The riskiest unknown is whether the
AI's fixes are good enough, so that is tested first, before any UI is built.

## Architecture decisions

- **Superseded is derived** (`NOT EXISTS fork`), never stored. Deleting the last fork
  restores the parent with no extra code.
- **One shared SQL predicate** for the live-meals filter, used by `listFiltered`,
  `search` and `Random`. `GetByID` and `plan_store.GetRange` are NOT filtered
  (existing plan entries must still show a superseded meal).
- **`ListFilters.IncludeSuperseded`** (new, default false). Export and import set it
  true; otherwise backups would silently drop superseded recipes and import
  de-duplication would break. The plan calendar picker uses `List` (plan.go:87), so it
  is covered by the default exclusion.
- **Separate `Reviser` interface**, implemented only by the Anthropic provider. Gemini
  stays unchanged.
- **Server computes the diff** (`internal/diff`, hand-written, no dependency) from the
  original and revised lists; the model only returns the full revised lists, a name
  and per-change reasons.
- **Fork copies**: description, category, meal types, cuisine, times, servings, notes,
  image, sources, plus revised ingredients/instructions. **Clears** `shopping_names`
  (callers already fall back to regex parsing). Copies no ratings/cooked/feedback.
- **Synchronous revise** with an HTMX loading indicator; no job queue.

## Dependency graph

```
T1 Revise core (ai) ─────────────┐
T2 diff package ─────────────────┤
T3 migration 010 + feedback ─┬───┼─► T6 suggest-a-fix + diff UI ─► T7 save fork + lineage UI ─► T9 hardening
                             │   │
T4 fork creation (store) ────┴───┼─► T7
T5 superseded exclusion ─────────┘        (T5 needs T4 for test data)
                                                                   T8 delete/restore behaviour (T4,T5)
                                                                   T10 release
```

T1, T2 and T3 are independent and can run in parallel. T4 needs T3 (column). T5 needs T4.

## Phase 1: De-risk and foundation

### T1 (M): `Revise` core and quality probe
**Description:** Add the `Reviser` interface, request/response types, prompt (taste
reasoning: "too sweet" means cut sweetener and consider acid or salt, not only
quantities; stay metric and at original servings; include lineage feedback), parsing
with validation, and the Anthropic HTTP implementation. Add
`ANTHROPIC_REVISE_MODEL` config (Sonnet default; confirm ID with the claude-api skill).
**Acceptance:**
- [ ] Prompt includes current feedback plus all ancestor feedback in order, oldest first.
- [ ] Malformed, empty, or length-collapsed responses are rejected with an error.
- [ ] Request goes to the revise model, not `ANTHROPIC_MODEL`.
- [ ] Gemini provider compiles unchanged.
**Verify:** `go test ./internal/ai/...`; httptest for request shape and error statuses.
**Files:** `internal/ai/provider.go`, `revise.go`, `anthropic.go`, `revise_test.go`, `internal/config/config.go`.

### T2 (S): Line diff package
**Description:** Pure function comparing two `[]string` into added/removed/changed/
unchanged entries (reorders handled sensibly).
**Acceptance:**
- [ ] Table tests: identical, add, remove, change, reorder, empty either side.
- [ ] No third-party dependency.
**Verify:** `go test ./internal/diff/...`
**Files:** `internal/diff/diff.go`, `diff_test.go`.

### Checkpoint A: quality probe (human gate)
- [ ] Throwaway script (scratchpad, **not committed**) sends ~10 real complaints
      against ~5 of your recipes through T1's `Revise`.
- [ ] You'd accept at least 7 of 10 unedited. If not, iterate on the prompt before
      continuing; do not build the review UI on a weak prompt.
- [ ] `go test ./...` green.

### T3 (M): Feedback capture, end to end
**Description:** Migration `010` (`meal_feedback`, `parent_meal_id` + index), models,
store methods (add, list for a meal, list for lineage), handler, and an "Improve this
recipe" section on the detail page with a textarea and a list of past feedback.
**Acceptance:**
- [ ] Submitting feedback stores it and re-renders the section via HTMX.
- [ ] Empty/whitespace-only feedback shows a message and stores nothing.
- [ ] Feedback works with no AI key configured.
- [ ] Feedback is removed when its meal is deleted (cascade).
**Verify:** `go test ./internal/store/... ./internal/handlers/...`; `make generate`;
manual: add feedback on a seeded meal.
**Files:** `010_recipe_feedback_fork.sql`, `models/feedback.go`, `models/meal.go`,
`store/feedback.go`, `handlers/feedback.go`, `templates/meals/detail.templ`, `cmd/server/main.go`
(Medium-large: stay at 5-7 files by keeping the store test in the existing file.)

## Phase 2: Fork and exclusion

### T4 (M): Create a fork in the store
**Description:** `MealStore.CreateFork(ctx, parentID, revised)` copies the carry-over
fields and sources, sets `parent_meal_id`, clears `shopping_names`, copies no
ratings/cooked/feedback. Add `GetLineageFeedback` ordering if not done in T3.
**Acceptance:**
- [ ] Fork has `parent_meal_id` set and the revised ingredients/instructions.
- [ ] Fork has zero ratings, zero cooked-log rows, zero feedback rows.
- [ ] Sources are copied with fresh IDs; `shopping_names` is empty.
- [ ] Forking a nonexistent parent returns `sql.ErrNoRows`.
**Verify:** `go test ./internal/store/... -run Fork`.
**Files:** `store/meal_store.go` (or `store/fork.go`), `store/store_test.go`, `models/meal.go`.

### T5 (M): Superseded exclusion across all query paths
**Description:** Shared predicate applied to `listFiltered`, `search` and `Random`;
new `ListFilters.IncludeSuperseded`; export and import set it true. Prove-It: write a
failing test for each path first.
**Acceptance:**
- [ ] Parent with a fork is absent from `List`, `Page`, search, and `Random`.
- [ ] `GetByID` and `plan_store.GetRange` still return it.
- [ ] Export includes superseded recipes; import de-dup still sees them.
- [ ] Deleting the only fork makes the parent reappear; deleting a parent leaves forks
      live with `parent_meal_id` NULL.
**Verify:** `go test ./internal/store/... ./internal/handlers/...`.
**Files:** `store/meal_store.go`, `handlers/export.go`, `store/store_test.go`, `handlers/export_test.go` (new).

### Checkpoint B: store behaviour
- [ ] All tests green; `make lint` clean.
- [ ] Using a seeded DB: fork a meal via a test helper and confirm the four views.

### T6 (M): "Suggest a fix" shows a diff
**Description:** Button on the feedback section posts to a revise endpoint: loads the
meal, gathers lineage feedback, calls `Revise`, computes the diff, and renders a review
fragment (changed/added/removed lines, per-change reasons, editable name and lists).
Nothing is saved yet. Hidden/disabled with an explanation if no Anthropic key.
**Acceptance:**
- [ ] Review screen shows the diff with unchanged lines dimmed.
- [ ] AI error shows a banner; the feedback text remains saved and visible.
- [ ] Loading indicator while the call runs.
- [ ] Endpoint requires the same auth/CSRF as other writes.
**Verify:** handler tests with a fake `Reviser`; manual with a real key via `make dev`.
**Files:** `handlers/feedback.go`, `templates/meals/revise_review.templ`, `detail.templ`, `cmd/server/main.go`, `handlers/feedback_test.go`.

### T7 (M): Accept and save as fork; lineage UI
**Description:** Accept posts the (possibly edited) revision and name; handler calls
`CreateFork`, redirects to the new meal. Fork detail shows "Forked from X"; parent
detail lists its forks.
**Acceptance:**
- [ ] Saved fork opens at its own URL with the edited name.
- [ ] "Forked from <parent>" link on the fork; fork list on the parent.
- [ ] Fork scales via `GET /meals/{id}/scale` at another serving size.
- [ ] Parent no longer appears in list/random/calendar (verified in the browser).
**Verify:** handler + scaling test; manual end-to-end below.
**Files:** `handlers/feedback.go`, `templates/meals/detail.templ`, `store/meal_store.go`, `handlers/feedback_test.go`.

### Checkpoint C: end-to-end (human review)
- [ ] `make dev`: feedback on a stir fry, suggest, review diff, save fork, confirm the
      original vanishes from list/random/plan picker, open it by URL, delete the fork,
      confirm the original returns.
- [ ] Fork of a fork: second revision's prompt contains both feedback items.
- [ ] `go test ./...`, `make lint`, `make generate` leaves no diff.

## Phase 3: Harden and ship

### T8 (S): Delete and edge behaviour
**Description:** Confirm UX when deleting a fork (parent restored message) or a parent
that has forks (forks become roots); tests for both.
**Acceptance:** [ ] matches the Spec's delete criteria; [ ] no dangling `parent_meal_id`.
**Verify:** `go test ./internal/store/... ./internal/handlers/...`.
**Files:** `handlers/meals.go`, `templates/meals/detail.templ`, tests.

### T9 (S): Hardening
**Description:** Size-limit feedback text (and cap lineage feedback at the 10 most
recent); validate AI output sizes; ensure no secrets in logs; confirm HA ingress base
path on new URLs.
**Acceptance:** [ ] oversized feedback rejected; [ ] works under `INGRESS_PATH`;
[ ] key never logged.
**Verify:** tests + manual with `SUPERVISOR_TOKEN` set.
**Files:** `handlers/feedback.go`, `ai/revise.go`, tests.

### T10 (XS): Release
**Description:** Bump `addon/config.yaml`, tag to match, per project rule.
**Acceptance:** [ ] version bumped; [ ] tag `vX.Y.Z` exists before push.
**Verify:** `git tag --list`; `make release VERSION=x.y.z` only on your go-ahead.

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| AI fixes are generic or wrong | High | Quality probe gate at Checkpoint A before any UI |
| A query path misses the exclusion | High | Prove-It test per path (T5); shared predicate |
| Export/import silently drops superseded meals | High | `IncludeSuperseded` plus export test (T5) |
| Revised ingredients break scaling/parsing | Med | Fork scale test (T7); validate through `internal/ingredient` |
| SQLite `ADD COLUMN` + FK/FTS interaction | Med | Migration test opens a fresh DB and an upgraded one |
| Forks pile up | Low | Deferred; revisit after a month of use |
| Sonnet cost/latency | Low | Occasional use; synchronous with a loading state |

## Open questions

- Confirm the current Sonnet model ID (T1).
- `shopping_names` cleared on fork rather than regenerated: confirm.
- Should the existing working-tree changes (servings work, `tasks/`) be committed or
  stashed before starting, so this feature gets a clean branch?
