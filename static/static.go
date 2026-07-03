// Package static embeds the vendored Tailwind build and htmx runtime so the
// app never fetches assets from a CDN at request time — see CLAUDE.md/task
// report #12 (external CDN dependency broke the app for HA users behind an
// air-gapped or ingress-only network).
package static

import "embed"

//go:embed app.css htmx.min.js
var FS embed.FS
