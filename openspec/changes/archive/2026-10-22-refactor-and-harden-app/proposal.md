# Proposal

## Why

Tengrinews has two open Dependabot alerts: #5 (GHSA-cp6g-7hqx-qxhp, mongo-driver < 1.17.7, GSSAPI heap out-of-bounds read) and #19 (GHSA-259r-337f-4rfw, klauspost/compress <= 1.18.6, s2 out-of-bounds write, high). The code itself is also unsafe and hard to change:
- Unescaped `text/template` output (stored and reflected XSS).
- A global category-cache map with a data race that can crash the process.
- A goroutine leak on post lookup, and panics inside goroutines.
- No timeouts or graceful shutdown.
- The API key is logged.
- Raw errors are sent to clients.
- About a third of the code is dead, and there are no tests.

This change rebuilds the code to production quality, closes both alerts and keeps the site's pages working as they do today.

## What Changes

- **Dependencies:** migrate to `go.mongodb.org/mongo-driver/v2` v2.9.2. Bring `github.com/klauspost/compress` to >= 1.18.7. Driver v2.9.2 already selects v1.19.2, so no explicit pin is needed. Remove `github.com/gorilla/mux` and use the standard `net/http` ServeMux with method patterns.
- **Structure:** reorganize packages by responsibility, with interfaces defined where they are consumed. Remove the `helpers` grab-bag and the misnamed `MockArticleRepository`. Mongo queries move out of handlers.
- **Removals:**
  - The `sample.json` readers.
  - `FetchDataByID`.
  - The whole unrouted search path (newsdata call, repo and usecase methods).
  - Unused models and fields, including the never-rendered `Link`/`source_url` field and its `$articles.link` projection.
  - The unused `safeHTML` func.
  - `CutImageData`, which does nothing because its result is discarded. Content cleaning stays out of scope.
- **Rendering:**
  - `html/template` replaces `text/template`.
  - Templates are parsed once at startup.
  - Each page renders to a buffer before writing, so errors give a clean 500.
  - The three pages share one base layout with one header and one footer partial. Today's three drifted copies are unified on the post page's version:
    - The tab title is `Tengrinews` on every page.
    - The copyright line has its typos fixed.
    - Every asset URL is absolute (`/assets/...`).

    Apart from these listed intended differences, rendered markup for benign data stays the same as today's.
  - Templates and `ui/assets` are embedded with `go:embed`, so the binary runs from any directory.
- **Behavior fixes (user-visible):**
  - An unknown post id returns 404 instead of 500.
  - The category path value is lowercased and then checked against the known category list, so `/category/Business` serves `business`. The lowercased slug is used for the cache key, the newsdata.io call and the page. Anything else returns 404 and is never sent to newsdata.io or reflected into the page.
  - Home-page `totalPages` comes from the real article count, not the hardcoded 5.
  - A missing, invalid, non-positive or out-of-range `?page=` falls back to page 1.
  - Routes allow GET and HEAD only. Other methods return 405.
  - `/assets/` no longer lists directories.
- **Category cache:** a concurrency-safe cache keyed only by known categories, with a 1-hour TTL. If a refresh fails, the last good copy is still served.
- **Post lookup:** Mongo queries by `articles.article_id` instead of loading the whole collection into Go. The app stays read-only; the README documents the index to create by hand.
- **Runtime:**
  - The newsdata.io key is sent in the `X-ACCESS-KEY` header and never logged.
  - Logging uses `log/slog`.
  - Clients never see internal error text.
  - The HTTP server, the outbound newsdata client and Mongo calls all have timeouts and use request contexts.
  - SIGINT/SIGTERM shut the server down gracefully and disconnect Mongo.
  - The port stays 8080.
- **Tests:**
  - Unit tests for the newsdata client (`httptest` server), handlers (fake service), the cache (fake clock), and templates. The template test compares against today's recorded markup and allows only the listed intended differences.
  - A Mongo integration test that runs only when `TEST_MONGODB_URI` is set. It skips locally when the variable is unset and always runs in CI.
- **CI:**
  - A GitHub Actions workflow runs `gofmt` (check), `go vet`, `go test -race ./...` and `govulncheck ./...` on push and PR.
  - The workflow starts a MongoDB service container and sets `TEST_MONGODB_URI`, so the integration test runs there.
  - The race detector runs in CI only. The local gate is `go test ./...` without `-race`.

