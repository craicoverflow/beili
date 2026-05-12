# Béilí Changelog

## v1.8.x (current)

### v1.8.4
- Fixed: Instagram-imported recipes no longer fail to load due to Firefox's frame-ancestors blocking — the embedded player is replaced with a "Watch on Instagram" thumbnail card that opens the post in a new tab

### v1.8.3
- Fixed: Home Assistant addon build — pinned `templ` CLI to match `go.mod` and disabled Go VCS stamping so shallow clones build cleanly

### v1.8.2
- Added: Baking and Drinks meal categories alongside Main and Toddler

### v1.8.1
- Fixed: "Add Meal" button now works reliably in the Home Assistant mobile app and on mobile browsers — previously a tap could return 401 Unauthorized in the HA app or appear unresponsive on mobile
- Improved: AI recipe scaling now outputs metric units only (g, ml, kg, l), converts imperial measurements before scaling, and rounds to sensible whole numbers instead of fractions

### v1.8.0
- Added: Instagram post and reel imports — pasting an Instagram URL is auto-detected and the meal detail page now shows an embedded player in place of the hero image
- Added: Instagram is selectable as a source type when adding sources manually

## v1.7.x

### v1.7.0
- Changed: Meal Plan is now the landing page (was Meals)
- Added: mobile meal-plan view — rolling 4-day list anchored at today, with prev/next stepping ±4 days, Google Calendar 4-day-style. Desktop keeps the 7-day Mon–Sun grid
- Added: meal categories (Main / Toddler) — toddler recipes are kept out of the default meals list, plan calendar, and random picker unless explicitly requested via the new "Show: Toddler" filter
- Improved: serving-size scaling on the recipe page now scales numbers in instructions as well as ingredients, and is unit-aware (kg/g/mg/l/ml/tbsp/tsp/cups/oz/lb) so non-quantity numbers like step counts aren't accidentally rescaled

## v1.6.x

### v1.6.3
- Fixed: copy link button in Home Assistant now generates a URL via the HA panel path (`/hassio/ingress/beili/...`) instead of the raw ingress URL, so deep links work correctly on devices that haven't previously opened the addon through the sidebar

### v1.6.2
- Added: YouTube video title is now auto-filled when importing a YouTube URL

### v1.6.1
- Fixed: YouTube recipe cards now show the video thumbnail in the recipe list and search results

### v1.6.0
- Added: YouTube video embed on recipe detail page — recipes with a YouTube source now show an embedded player in place of the hero image
- Added: importing a YouTube URL auto-detects the video and pre-fills the source as type YouTube, prompting you to add the name and ingredients manually

## v1.5.x

### v1.5.0
- Added: copy link button on recipe detail page — copies the full recipe URL to the clipboard, useful in Home Assistant where the address bar shows the HA parent URL rather than the specific recipe
- Added: serving count is now persisted per recipe via localStorage, so your last-used serving size is remembered between visits

## v1.4.x

### v1.4.3
- Fixed: font size buttons on recipe detail now work correctly on web and mobile
- Added: leftovers support when assigning meals to the meal plan

### v1.4.2
- Fixed: Save recipe button no longer hangs indefinitely — loading state is now applied via `onsubmit` instead of `onclick`, preventing the browser from cancelling form submission
- Added a 30-second timeout on AI ingredient normalisation so a slow or unresponsive Gemini API doesn't block saves

### v1.4.1
-

## v1.3.x

### v1.3.4
- Fixed a JavaScript error that could occur when saving a recipe after Gemini normalisation

### v1.3.3
- Fixed: replaced deprecated `armv7` architecture with `armhf` for correct HA addon targeting

### v1.3.2
- Duplicate import prevention — importing a recipe from a URL you've already imported is now blocked

### v1.3.1
- Per-user recipe ratings with averaged display across all users

### v1.3.0
- Font size controls on recipe detail view (useful for cooking at a glance)
- Chip-based ingredient and tag search with AND/OR logic
- Servings scaler on recipe detail view
- Loading state on the Gemini save button

---

## v1.2.x

### v1.2.9
- Gemini model is now configurable via addon option and environment variable

### v1.2.8
- Updated default Gemini model to `gemini-2.5-flash`

### v1.2.7
- AI-powered recipe normalisation on save via Gemini — ingredients are scaled and standardised automatically

### v1.2.6
- Fixed ISO week start calculation for weeks beginning on Sunday

### v1.2.5 / v1.2.4 / v1.2.3 / v1.2.2 / v1.2.1
- Shopping list webhook to send ingredients directly to Home Assistant automations
- Webhook is configurable with just a webhook ID in the addon options
- Fixed webhook payload key collision with Jinja2's built-in `dict` method
- Fixed default webhook base URL to use `homeassistant:8123` in addon mode

### v1.2.0
- Light/dark mode following your device's colour scheme preference

---

## v1.1.x

### v1.1.3
- Fixed mobile search trigger and horizontal overflow issues

### v1.1.2 / v1.1.1 / v1.1.0
- Mobile-friendly navigation with top bar and hamburger drawer

---

## v1.0.x

Initial Home Assistant addon releases — core recipe management, URL scraping, meal plan calendar, shopping list, full-text search, and HA ingress support.
