# CLAUDE.md

App runs at http://localhost:8080 by default.

## Critical constraints

**Templates**: Always edit `.templ` sources — never hand-edit `*_templ.go`. Run `make generate` after editing templates if not using `make dev`.

**Migrations**: Add new migrations as `NNN_description.sql` in `internal/db/migrations/` — never modify existing migration files. They are embedded at compile time and run in filename order.

**Store layer**: No interface abstraction — handlers depend on store structs directly. `MealTypes` and `StringList` on `models.Meal` are JSON TEXT in SQLite with custom `driver.Valuer`/`sql.Scanner` — don't swap these for native slice types.

**No CGO**: Uses `modernc.org/sqlite` (pure Go). Do not introduce CGO dependencies.

## Non-obvious gotchas

- `SUPERVISOR_TOKEN` env var switches the app into Home Assistant mode (port 8099, JSON logging, `INGRESS_PATH` prefix). Don't remove this env check.
- HTMX drives most interactions — full-page nav uses explicit `hx-boost` is intentionally avoided on some routes (reverted due to plan calendar button breakage).
- FTS5 full-text search is baked into the SQLite schema — use it for search queries rather than `LIKE`.
