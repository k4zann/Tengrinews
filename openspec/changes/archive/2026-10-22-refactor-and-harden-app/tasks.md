# Tasks

Every group ends with the build gate: `go build ./... && go vet ./... && go test ./...` green. Old and new code coexist until group 6. Design refs are `design.md` D1-D9.

The local test gate is `go test ./...` without `-race`: this Windows machine has no cgo/C compiler, so the race detector runs in CI only (user decision 2026-10-22).

User decisions applied (2026-10-22):
- one unified header and footer, with only the D7 intended differences
- `Link`/`source_url` dropped
- case-insensitive category match
- `CutImageData` removed
- MongoDB service container in CI (`mongo:8.0`, matches production 8.0)
- the stray `</a>` at the end of the copyright line is removed (confirmed)

## 1. [go-dev] Domain types and newsdata client

- [x] 1.1 Add `.gitattributes` with `*.go text eol=lf` (D9). Verify: `git check-attr eol -- internal/news/news.go` prints `eol: lf`.
- [x] 1.2 Create `internal/news` per D2: `Article` (no `Link` field), `Category{Slug, Name}`, `Categories()` (the nine in spec order, returns a copy), `ParseCategory(raw)` (`strings.ToLower`, then exact slug match; not `EqualFold`) and `ErrNotFound`. Verify `go test ./internal/news` passes and covers:
  - the order
  - `ParseCategory("business")`, `("Business")` and `("BUSINESS")` all returning slug `business`
  - `ParseCategory("politics")`, `("../x")` and `("ſports")` all returning false
- [x] 1.3 Create `internal/newsdata`: `DefaultBaseURL`, `New(baseURL, apiKey, hc)` and `Latest(ctx, category)` per D4. It encodes the query with `url.Values`, sends the `X-ACCESS-KEY` header, requires 200, reads at most 4 MiB, requires `status=="success"`, and maps `title`/`image_url`/`category`. Errors never include the key. Verify: `go build ./...`.
- [x] 1.4 Add `internal/newsdata` tests against `httptest.Server`. Verify `go test ./internal/newsdata` passes and covers:
  - GET with decoded query `category`/`language=en,ru`, the header equal to the key, and a raw URL without `apikey`
  - success mapping, including `image_url: null`
  - empty `results`
  - 401, 429 and 500
  - 200 with invalid JSON, and 200 with `{"status":"error",...}`
  - context deadline exceeded
  - no error string containing the key
- [x] 1.5 Gate. Verify: `gofmt -l internal/news internal/newsdata` prints nothing and `go build ./... && go vet ./... && go test ./...` is green.

## 2. [go-dev] Category feed cache

- [x] 2.1 Make `golang.org/x/sync` a direct requirement (`go get golang.org/x/sync@v0.21.0`, the exact version driver v2.9.2 requires, which keeps `go 1.25.0`; superseded by 6.8: `go 1.26.0`, x/sync v0.22.0). Verify: go.mod lists it without `// indirect`, and `go build ./...` is green.
- [x] 2.2 Create `internal/feed` per D5:
  - `Fetcher` interface, `TTL = time.Hour`, `New(f, log, now)`
  - `Articles(ctx, category)`: runs `news.ParseCategory` first; an unknown value gives `news.ErrNotFound` with no fetch and no entry; only the canonical slug is used as map key, single-flight key and `Latest` argument
  - the map is behind a mutex; the single-flight refresh runs with `context.WithoutCancel`; on failure it serves the stale copy and logs a `Warn`

  Verify: `go build ./...`.
- [x] 2.3 Add tests with a fake `Fetcher` and a fake clock. Verify `go test ./internal/feed` passes; CI also runs it with `-race`. They cover:
  - fresh copy at 59m with no fetch
  - refresh at 61m
  - stale copy served on error, with a Warn record captured from a buffer-backed slog handler
  - no copy plus error returns the error
  - recovery on the next request
  - `Sports` after `sports` reusing the same entry, with the fetcher only ever receiving `sports`
  - `politics`/`../x`/`ſports` giving `ErrNotFound` with zero fetches
  - 50 concurrent cold requests making exactly 1 fetch and all getting the same articles
  - the first caller's cancelled ctx not cancelling the shared fetch
- [x] 2.4 Gate. Verify: `gofmt -l internal/feed` prints nothing and `go build ./... && go vet ./... && go test ./...` is green.

## 3. [go-dev] Read-only Mongo v2 store

