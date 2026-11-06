# Implementation Plan: Usability Report Fixes

Source: `usability-report.md` (2026-07-02 Chrome test session).

## Overview

Fix the bugs and close the feature gaps found in the usability test, ordered so that
high-value correctness fixes land first, the risky infrastructure change (vendoring
CDN assets) lands early behind a visual check, and the schema-touching vertical slice
(servings → plan → shopping list) lands as one coherent path. Each task leaves the app
working; `make generate` + `go test ./...` + a driven browser check are the gates.

## Architecture decisions

- **Search fix stays client-side-thin**: the chip builder keeps building `q`, but
  instead of mutating `hx-get` on a processed button (htmx caches the path at init —
  the root cause), call `htmx.ajax('GET', url, {target:'#meal-grid', pushUrl:true})`.
  No server changes: `HandleList` already honours `q` via FTS.
- **Vendor assets, don't add a JS build**: compile Tailwind once with the standalone
  CLI into `static/app.css` (checked in, rebuilt via `make css`), download htmx to
  `static/htmx.min.js`, serve both from an embedded `//go:embed static` FS. No Node
  toolchain in the runtime image.
- **Servings on the plan entry, not the meal**: add `servings INTEGER NULL` to
  `meal_plan_entries` (new migration `007_...`). NULL means "meal default". The
  shopping list scales with the existing deterministic `internal/scaling` package —
  never AI at shopping-list time.
- **Leftovers become first-class**: `is_leftover` flag on plan entries (same 007
  migration) instead of inferring nothing; badge in `day_cell.templ`, skipped by the
  shopping list builder (leftovers need no new ingredients).
- **CSRF via origin checking, not tokens**: middleware rejecting mutating requests
  whose `Origin`/`Sec-Fetch-Site` is cross-site. Fits the no-JS-framework HTMX stack;
  no template churn.

## Dependency graph

```
T6 vendor assets (base.templ, embed) ──> fixes first-click dead zone (bug 5) by itself
T1 search fix ─ independent
T2 copy/plural fixes ─ independent
T3..T5, T13..T19 ─ independent S-sized UI/handler fixes
007 migration (T8) ──> T9 modal servings ──> T10 shopping scaling
                └────> T11 leftover badge
```

---

## Phase 1 — Correctness quick wins

### Task 1: Make the ingredient/tag search chips actually filter (report #1)
**Description:** Replace the hidden-button `setAttribute('hx-get')+click()` hack in
`list.templ`'s chip builder with `htmx.ajax()`. Remove the dead button.
**Acceptance criteria:**
- [ ] Adding chip `chicken` narrows the grid to meals whose FTS doc matches
- [ ] `chicken AND zzzblorp` → "No meals found"; OR toggle broadens again
- [ ] URL updates (`?q=…`) and is restorable on reload (chips re-hydrate)
**Verification:** drive in browser; `GET /meals?q=…` visible in network log.
**Dependencies:** none. **Files:** `internal/templates/meals/list.templ`. **Scope:** S

### Task 2: Copy and pluralization fixes (report #3)
**Description:** Fix the import banner ("9 ingredient s , 9 step s" — templ whitespace
around the bare `s` branches in `scraped_fields.templ`), card "1 ingredients"
(`meal_grid.templ`), and investigate the "(about1.5kg)" missing space (likely scraper
text-join in `internal/scraper/schema_org.go`).
**Acceptance criteria:**
- [ ] Banner reads "9 ingredients, 9 steps imported" / "1 ingredient"
- [ ] Card reads "1 ingredient"
- [ ] Scraped ingredient text keeps a space before parenthesised amounts
**Verification:** unit test for a `plural(n, word)` helper; re-import lasagne URL.
**Dependencies:** none. **Files:** `scraped_fields.templ`, `meal_grid.templ`,
`internal/scraper/schema_org.go` (+test). **Scope:** S