## Capabilities

### New Capabilities
- `news-browsing`: the home, category and post pages and static assets.
  - Routes and allowed methods.
  - Status codes (404/405/500).
  - Pagination rules.
  - Output escaping.
  - Error pages that never leak internals.
- `category-feed`: how category news is obtained from newsdata.io.
  - Allowed categories.
  - API-key handling.
  - 1-hour caching, and serving the last good copy when a refresh fails.
- `service-runtime`: how the process runs.
  - Required config (`NEWSDATA_API_KEY`, `MONGODB_URI`) and fail-fast startup.
  - Timeouts.
  - Structured logging without secrets.
  - Read-only Mongo access.
  - Graceful shutdown.

### Modified Capabilities
None. No specs exist yet.

## Non-goals

- Search (stays future work).
- Docker.
- Own news parser or scheduler.
- UI redesign.
- Any write to Mongo, including index creation.
- A configurable port.
- testcontainers.

## Decisions (2026-10-22, with the user)

| Decision | Why |
|---|---|
| mongo-driver v2.9.2, not v1.17.x | Every Mongo call is rewritten anyway. v1 gets patches only. |
| Delete all dead code including search | It has no route or form. Git history keeps it. |
| Fix the six user-visible bugs | Each is a correctness or security defect on paths the refactor touches. |
| stdlib ServeMux | Covers everything mux did here, gives a 405 for free, and drops a dependency. |
| 1h TTL, serve stale copy on refresh error | Keeps news fresh within the newsdata free-tier quota, and an outage doesn't break pages. |
| Shared layout; unify the drifted header/footer on the post page's version (title `Tengrinews`, fixed copyright, absolute `/assets/` URLs) | Removes the duplication and the typos. The golden test allows only the listed intended differences, so any other visual change fails it. |
| Drop the `Link`/`source_url` field and its projection | No page renders it; it is dead code. This replaces the earlier plan to fix its mapping. |
| Match categories case-insensitively (lowercase, then check the allowlist) | `/category/Business` works. One canonical slug is used for the cache, newsdata.io and the page. |
| Remove the no-op `CutImageData`; no content cleaning | Its result was discarded, so removing it changes nothing visible. |
| Env-gated Mongo integration test | Tests the real queries with no new dependency. |
| MongoDB service container in CI | The integration test runs on every push and PR instead of only by hand. |
| Race detector in CI only | This Windows machine has no cgo/C compiler. The local gate is `go test ./...`. |
| App stays read-only, index documented | The app's DB user doesn't need write access. |
| Add CI with govulncheck | Catches the next vulnerable dependency on every PR. |
| Info log per upstream category refresh | Lets the owner track calls against the newsdata.io free-tier quota. |
| Bump x/text to v0.41.0 and x/crypto to v0.56.0, raising `go` to 1.26.0 | Clears module-level advisories GO-2026-6629 and GO-2026-6355/6354/6303 before Dependabot raises them. They were not reachable here. The openpgp advisory GO-2026-5932 has no fix and stays, not reachable. |

## Impact

- **Code:** every Go file under `cmd/` and `internal/` is rewritten or deleted; `ui/*.html` is restructured into a layout plus partials; a new `.github/workflows/` file is added.
- **Dependencies:**
  - `gorilla/mux` is removed.
  - `mongo-driver` v1 is replaced with v2.
  - `klauspost/compress` is bumped.
  - `go.sum` is regenerated.
- **Behavior:**
  - The status codes listed under behavior fixes change.
  - `/category/Business` (any letter case) now serves the category instead of passing the raw value upstream.
  - Small intended visual changes:
    - The post page's tab title.
    - The copyright line on all three pages.
    - The home footer now matches the other two pages.
  - Post lookups need a hand-made index on `articles.article_id` for speed.
- **Ops:** the same two env vars are required. The new optional `TEST_MONGODB_URI` gives the Mongo URI for integration tests, and CI sets it. The binary no longer depends on the working directory.
- **Security:** closes Dependabot #5 and #19. The newsdata key and Mongo URI that sat in git history before commit `a34419e` must be rotated by the owner; no code change can fix that.