- [x] 3.1 Run `go get go.mongodb.org/mongo-driver/v2@v2.9.2`; v1 stays until group 6. Verify:
  - `go list -m go.mongodb.org/mongo-driver/v2` prints `v2.9.2`
  - `go list -m github.com/klauspost/compress` prints >= `v1.18.7` (expected `v1.19.2`)
  - `go build ./...` is green
- [x] 3.2 Create `internal/mongostore` with `Open(ctx, uri, database)` and `Close(ctx)` per D3: client `SetTimeout(5s)`, ping, disconnect on ping failure, a fixed `invalid MONGODB_URI` error for URI parse failures (text extended in 7.5 to `invalid MONGODB_URI or SRV lookup failed`), and a private `articleDoc` with bson tags for the D3 data-model fields (no `link`). Verify: `go build ./...`.
- [x] 3.3 Implement `CountArticles`, `ListArticles(skip, limit)` (projection without `link`) and `ArticleByID` with the pipelines and positional projection from D3; `ErrNoDocuments` becomes `news.ErrNotFound`. Verify: `go build ./...`.
- [x] 3.4 Add unit tests that need no server. Verify `go test ./internal/mongostore` passes and covers:
  - `Open` with a malformed URI containing `s3cr3t` returns an error without `s3cr3t`
  - `Open` against `mongodb://user:s3cr3t@127.0.0.1:1/?serverSelectionTimeoutMS=300` fails fast, with an error lacking `s3cr3t`
- [x] 3.5 Add `store_integration_test.go`, gated on `TEST_MONGODB_URI`. It skips when the var is unset, but fails if the var is unset while `CI` is set (D9). It seeds a throwaway database `tengrinews_it_<unixnano>` with two `{articles:[...]}` documents (7 and 5 articles, one with an empty `category`, stored `link` values present) and drops it in `t.Cleanup`. Verify that `go test ./internal/mongostore` skips locally without the var, and with the var set against a mongod passes and covers (not run locally, no mongod here; verified in CI via 7.1, which must show PASS, not SKIP):
  - count = 12
  - `List(0,10)` returns 10 in storage order
  - `List(10,10)` returns 2
  - `ArticleByID` returns all `news.Article` fields
  - a missing id gives `news.ErrNotFound`
- [x] 3.6 Add a "MongoDB" section to the README:
  - the read-only `news.articles` access
  - the `db.articles.createIndex({"articles.article_id": 1})` command (run in db `news`)
  - that the app user needs only the `read` role
  - `TEST_MONGODB_URI`, which needs write access to throwaway `tengrinews_it_*` databases and is set by CI

  Verify: the command runs as written in `mongosh`. (Not run locally, no mongod here; verified in CI via 7.1.)
- [x] 3.7 Gate. Verify: `gofmt -l internal/mongostore` prints nothing and `go build ./... && go vet ./... && go test ./...` is green.

## 4. [go-dev] Embedded templates, unified frame and html/template rendering

- [x] 4.1 Record the goldens before touching any template:
  - Add fixtures in `internal/web/fixtures_test.go`, as benign data: no `<>&'"+`, plain https image URLs. They cover a home page at page 2 of 3 (both links), a `business` category page with 2 articles, and a post with 1 creator.
  - Add `internal/web/golden_record_test.go` behind `//go:build goldenrecord`. It runs today's `ui/index.html`, `ui/category.html` and `ui/post_details.html` through `text/template` with `lower`, and writes `internal/web/testdata/golden/{home,category,post}.html`.

  Verify: `go test -tags goldenrecord -run TestRecordGolden ./internal/web` writes the three files, they contain the fixture titles, and they are committed unedited.
- [x] 4.2 Create `ui/ui.go` (`package ui`, `//go:embed templates all:assets`, `var Files embed.FS`). Create `ui/templates/layout.html`, `partials/header.html`, `partials/footer.html` and `pages/{home,category,post}.html` per D7:
  - one frame taken from `ui/post_details.html`, with the title `Tengrinews`, the copyright `This site was made by Arshat Aitkozha` without the stray `</a>`, and only absolute `/assets/` URLs, including the home image fallback
  - pages define only `"main"`
  - nav built from `.Slug`/`.Name`
  - `{{with .Category}}{{index . 0}}{{end}}` guards
  - commented-out markup that contains actions deleted

  Leave the old `ui/*.html` untouched until group 6. Verify: a `ui` package test checks that `templates/layout.html`, `assets/css/style.css` and `assets/scss/_footer.scss` exist in `Files`.