### Task 3: Delete button no longer masquerades as disabled (report bug 4)
**Description:** In `detail.templ`, restyle Delete as an enabled destructive action
(red text/border at rest, not `text-ui-muted`).
**Acceptance criteria:**
- [ ] Delete is visually distinct from genuinely disabled controls
**Verification:** visual check on meal detail.
**Dependencies:** none. **Files:** `internal/templates/meals/detail.templ`. **Scope:** XS

### Task 4: Styled 404 / error pages (report bug 6)
**Description:** Add a `NotFound` templ page in the app layout with a "Back to meals"
link; wire it into chi's `r.NotFound(...)` and into `respondError`'s non-HTMX fallback
for 404s.
**Acceptance criteria:**
- [ ] `/meals/<bad-id>` and `/nonsense` render the app shell with nav
- [ ] HTMX partial requests still get the inline error banner
**Verification:** curl + browser both paths.
**Dependencies:** none. **Files:** new `templates/layout/error.templ`,
`internal/handlers/errors.go`, `cmd/server/main.go`. **Scope:** S

### Task 5: Unambiguous filter-chip selected state (report minor)
**Description:** In `list.templ`, when a non-default chip in a row is active, the
default chip ("All"/"Meals"/"Any") must render unselected; increase contrast of the
selected style on non-default chips.
**Acceptance criteria:**
- [ ] Exactly one chip per row reads as selected at any time
**Verification:** click through Type/Show/Stars rows.
**Dependencies:** none. **Files:** `list.templ`. **Scope:** XS

### Checkpoint A
- [ ] `make generate && go test ./...` clean; drive search, import banner, 404 in browser.

---

## Phase 2 — Self-hosted assets (infrastructure, do early: highest regression risk)

### Task 6: Vendor Tailwind + htmx, embed static assets (report #12, fixes bug 5)
**Description:** Add `make css` using the standalone Tailwind CLI with a
`tailwind.config.js` whose `content` globs cover `**/*.templ` and inline `<script>`
JS (safelist classes built dynamically in the chip builder). Commit `static/app.css`
and `static/htmx.min.js`; serve via `//go:embed`; replace both CDN `<script>` tags in
`base.templ` (both layout variants). Load htmx in `<head>` (defer) so `hx-*` elements
are live before first interaction.
**Acceptance criteria:**
- [ ] No requests leave localhost when browsing (network log clean of CDN hosts)
- [ ] All pages visually identical to before (spot-check meals/detail/plan/shopping)
- [ ] First click after a cold load works (htmx present at DOMContentLoaded)
**Verification:** browser network log; visual diff of the four pages; offline check.
**Dependencies:** none. **Files:** `base.templ`, `cmd/server/main.go`, `Makefile`,
`tailwind.config.js`, `static/*`, Dockerfile. **Scope:** L (bounded: no logic changes)

### Checkpoint B
- [ ] Full visual pass on all pages, light+dark, before proceeding.

---

## Phase 3 — Servings through the pipeline (vertical slice, report #7)

### Task 7: Migration 007 — plan-entry servings + leftover flag
**Description:** `007_plan_servings_leftover.sql`: add `servings INTEGER` (NULL =
meal default) and `is_leftover INTEGER NOT NULL DEFAULT 0` to `meal_plan_entries`;
extend `models.MealPlanEntry` + `PlanStore` read/write paths.
**Acceptance criteria:**
- [ ] Existing DBs migrate cleanly; store round-trips both fields
**Verification:** `go test ./internal/store/...` with new cases.
**Dependencies:** none. **Files:** migration, `models/plan.go`, `store/plan_store.go`
(+test). **Scope:** S

### Task 8: Servings picker in Add-to-plan and Assign modals
**Description:** Add a +/− servings stepper to both modals, defaulting to the meal's
base servings; when opened from a scaled detail page, prefill the scaled value.
Leftover extra-day entries are created with `is_leftover=1`.
**Acceptance criteria:**
- [ ] Planned entry stores chosen servings; calendar card shows "×6" style hint when
      it differs from the meal default
