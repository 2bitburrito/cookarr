# Cookarr

Self-hosted recipe reader. A browser extension captures a recipe page. The server stores a clean copy: story, ingredients, steps, photos and author links. The reader is a **tiling workspace** where those parts are panels you arrange yourself: tab them, split them, float them over the top, or pop them into another window.

## Why

Mealie, Tandoor, Paprika, Mela and Pestle all use a fixed layout. The best they offer is a separate cook mode. None lets you place panels. User-arranged tiling is the differentiator, and everything else supports it.

## Decisions

| Area | Choice |
|------|--------|
| Server | Go. JSON API, with the built frontend embedded via `go:embed` |
| Frontend | Plain Svelte + Vite + a client router (SPA, no SvelteKit, no Go templates) |
| Workspace | Dockview (vanilla TS core, wrapped in Svelte) |
| DB | SQLite by default. Optional Postgres via user-supplied connection details. goose migrations + sqlc |
| Users | Single owner, self-hosted |
| Packaging | Docker. The Node build happens in the image build |

## Auth

- Username and password for the owner, set on first run. Session cookie for the web UI.
- Password reset is a CLI command that only runs inside the container (`docker exec`).
- The extension logs in with the same credentials, exchanges them once for a revocable token, and stores only the token.
- `AUTH_DISABLED=true` skips login for people behind Tailscale or an auth proxy.

## Capture pipeline

This is the most important part of the server. Parsing lives on the server, not in the extension, so fixes ship without a browser-store review.