- [x] 4.3 Create `internal/web/render.go` per D7:
  - parse each page once from an `fs.FS` (layout, then partials, then page), returning an error
  - `homeView`, `categoryView` and `postView`
  - `render` executes into a buffer and writes the `Content-Type` header, the status and the body only on success

  Verify: tests confirm that parsing `ui.Files` succeeds and that parsing an `fstest.MapFS` with a broken template returns an error.
- [x] 4.4 Add a golden test per D7:
  - normalize both sides (strip `<!--…-->`, drop whitespace next to tags, collapse whitespace runs)
  - encode the nine intended-difference rows as per-page `{old, new, count}` pairs, each of which must occur exactly `count` times in the normalized recording
  - apply the pairs and require equality with the new rendering, printing the first differing offset on failure

  Verify: `go test ./internal/web -run Golden` passes. A temporary unlisted template edit (e.g. changing one class) makes it fail; that edit is reverted.
- [x] 4.5 Add frame, escaping and edge tests. Verify `go test ./internal/web` passes and covers:
  - the three pages having byte-identical `<head>`, `<header>` and `<footer>` sections, with title `Tengrinews`
  - no `href="assets/` or `src='assets/` anywhere
  - title `<script>alert(1)</script>` rendered escaped
  - content `<img src=x onerror=alert(1)>` rendered as text
  - image URL `javascript:alert(1)` leaving no `javascript:` in the output
  - an empty `Category` on home and post rendering an empty label
  - an empty `Creator` hiding the "Creator" line
  - a failing execution writing nothing (empty recorder body, no header written)
- [x] 4.6 Gate. Verify: `gofmt -l ui internal/web` prints nothing and `go build ./... && go vet ./... && go test ./...` is green.

## 5. [go-dev] HTTP routes, handlers, assets and middleware

- [x] 5.1 Add the `ArticleStore` and `CategoryFeed` interfaces and `NewHandler(log, store, feed, files)` to `internal/web`. `NewHandler` builds a `ServeMux` with `GET /{$}`, `GET /category/{category}`, `GET /post/{id}` and `GET /assets/` (D6). Verify: `go build ./...`.
- [x] 5.2 Add the pure function `paginate(raw, total)` with page size 10. Verify a table test passes for:
  - `("",25)` gives page 1 with next
  - `("3",25)` gives prev and no next
  - `("2",20)` gives no next
  - `"abc"`, `"0"`, `"-2"` and `"4"` with 25 give page 1
  - `("",0)` gives page 1 with no links
- [x] 5.3 Implement the home, category and post handlers per D6. The category handler runs `news.ParseCategory` on the path value: a failure gives 404 before any feed call, and success calls the feed with, and renders, the canonical slug. `news.ErrNotFound` gives 404; other errors give 500 with `http.StatusText`, plus `slog.Error` with method, path and error. Verify: `go build ./...`.
- [x] 5.4 Serve the assets with `noDirFS` + `http.FileServerFS` + `http.StripPrefix("/assets", …)` over `fs.Sub(files, "assets")`. Verify: covered by 5.6.
- [x] 5.5 Add the middleware: a request log (method, `r.URL.Path`, status, duration) wrapping recover (500 if headers are unwritten; re-panic `http.ErrAbortHandler`). Verify: covered by 5.6.
- [x] 5.6 Add handler tests with a fake store, a fake feed, `ui.Files` and a buffer-backed slog. Verify `go test ./internal/web` passes and covers:
  - **Methods and paths:** GET/HEAD `/` (200, same `Content-Type`, empty HEAD body); `POST /`, `PUT /category/business` and `DELETE /post/abc` give 405 with `Allow` containing GET and HEAD; `/nope`, `/category/` and `/post/a/b` give 404.
  - **Home:** page links with 25 articles; a store error gives 500 with body `Internal Server Error` and no error text, and the error is logged.
  - **Category:**
    - `/category/business`, `/category/Business` and `/category/BUSINESS` give 200 with breadcrumb `business`, and the fake feed receives `business`
    - `politics` and `%3Cscript%3E` give 404 with a body that does not contain the input, and the feed is never called
    - a feed error gives a generic 500
  - **Post:** found gives 200; not found gives 404; a store error gives 500.
  - **Panic:** gives 500, and the next request is 200.
  - **Assets:** `/assets/css/style.css` gives 200 `text/css`; `/assets/`, `/assets/css/` and `/assets/css` give 404 with no listing; `/assets/css/owl.carousel.min.css` gives 404.
  - **Logging:** one log record per request, and `/?page=2` is logged with path `/`.
  - **Real feed:** a `feed.New` with a counting fake `Fetcher` shows that `/category/BUSINESS` then `/category/business` make one fetch for `business`, and that `/category/politics` makes zero fetches.
