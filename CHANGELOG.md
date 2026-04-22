# Béilí Changelog

## v1.3.x (current)

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
