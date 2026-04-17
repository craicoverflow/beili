# Béilí

A family meal database and weekly meal planner — runs as a Home Assistant addon or standalone locally.

## Features

- Store meals with ingredients, prep/cook time, cuisine, servings, ratings, and step-by-step instructions
- Multiple source types per meal: URL, book page, YouTube, or freeform notes
- Auto-fill form fields by pasting a recipe URL (schema.org/Recipe scraping)
- Import a full meal directly from a recipe URL in one click
- Recipe images scraped from og:image / twitter:card meta tags
- Full-text search across name, description, cuisine, and ingredients
- Filter by meal type and star rating
- Weekly meal plan calendar — assign meals to breakfast/lunch/dinner/snack slots
- Weekly shopping list — aggregated ingredients for the current plan
- Cook log — mark meals as cooked and track history
- Duplicate a meal to create variations
- Export/import meals as JSON
- Random meal picker
- JSON API for Home Assistant sensor integration

## Tech stack

- **Go** single binary with **SQLite** (no CGO, cross-compilation friendly)
- **Templ** type-safe templates + **HTMX** for SPA-like UX without a JS framework
- **Tailwind CSS** for styling

---

## Local development

### Prerequisites

- Go 1.25+
- [Templ](https://templ.guide/) — `go install github.com/a-h/templ/cmd/templ@latest`
- [Air](https://github.com/air-verse/air) (optional, for live reload) — `go install github.com/air-verse/air@latest`

### Run

```bash
# Build and run
make build
./bin/server

# Or with live reload
make dev
```

Open http://localhost:8080 — redirects to `/meals`.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DATA_DIR` | `./data` | Directory for the SQLite database |
| `SUPERVISOR_TOKEN` | _(unset)_ | Set by HA; enables HA mode (port 8099, JSON logs) |
| `INGRESS_PATH` | _(unset)_ | URL prefix injected by HA ingress proxy |

### Make targets

```bash
make build              # templ generate + go build
make dev                # live reload with Air
make test               # go test ./...
make lint               # run golangci-lint
make seed               # seed the database with sample data
make build-linux-amd64  # cross-compile for HA amd64
make build-linux-arm64  # cross-compile for HA arm64
make docker-build       # build the addon Docker image locally
make clean              # remove bin/ and generated *_templ.go files
```

---

## Home Assistant installation

1. In HA, go to **Settings → Add-ons → Add-on Store → ⋮ → Repositories**.
2. Add `https://github.com/craicoverflow/beili`.
3. Install **Béilí** from the store.
4. Start the addon — the panel icon appears in the HA sidebar.

The addon uses HA ingress, so no port forwarding is needed. Data is stored in `/data/meals.db` inside the addon container (backed by the HA `data` map).

### Home Assistant JSON API

When running in HA mode, a JSON API is available for use in sensors and automations:

- `GET /api/plan/today` — today's meals (breakfast/lunch/dinner/snack)
- `GET /api/plan/week` — the current week's meal plan

---

## Project structure

```
cmd/server/          Entry point
cmd/seed/            Database seeder
internal/
  config/            HA vs local mode detection
  auth/              X-Remote-User middleware (HA ingress auth)
  db/                SQLite open + migration runner
  db/migrations/     SQL migrations (meals, sources, meal_plan, cook_log, FTS5)
  models/            Meal, Source, MealPlanEntry, CookedLog structs
  store/             MealStore, PlanStore (CRUD + FTS search)
  scraper/           schema.org/Recipe URL scraper
  handlers/          HTTP handlers (meals, plan, search, scrape, shopping, export, api)
  templates/         Templ components (layout, meals, plan, components)
addon/               HA addon metadata and Dockerfile
```