- [x] 5.7 Group 4 review follow-ups (non-blocking):
  - `pages_test.go` TestEscaping: in the hostile run, also set `categoryView.Category` and `Category[0]` to `<script>alert(1)</script>`, which pins the "values from the request" part of the Output escaping requirement at the template layer
  - `render.go`: drop the speculative `status int` parameter (every caller renders 200) and write `http.StatusOK`
  - 5.6 adds a handler-level "Rendering failure" test (spec news-browsing): a page whose execution fails gives 500 with `http.StatusText` only, plus a `slog.Error` record with method, path and error
- [x] 5.8 Gate. Verify: `gofmt -l internal/web` prints nothing and `go build ./... && go vet ./... && go test ./...` is green.

## 6. [go-dev] Cut-over: new main, legacy removal, drop gorilla/mux and driver v1

- [x] 6.1 Add `cmd/tengrinews/config.go` with `loadConfig(getenv)`. Verify tests pass for:
  - both vars missing gives an error naming both and containing neither value
  - one var missing names only that var
  - both set gives a config
- [x] 6.2 Add `cmd/tengrinews/main.go` per D8:
  - slog JSON to stdout, and `run(ctx, logger, getenv)` with the signal context
  - `mongostore.Open` with a 10 s context and a deferred `Close`
  - the newsdata client with a 10 s timeout, `feed.New`, and `web.NewHandler(..., ui.Files)`
  - the newsdata `http.Client` sets `CheckRedirect` to return `http.ErrUseLastResponse`. Go forwards `X-ACCESS-KEY` on cross-host redirects, so a 3xx must fail the 200 check instead of being followed (group 1 review finding).
  - `newServer` and `serve(ctx, srv, ln, shutdownTimeout)` on `:8080`

  Verify: `go build ./...`.
- [x] 6.3 Add runtime tests. Verify `go test ./cmd/tengrinews` passes and covers:
  - `run` with a missing env var returns an error before listening
  - `run` with `MONGODB_URI=mongodb://user:s3cr3t@127.0.0.1:1/?serverSelectionTimeoutMS=300` returns an error, and the captured logs lack `s3cr3t`
  - `serve` on `127.0.0.1:0`: an in-flight 200 ms request completes after the ctx is cancelled and `serve` returns nil
  - a handler that blocks past the shutdown timeout makes `serve` return an error
  - `newServer` sets ReadHeaderTimeout 5s, ReadTimeout 10s, WriteTimeout 20s and IdleTimeout 60s
  - the newsdata `http.Client` built in main has `Timeout == 10*time.Second`. It is the only bound on a refresh, because feed uses `WithoutCancel` (group 2 review finding).
  - the newsdata `http.Client` built in main does not follow a 302 to another host: the key header never reaches the redirect target, and the fetch returns an error
- [x] 6.4 Delete the legacy code:
  - `cmd/main.go`
  - `internal/api`, `internal/domain`, `internal/handler`, `internal/helpers` (incl. `CutImageData`), `internal/models`, `internal/render` and `internal/usecase`
  - `ui/index.html`, `ui/category.html` and `ui/post_details.html`
  - `internal/web/golden_record_test.go`

  Keep `internal/web/testdata/golden/*`. Verify:
  - `go build ./...` is green
  - the golden test still passes against the recordings
  - no `.go` file imports `text/template`, `github.com/gorilla/mux` or a `go.mongodb.org/mongo-driver/` path without `/v2`
  - no `.go` file mentions `source_url` or `CutImageData`
- [x] 6.5 Run `go mod tidy`. Verify `go list -m all`:
  - shows no `github.com/gorilla/mux`
  - shows no line starting `go.mongodb.org/mongo-driver ` (v1)
  - shows `go.mongodb.org/mongo-driver/v2 v2.9.2`
  - shows `github.com/klauspost/compress` >= `v1.18.7`
- [x] 6.6 Update the README:
  - Technologies: stdlib `net/http` and MongoDB Go driver v2; remove Gorilla Mux
  - Setup: `go run ./cmd/tengrinews`, the same two env vars, runs from any directory
  - "Need to Improve": the search line becomes "Search (future work)"

  Verify: the documented run command starts the app.