- [ ] Leftover days created via "extra days" get the flag
**Verification:** plan a meal at 6 servings, inspect card + DB.
**Dependencies:** T7. **Files:** `add_to_plan_modal.templ`, `assign_modal.templ`,
`handlers/plan.go`, `day_cell.templ`. **Scope:** M

### Task 9: Shopping list respects entry servings and skips leftovers
**Description:** In `buildShoppingGroups`, scale each meal's ingredients with
`internal/scaling` from meal base servings to entry servings (when set), and exclude
`is_leftover` entries. Group header shows "(6 servings)" when scaled.
**Acceptance criteria:**
- [ ] Carbonara planned at 5 servings lists 400g spaghetti, not 320g
- [ ] Leftover day contributes no duplicate ingredient group
**Verification:** unit test on `buildShoppingGroups`; browser check.
**Dependencies:** T7, T8. **Files:** `handlers/shopping.go` (+test),
`plan/shopping.templ`. **Scope:** M

### Task 10: Leftover badge on the calendar (report #11 part)
**Description:** In `day_cell.templ`, render an "↩ leftovers" badge instead of the
cook-time line for `is_leftover` entries.
**Acceptance criteria:**
- [ ] Leftover card visually distinct; no misleading "2h 10m"
**Verification:** browser check.
**Dependencies:** T7, T8. **Files:** `day_cell.templ`. **Scope:** XS

### Checkpoint C
- [ ] End-to-end: scale → plan (leftover +1) → shopping list quantities correct.

---

## Phase 4 — Shopping list usability (report #8)

### Task 11: Ad-hoc shopping items
**Description:** New table `shopping_extras` (week, text, checked) via migration 008;
an "Add item" input on the shopping page; check/uncheck/delete parity with meal
ingredients.
**Acceptance criteria:**
- [ ] Items persist per-week across reloads; deletable; included in "Clear all checks"
**Verification:** store test + browser.
**Dependencies:** none (parallel to Phase 3). **Files:** migration,
`store/plan_store.go` or new store, `handlers/shopping.go`, `plan/shopping.templ`.
**Scope:** M

### Task 12: Checked-progress counter (report #8 part)
**Description:** Header shows "n of 21 checked" and live-updates via the existing
HTMX check-toggle response (OOB swap).
**Acceptance criteria:**
- [ ] Counter correct on load and after each toggle
**Verification:** browser.
**Dependencies:** none. **Files:** `plan/shopping.templ`, `handlers/shopping.go`. **Scope:** S

*(Deferred, needs design input: cross-meal ingredient aggregation with quantity math —
see Open Questions.)*

---

## Phase 5 — Remaining gaps and polish

### Task 13: CSRF origin-check middleware (report #13)
**Description:** Middleware rejecting POST/PUT/DELETE when `Sec-Fetch-Site` is
`cross-site` or `Origin` host mismatches (allowing empty headers for same-origin
non-browser clients is acceptable for this threat model; webhook route exempt).
**Acceptance criteria:**
- [ ] Cross-origin fetch DELETE rejected (403); app flows all still work
**Verification:** middleware unit test; full browser regression of create/plan/check.
**Dependencies:** none. **Files:** new `internal/auth/csrf.go` (+test),
`cmd/server/main.go`. **Scope:** S

### Task 14: Expose Baking & Drinks in the list filter (report #10)
**Description:** Render Show chips from `models.Categories` instead of the hardcoded
pair.
**Acceptance criteria:**
- [ ] A meal saved as Baking is reachable via its chip
**Verification:** create Baking meal (then delete), filter.
**Dependencies:** none. **Files:** `list.templ`. **Scope:** XS

### Task 15: Name search affordance (report #9)
**Description:** FTS already indexes names — chips work for names today. Update the
placeholder/help copy to "Search name, ingredient or tag…" and confirm `meals_fts`
covers the name column (add if not).
**Acceptance criteria:**
- [ ] Chip "carbonara" finds Ultimate Spaghetti Carbonara
**Verification:** browser (depends on T1 landing first).
**Dependencies:** T1. **Files:** `list.templ`, possibly FTS migration. **Scope:** XS–S

