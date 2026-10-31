# Roadmap

Each phase ends with something runnable.

## Phase 0: Spikes

- [ ] **Parser corpus:** save about 30 real recipe pages as fixtures (big sites, small blogs, ad-heavy pages, a few with no JSON-LD). Run `krip` and `go-recipe`, pick one, record the failure rate.
- [ ] **Story extraction:** run readability on the same fixtures and confirm the maintained fork.
- [ ] **Dockview in Svelte:** gaps, coloured active border, pill tabs, floating groups, popout, JSON save and restore, and deeply nested splits (issue #613).
- [ ] **Capture:** minimal extension that POSTs stripped `{url, html}` to a stub, including the 413 to URL-only fallback.
- Exit: parser chosen, Dockview confirmed or a fallback decided.

## Phase 1: Server core

- [ ] Repo layout, config, structured logging
- [ ] goose + sqlc on SQLite and Postgres, with CI against both
- [ ] Owner login, sessions, extension tokens, `AUTH_DISABLED`, container-only password reset
- [ ] Capture pipeline: parse, sanitise, image fetch with SSRF guard, store, keep stripped HTML
- [ ] URL-fetch path
- [ ] Dockerfile (Node build stage) and compose example
- Exit: paste a URL and get a clean stored recipe.

## Phase 2: Extension

- [ ] MV3 extension: options page (server URL, login), save button, success and error states
- [ ] Detect a recipe on the page and badge the icon
- [ ] Browser builds per the open question
- Exit: one click on a real recipe site saves it.

## Phase 3: Tiling workspace, the main feature

- [ ] Panel types and data endpoints
- [ ] Dockview workspace: tabs, splits, float, popout
- [ ] Layout save and restore, built-in presets, preset manager, per-recipe override, per-device-class layouts
- [ ] Keyboard shortcuts, status bar
- Exit: arrange a recipe your way and it stays that way.

## Phase 4: Look, prefs and cook mode

- [ ] Warm theme system (CSS variables) with swappable themes, pill and dough shapes, cooking illustrations
- [ ] Prefs: theme, font and size, units, default preset
- [ ] Cook mode: wake lock, step focus, check-off for ingredients and steps
- [ ] Photo gallery panel with lightbox

## Phase 5: Authors

- [ ] Extract and classify author, social and support links, with editing
- [ ] Author panel, per-author page, always-visible source attribution

## Phase 6: Library

- [ ] Search behind an interface (FTS5 / `tsvector`), tags, collections, favourites
- [ ] Re-parse from stored HTML
- [ ] Export and import (schema.org JSON, DB plus images backup)

## Phase 7: Release

- [ ] Backup, restore and upgrade docs; reverse-proxy body-size note
- [ ] Rate limits, size limits, CSP
- [ ] CI/CD: test against both databases, lint, build the image, tagged releases
- [ ] Docs and a demo instance

## Later

- Video capture
- Ingredient parsing, scaling, unit conversion (if not in v1)
- Extension-side image upload for hotlink-protected hosts
- Shopping list and meal planner (Mealie and Tandoor already do these well)
- Multi-user, OIDC, shared recipes
