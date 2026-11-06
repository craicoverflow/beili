# Todo — usability report fixes

See `tasks/plan.md` for full task specs, acceptance criteria, and checkpoints.

## Phase 1 — correctness quick wins
- [x] T1 (S): Fix search chip filtering — replace stale `hx-get` hack with `htmx.ajax()` in `list.templ`
- [x] T2 (S): Pluralization/copy fixes — banner + card done via models.Pluralize (tested). "(about1.5kg)" spacing NOT reproducible: scaler/parser keep the space; needs the source page's JSON-LD
- [x] T3 (XS): Restyle Delete button as enabled destructive action
- [x] T4 (S): Styled 404/error page wired into chi NotFound + respondError
- [x] T5 (XS): Fix ambiguous filter-chip selected state
- [ ] **Checkpoint A**: generate + tests + browser pass

## Phase 2 — self-hosted assets
- [x] T6 (L): Vendor Tailwind (standalone CLI → embedded `static/app.css`) + htmx; drop CDN tags; htmx in head
- [ ] **Checkpoint B**: visual pass all pages, light + dark, offline

## Phase 3 — servings vertical slice
- [x] T7 (S): Migration 007 — `servings` + `is_leftover` on plan entries; model + store
- [x] T8 (M): Servings stepper in Add-to-plan / Assign modals; leftover entries flagged
- [x] T9 (M): Shopping list scales by entry servings; skips leftover entries
- [x] T10 (XS): Leftover badge on calendar cards
- [ ] **Checkpoint C**: scale → plan → shopping end-to-end

## Phase 4 — shopping list usability
- [x] T11 (M): Ad-hoc shopping items (migration 008 + UI)
- [x] T12 (S): "n of N checked" progress counter with OOB update

## Phase 5 — remaining gaps and polish
- [x] T13 (S): CSRF origin-check middleware (webhook + ingress exemptions)
- [x] T14 (XS): Baking & Drinks filter chips from `models.Categories`
- [x] T15 (XS–S): Name-search copy + FTS name coverage check (after T1)
- [x] T16 (M): Import maps meal type/cuisine; fix unit-drop with scraper fixture test
- [x] T17 (M): Touch-visible plan remove + undo toast (after T7)
- [x] T18 (S): Card image placeholder + missing-ingredients nudge
- [x] T19 (S): Scaler layout-shift fix + dirty-form cancel guard
- [ ] **Checkpoint D**: full regression + version bump in `addon/config.yaml` + git tag

## Open questions (blocking noted tasks only)
- [ ] Q1: cross-meal ingredient aggregation — build or skip? (deferred task)
- [ ] Q2: should ad-hoc items push to OurGroceries? (T11)
- [ ] Q3: undo toast vs confirm on plan removal? (T17)