- [ ] 6.7 Manual smoke test with real env vars. Build with `go build -o <tmp>/ ./cmd/tengrinews` (the trailing slash makes Go name it `tengrinews.exe` on Windows) and run it from a directory without `ui/`. Verify:
  - `/` and `/?page=2` give 200, and every page shows the unified footer and the `Tengrinews` title
  - `/category/business` then `/category/Business` both give 200, with only one upstream fetch logged; this also checks that `language=en%2Cru` is accepted
  - `/post/<known id>` gives 200
  - `/post/missing` and `/category/politics` give 404
  - `POST /` gives 405
  - `/assets/` gives 404 and `/assets/css/style.css` gives 200
  - logs are JSON and contain no key or URI
  - Ctrl+C exits 0 after the Mongo disconnect

  Status (developer, no real credentials available, `NEWSDATA_API_KEY` and `MONGODB_URI` unset): the steps were run against a fake MongoDB (handshake, ping, count, list, find) and a dummy key, with the real binary started from a directory without `ui/`, and all passed: `/`, `/?page=2` and `/?page=3` 200 with the unified footer and `Tengrinews` title, `/post/<id>` 200, `/post/missing` and `/category/politics` 404, `POST /` 405, `/assets/` 404, `/assets/css/style.css` 200, JSON logs without key or URI, no write or index command sent, and an interrupt (CTRL_BREAK) exits 0 with both Mongo connections closed. Missing-env and unreachable-Mongo fail-fast (exit 1 in 10 s, no secret in output) also passed. Pending manual smoke by the user, with real credentials:
  - `/category/business` then `/category/Business` both 200 from the real API (accepts `language=en%2Cru`), with one upstream fetch: the logs show exactly one `category refreshed` record (`category` `business`, `articles` count, `duration`) for the two requests.
  - `/` and `/post/<known id>` against the real data
  - the app running as the read-only Mongo user
  So 6.7 stays unticked.
- [x] 6.8 Group 3 review follow-ups (non-blocking, done here so the group 6 review covers them):
  - `internal/mongostore/store.go`, the Connect-error comment: for `mongodb+srv://`, parsing the URI also does SRV/TXT DNS lookups, so the comment must not claim "no I/O"
  - `internal/mongostore/store.go`, the Timeout comment: v2 replaces the client Timeout with any ctx deadline, so it "applies only when the caller's ctx has no deadline"
  - `store_integration_test.go`: also seed `bson.M{}` and `bson.M{"articles": bson.A{}}`. Count stays 12 and the pages are unchanged.
  - README index section: create the index while connected as a user that may create indexes (e.g. `dbAdmin` on `news`), not as the app's read-only user

  Group 5 review follow-ups (non-blocking), also in `internal/web`:
  - `middleware.go` `statusWriter`: add `Unwrap() http.ResponseWriter`, so `http.ResponseController` and `Flusher` still reach the real writer
  - `middleware.go` recover: if the panic happens after the response started, log it and then `panic(http.ErrAbortHandler)`, so the client sees a dropped connection rather than a truncated page that looks complete. Update `handler_test.go`'s "panic after write" case to match.
  - `middleware.go` request log: `defer` the Info record, so requests that end in a re-panicked `http.ErrAbortHandler` are still logged
  - `handler_test.go`: cover the home `ListArticles` error branch (count succeeds, list fails, 500)

  User decision (2026-10-22), also part of this group:
  - [x] `internal/feed`: on every successful upstream refresh, log one Info record `category refreshed` with `category` (slug), `articles` (count) and `duration` (D5). Test: one cold request gives exactly one such record, and a cached hit gives none. Task 6.7's "one upstream fetch logged" then checks this record.
  - [x] `go get golang.org/x/text@v0.41.0 golang.org/x/crypto@v0.56.0`, then `go mod tidy`. Verify: go.mod says `go 1.26.0` with no `toolchain` line (unless Go adds one, which is acceptable). The driver stays at v2.9.2 and x/sync at v0.21.0, unless the bump forces x/sync up, in which case report it. `GOTOOLCHAIN=go1.27.3 govulncheck ./...` no longer lists GO-2026-6629 or GO-2026-6355/6354/6303. GO-2026-5932 has no fix and may stay. Result: x/sync was forced to v0.22.0, because `golang.org/x/text@v0.41.0` requires it. This supersedes the v0.21.0 pin in 2.1. govulncheck reports only GO-2026-5932, which is unreachable.

  Group 6 review follow-ups (non-blocking):
  - [x] `cmd/tengrinews/main.go`: add `context.AfterFunc(ctx, stop)` right after `signal.NotifyContext`, so a second Ctrl+C during the drain kills the process at once
  - [x] README: build with `go build ./cmd/tengrinews`, then run `./tengrinews` (`.\tengrinews.exe` on Windows) from any directory. No explicit `-o`, because `-o tengrinews` gives no `.exe` on Windows.
