# Béilí — Usability & Feature Test Report

**Date:** 2026-07-02 · **Tested against:** `http://localhost:8080` (Chrome, desktop viewport)
**Areas covered:** Meals list + filters, meal detail + serving scaler, add/edit/duplicate/delete, URL import, Surprise me, Meal Plan (assign, leftovers, remove, week nav), Shopping List (check-off, persistence), 404 handling.

All test data created during the session was cleaned up (test meal deleted, plan entries removed).

---

## Bugs

### 1. Ingredient/tag search filter does nothing (high)
Adding chips (e.g. `chicken AND zzzblorp`) still shows all meals. The request fired is `GET /meals` with **no query parameters** — the chips are never serialized into the HTMX request. The AND/OR toggle is consequently also dead. (Type and Stars filters do work — e.g. `?meal_type=breakfast`.)

### 2. URL import drops ingredient units
Importing `bbcgoodfood.com/recipes/classic-lasagne-0` produced first ingredient **"2 olive oil plus extra for the dish"** — the `tbsp` unit was lost. Ingredients also weren't normalised to metric/BASE_SERVINGS form (contradicts the intended Gemini normalisation-at-import behaviour).

### 3. Pluralization/spacing copy bugs
- Import success banner: *"9 ingredient s , 9 step s imported"* (broken pluralization + stray spaces).
- Meal card: *"1 ingredients"*.
- Ingredient text: *"1 free-range chicken (about1.5kg)"* — missing space.

### 4. Delete button looks permanently disabled
On the meal detail page, Delete is styled `text-ui-muted` at rest so it reads as disabled, but it's fully active (`disabled: false`, `hx-confirm` set). Misleading affordance — users may believe deleting is impossible.

### 5. First click after page load frequently does nothing
Reproduced multiple times (meal card click, next-week arrow): the first click after a full page load has no effect; the second works. Almost certainly because htmx is loaded from `unpkg.com` at page bottom — until it arrives, `hx-get` elements are inert. See finding 12.

### 6. Unstyled 404
`/meals/<bad-id>` returns bare plain text `404 page not found` — no layout, no way back.

---

## Feature gaps

### 7. Serving scaling doesn't flow into the plan or shopping list
The detail-page scaler works correctly (320g→400g, 3→4 eggs, 2→2½ cloves at 4→5 servings), but:
- The "Add to plan" modal has **no servings field**; the scaled state is not captured.
- The shopping list shows base quantities (320g spaghetti) regardless of scaling.
For a family app, "we're 6 this Sunday" is a core scenario the pipeline can't express.

### 8. Shopping list is per-meal only, with no editing
- Duplicate ingredients aren't aggregated across meals (garlic + olive oil appear in both columns).
- No way to add ad-hoc items (bin bags, milk).
- No checked/total progress (header says "21 ingredients across 2 meals" even as items are checked).
- No export/share (relevant to the planned OurGroceries → Tesco pipeline).
- Junk entries flow straight through from recipe text: *"roasted or mashed potatoes"* (a serving suggestion) and *"white sauce (find a recipe in the method…)"* appear as buyable items.

### 9. No name search on the Meals page
The only search input is "Add ingredient or tag…". You cannot search meals by name (the plan's Assign modal *does* have name search — inconsistent). FTS5 is already in the schema; the main library page doesn't expose it.

### 10. Baking & Drinks categories are unreachable from the list
The Add Meal form offers categories Meals / Toddler / Baking / Drinks, but the Meals page "Show" filter only has Meals and Toddler chips. Anything saved as Baking or Drinks appears to have no browsing entry point.

### 11. Meal plan interaction gaps
- Removing a planned meal is a small hover-only "×" — undiscoverable on touch devices, and it deletes instantly with **no confirm and no undo** (while meal delete does confirm — inconsistent).
- No way to move/swap a planned meal to another day/slot (no drag & drop; must remove + re-add).
- Leftover days are visually identical to cook days — the Saturday leftover card still shows "2h 10m" cook time and no "leftover" badge.
- Clicking a planned entry jumps to the recipe; there's no lightweight menu (view / move / remove / scale).

### 12. External CDN dependencies in production
Pages load `cdn.tailwindcss.com` (the Play CDN — explicitly not for production; recompiles CSS in the browser on every load) and `htmx.org@2.0.4` from unpkg. Consequences: broken styling/interactivity when offline or when the HA ingress host has no internet; the first-click dead zone (bug 5); slower loads. Vendor both assets into the binary (embed like the migrations).

### 13. No CSRF protection on mutating endpoints
`DELETE /meals/{id}` succeeded from page JS with no token/header requirement. Since Béilí is exposed via Cloudflare tunnel, a cross-site page in a family member's browser could fire state-changing requests. Worth adding same-site cookie/origin checks or a CSRF header requirement.

### 14. Meals with no ingredients are silently fine
The YouTube-imported "Baked Feta Salmon Pasta" has no ingredients, instructions, meal type, or servings — the detail page just omits those sections, it contributes nothing to the shopping list, and nothing nudges you to complete it. A "missing ingredients" hint on the card/detail would help.

---

## Minor / polish

- **Cards without images** (Carbonara) have no placeholder image — grid looks broken next to image cards.
- **Filter chip selected state is ambiguous**: "All"/"Meals" stay solid orange while another chip (Breakfast/Toddler) is selected with only a subtle border — hard to tell what's active.
- **Scaler +/− buttons shift position** when the label grows to "5 servings (from 4)", causing misclicks.
- **Empty-form submit** only focuses the Name field — no visible validation message.
- **Cancel on a filled Add Meal form** (including a completed import) discards everything with no confirmation.
- **Import doesn't map meal type/cuisine**: lasagne import filled name/description/times/ingredients/steps/source, but left Meal Type unselected.

## What works well

- URL import is fast and fills name, description, times, servings, ingredients, steps, and the source entry.
- Deterministic scaler output is correct, including vulgar fractions (2½).
- Leftovers option in both Add-to-plan and Assign flows; shopping list annotates "Dinner +1".
- Shopping list check-offs persist across reloads; "Clear all checks" available.
- Surprise me (`/meals/random`) with "Try another" is a nice touch.
- Rating (with Clear), Mark as cooked today, Copy link, Duplicate all present on detail page.
- Week navigation and Today work (once htmx has loaded); plan/shopping stay in sync.

## Not tested

- Print view (native dialog would block the automation session).
- Mobile layout (viewport meta + responsive classes exist; window resize couldn't be verified in this session) — worth a manual phone check, especially the hover-only plan "×".
- Home Assistant ingress mode (`SUPERVISOR_TOKEN`) behaviours.