1. The extension strips `<script>` and `<style>` from the tab's rendered DOM and POSTs `{url, html}`. Stripping keeps session tokens and tracking IDs out of storage. Scripts of type `application/ld+json` are kept, since that's where the recipe data lives. It works on paywalled and logged-in pages.
2. If the server returns 413, the extension sends just the URL and the server fetches the page itself. The web UI's "paste a URL" uses the same path.
3. The server extracts:
   - **Structured recipe data:** JSON-LD, microdata, OpenGraph. Keep `borschtapp/krip` as a benchmark baseline; build and test Cookarr's own extractor against the fixture corpus.
   - **Story text:** a readability port. The maintained one appears to be `codeberg.org/readeck/go-readability/v2` (go-shiori's is reportedly archived). Verify before adopting.
   - **Sanitising:** `bluemonday` on all stored HTML.
4. The server downloads the photos, with a guard against fetches to private and loopback addresses that re-checks redirects. Each image is re-encoded and resized (which strips EXIF), then stored on disk by content hash. The `image` row keeps its source URL.
5. The server stores the stripped HTML so a recipe can be re-parsed when the extractor improves.

Limits: set our own body cap of about 10–20MB. nginx defaults to 1MB, so the README needs to document the proxy setting.

## Workspace

- **Panels:** `ingredients`, `steps`, `story`, `photos`, `notes`, `author`. Timers come later.
- **Layouts:** Dockview's serialised JSON, stored in the DB. Resolution order is per-recipe override, then the user's chosen preset, then the built-in default.
- **Built-in presets:** *Classic* (ingredients and steps side by side, story in a tab), *Cook* (narrow pinned ingredients, one large steps panel), *Read* (story first, ingredients floating).
- **Device class:** layouts are stored per class (desktop, tablet, phone).
- **Floating and popout:** Dockview floating groups cover "hover over the steps". Popout windows cover a second screen. Quick transient popovers, like a timer or an ingredient lookup, are Svelte components outside Dockview.
- **Keyboard:** tmux/i3-style focus, split, move and close. Dockview doesn't provide this, but its API is enough to build it.

Dockview risks to test in the spike. I've only read the docs and haven't run it:
- Gaps between groups use `--dv-group-gap-size`. Issue #613 reports panel overflow with gaps and nested splits.
- Group layout is absolutely positioned, so soft or squishy effects go on panel contents and frames, not on layout behaviour.
- Floating behaviour on touch devices.

## Look and feel

Warm dark and soft, not black and purple, and not ASCII-terminal. Think a Hyprland rice made friendly for a kitchen.

- **References:** evervault (dark, pill corners, without the purple) and Linear (restraint), kept warm.
- **Shape:** pill tabs, rounded tiles with 1px borders, gaps between tiles, a coloured active-tile border, a status bar. Dough and spill blob shapes as decoration, and small SVG cooking illustrations (pots, pans, utensils) for empty states, panel icons and loading.
- **Themes:** CSS variable sets, swappable at runtime. Each theme can give the active-tile border a different spice colour.

Starting palette (placeholders to react to, check contrast before locking in):

| Role | Colour | Hex |
|------|--------|-----|
| Base | espresso | `#1c1714` |
| Tile surface | warm brown | `#2a221d` |
| Idle border | toasted taupe | `#3d322a` |
| Text | cream | `#f3e9d8` |
| Primary / active border | butter | `#f5c84c` |
| Secondary | apricot | `#e8873a` |
| Danger / timers | paprika | `#d4553a` |
| Success | sage | `#9bb07a` |

## Authors

- Extract schema.org `author`, `publisher` and `sameAs`, plus `rel="me"` and footer or about links.
- Classify links by domain into social and support (Ko-fi, Patreon, Buy Me a Coffee, GitHub Sponsors and similar). No standard exists for "support the author", so the domain list is ours to maintain. The user can edit links.
- The source URL and site name are always visible. The app only links out and never handles payments.

## Data model (first cut)

`owner`, `api_token`, `session` · `recipe` (title, description, yield, times, source_url, story_html, stripped_html, captured_at) · `ingredient` (group, raw text) · `step` (ordinal, text) · `image` (hash, path, source_url, role) · `author`, `author_link` (kind: social or support) · `tag`, `recipe_tag` · `pref` (key/value JSON) · `layout_preset` (name, dockview JSON, device class).

## Database notes

- Write queries once with `sqlc.arg()` and generate for both engines, behind a store interface. To confirm in Phase 1.
- Keep the core to portable SQL. Search sits behind an interface: FTS5 on SQLite, `tsvector` on Postgres.
- CI runs migrations and tests against both.

## Repo layout

One Go module at the root. The web app and extension are separate JS projects.

```
cookarr/
├── cmd/cookarr/          server entrypoint (password reset lives in scripts/reset-password.go)
├── internal/
│   ├── api/              HTTP handlers, middleware, routing
│   ├── auth/             login, sessions, extension tokens
│   ├── capture/          JSON-LD/microdata parsing, readability, sanitising
│   │   └── testdata/     saved recipe pages and source list for parser tests
│   ├── images/           download, SSRF guard, resize, hash storage
│   ├── authors/          link extraction and social/support classification
│   └── store/            store interface plus sqlc-generated code per engine
├── db/
│   ├── migrations/       goose, one folder per engine (sqlite/, postgres/)
│   ├── queries/          sqlc queries
│   └── sqlc.yaml
├── web/                  Svelte + Vite app (src/, dist/), own package.json
│   └── embed.go          //go:embed all:dist
├── extension/            MV3 extension, own package.json
├── docs/
├── magefile.go           build, test, generate, dev tasks
├── Dockerfile            Node build stage, Go build stage, final image
└── compose.yml
```

- `web/dist` is gitignored and built in the Docker Node stage. Keep a placeholder file in it so plain `go build` works.
- Dev loop: `vite dev` proxying to the Go server, with no embedding.
- No shared JS package to start. If the API grows, generate TS types from an OpenAPI spec.
- One migrations folder per engine, to confirm in Phase 1.
- Tasks run through [Mage](https://magefile.org) (`magefile.go`), not Make.

## Open questions

1. **SQLite to Postgres:** does "plug in your Postgres" mean a fresh schema, or also moving existing data from SQLite?
2. **Browsers:** Chrome, Firefox, or both? (Manifest V3 makes both cheap, but it doubles testing.)
3. **Mobile:** is phone reading a first-class target? A phone needs a one-panel-at-a-time but **needs** the ability to have the ingredients always viewable. 
4. **Ingredient parsing:** scaling and unit parsing in v1, or raw text first? (Plan assumes raw first.)
5. **Licence:** AGPL or MIT?