- [x] 6.9 Gate. Verify: `gofmt -l .` prints nothing and `go build ./... && go vet ./... && go test ./...` is green.

## 7. [go-dev] CI workflow and acceptance gates

- [x] 7.1 Add `.github/workflows/ci.yml` per D9:
  - `push` + `pull_request` triggers, `permissions: contents: read`, `ubuntu-latest`
  - `actions/setup-go` with `go-version: stable` and `check-latest: true`
  - a `services.mongodb` container (`mongo:8.0`, port 27017, `mongosh` ping health check) and job env `TEST_MONGODB_URI: mongodb://localhost:27017`
  - steps: the gofmt check (fails and lists files), `go vet ./...`, `go test -race ./...` and `go run golang.org/x/vuln/cmd/govulncheck@<pinned release> ./...`

  Verify locally: the YAML parses, and the workflow includes a step that proves the mongostore integration test ran (PASS, not SKIP), e.g. `go test -race -v -run Integration ./internal/mongostore`. The CI run itself is checked only after the user approves a push; developers do not push.
- [x] 7.2 Add a README "Tests and CI" section:
  - `go test ./...` locally (no `-race` without cgo)
  - the optional `TEST_MONGODB_URI` integration test, which skips locally when unset
  - the four CI checks and the MongoDB service container

  Also add `/tengrinews` and `/tengrinews.exe` to `.gitignore`, so the binary that the README build leaves in the clone root can't be committed (group 6 review).

  Verify: the documented commands run as written.
- [x] 7.3 Run the acceptance gates and put their outputs in the group 7 report, for the PR description later. Verify all pass:
  - `gofmt -l .` prints nothing
  - `go vet ./...` is clean
  - locally `go test ./...` passes; `go test -race ./...` passes in CI (CI only)
  - `govulncheck ./...` reports no reachable vulnerabilities (only the unfixable, uncalled module-level GO-2026-5932 is listed), when run with the latest patch toolchain, e.g. `GOTOOLCHAIN=go1.27.3 go run golang.org/x/vuln/cmd/govulncheck@<pinned release> ./...`
  - `go list -m all` shows neither `github.com/gorilla/mux` nor `go.mongodb.org/mongo-driver` (v1)
- [x] 7.4 Spec coverage check. Map every scenario in `specs/news-browsing`, `specs/category-feed` and `specs/service-runtime` to a named test or a 6.7 smoke step, and add a test for any gap. Also add the in-package feed test for `refresh`'s cache re-check (a fresh entry gives zero fetches; group 2 review). Verify: the mapping is in the group 7 report with no unmapped scenario.
- [x] 7.5 Final-review polish (user-approved 2026-10-22):
  - `internal/web/handler.go`: stop re-wrapping errors that the store and feed already prefix. Logs currently read `count articles: count articles: …`. Pass `err` straight to `h.fail`, since the record already carries method and path. Update any test that asserts the old text.
  - `internal/mongostore/store.go`: change the fixed Connect error text to `invalid MONGODB_URI or SRV lookup failed`, so a `mongodb+srv://` DNS failure at boot doesn't read as a pure config error. It must still contain no part of the URI. Update the tests that match the old text.
  - README "Tests and CI": change "govulncheck reports no vulnerabilities" to "reports no reachable vulnerabilities". It still prints module-level GO-2026-5932 (no fix, not called).
  - `internal/mongostore/store_fake_test.go`: add a one-line comment at the top saying the fake speaks the driver's handshake (legacy OP_QUERY hello, `maxWireVersion` 21) and may need updating when the driver is bumped.

  Verify: `gofmt -l .` prints nothing, and `go build ./... && go vet ./... && go test ./...` is green.

## Workflow follow-up

- The owner rotates the newsdata.io key and the MongoDB credentials that were in git history before `a34419e`.
- Optionally, before deploy, create the `articles.article_id` index (README).
- Archive the change after review (`/opsx:archive`).