### Task 16: Import maps meal type/cuisine + unit-drop investigation (report bug 2, minor)
**Description:** Map schema.org `recipeCategory`→meal type chips and
`recipeCuisine`→cuisine in the scrape autofill. Reproduce the lasagne "2 olive oil"
unit drop in a scraper unit test (fixture from BBC page) and fix the extraction.
**Acceptance criteria:**
- [ ] Lasagne import yields "2 tbsp olive oil…", Dinner pre-selected, cuisine filled
      (when source provides them)
**Verification:** scraper unit tests with fixture; live re-import.
**Dependencies:** none. **Files:** `internal/scraper/schema_org.go` (+test),
`handlers/scraper.go`, `scraped_fields.templ`. **Scope:** M

### Task 17: Plan-entry removal is touch-friendly with undo (report #11 part)
**Description:** Make the × always visible on coarse pointers (`@media (hover:none)`)
and add a 5s "Removed — Undo" toast (htmx re-POST of the captured entry) instead of a
confirm.
**Acceptance criteria:**
- [ ] × visible without hover on touch; Undo restores the entry with same fields
**Verification:** browser with device emulation.
**Dependencies:** T7 (restores servings/leftover fields). **Files:** `day_cell.templ`,
`handlers/plan.go`. **Scope:** M

### Task 18: Card image placeholder + missing-ingredients nudge (report #14, minor)
**Description:** Meals without `ImageURL` get a placeholder block (plate glyph) so the
grid stays aligned; cards/detail for meals with zero ingredients show a subtle
"No ingredients yet — add some" hint linking to edit.
**Acceptance criteria:**
- [ ] Carbonara card aligns with image cards; salmon meal shows the nudge
**Verification:** browser.
**Dependencies:** none. **Files:** `meal_grid.templ`, `detail.templ`. **Scope:** S

### Task 19: Scaler layout shift + dirty-form guard (report minors)
**Description:** Reserve fixed width for the servings label (so +/− don't move when
"(from 4)" appears). Add a `beforeunload`/cancel-click guard on the meal form when
fields differ from initial values.
**Acceptance criteria:**
- [ ] Clicking + twice quickly hits the button both times
- [ ] Cancel on a dirty form asks before discarding; pristine form doesn't
**Verification:** browser.
**Dependencies:** none. **Files:** `detail.templ`, `meals/form.templ`. **Scope:** S

### Checkpoint D (final)
- [ ] `make generate && go test ./...` green
- [ ] Full browser regression: meals list (search/filters), import, detail (scale,
      rate, cook, duplicate, delete), plan (assign, leftovers, remove+undo, week nav),
      shopping (scaled quantities, extras, progress), 404, offline load
- [ ] Bump `addon/config.yaml` version + matching git tag before push (release rule)

---

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Tailwind static build misses dynamically-built class names (chip builder JS) | High (broken styling) | `content` globs include templ+JS; safelist audit; Checkpoint B visual pass |
| Scaling shopping quantities mis-parses free-text ingredients | Med | reuse tested `internal/scaling`; unit tests on real report examples; fall back to unscaled text on parse failure |
| Origin-check middleware breaks HA ingress (proxied Origin headers) | Med | exempt when `SUPERVISOR_TOKEN` mode + ingress path header present; test both modes |
| 007 migration on live tunnel DB | Low | additive columns only; never edit old migrations |

## Open questions (need your input before the affected tasks)

1. **Aggregation (deferred task):** combined view merging "2 tbsp + 5 tbsp olive oil"
   across meals — worth the parsing complexity, or is per-meal + extras enough given
   the OurGroceries mirror is the real shared list?
2. **Ad-hoc items (T11):** should extras also push to OurGroceries when configured?
3. **Undo vs confirm (T17):** happy with undo-toast (no confirm), or prefer a confirm?
