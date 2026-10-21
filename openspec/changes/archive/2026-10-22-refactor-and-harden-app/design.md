# Design

## Context

See proposal.md (Why, Decisions) for motivation and the binding decisions. Requirements are in `specs/news-browsing`, `specs/category-feed` and `specs/service-runtime`; this document only says how.

Current state, verified in code on 2026-10-22:

- `cmd/main.go` wires gorilla/mux, `helpers.InitMongoDB()` (driver v1.14.0) and `domain.MockArticleRepository`, which is the real implementation despite its name.
- `internal/handler` holds the home-page aggregation, an unlocked global `categoryCache` map (data race, never expires, any URL value becomes a key and is sent upstream), and maps errors with `err.Error() == "not found"` string checks.
- `api.FetchByIDMongo` streams the whole collection through a goroutine that leaks when a match returns early; the "not found" error is a plain string, so unknown ids give 500.
- `api.FetchDataByCategory` puts the key in the query string, prints the URL with `fmt.Println`, ignores the HTTP status, and races on `err`. `FetchAllArticles`, `FetchByID`, `FetchDataByID`, `FetchDataBySearch` and the `GetAll`/`GetBySearch` paths are unreachable.
- `render` parses `ui/*.html` with `text/template` in `init()` via relative paths and ignores `Execute` errors. `safeHTML` is never used.
- `helpers.CutImageData` is called in two places, but its return values are discarded, so it is a no-op.
- `Post.Link` has bson tag `source_url`; the list aggregation projects `$articles.link` into `source_url`. No template renders `Link`, so the field and the projection are dead and are dropped (user decision).
- `{{index .Category 0}}` (home, category) and `{{index .Post.Category 0}}` (post) fail on empty slices.
- Templates are three full copies that have drifted (see D7).
- Toolchain facts: go.mod says `go 1.25.0`; local toolchain is go1.27.0 with `CGO_ENABLED=0` and no C compiler; latest Go is 1.27.3. `go.mongodb.org/mongo-driver/v2@v2.9.2` declares `go 1.25.0` and requires `klauspost/compress v1.19.2` and `golang.org/x/sync v0.21.0`.
- newsdata.io, probed without a key on 2026-10-22: `/api/1/news` is live, the `X-ACCESS-KEY` header is read, and errors look like `{"status":"error","results":{"message":"...","code":"Unauthorized"}}` with HTTP 401.

## Goals / Non-Goals

**Goals:**
- Each package has one responsibility. Interfaces live in the package that consumes them. Every external dependency (Mongo, newsdata, clock) can be faked in unit tests.
- The work lands in seven groups that each leave `go build ./... && go vet ./... && go test ./...` green. Old and new code live side by side until the cut-over group.

**Non-Goals (design level, in addition to the proposal's):**
- No service/usecase layer: handlers call the store and the feed directly through two small interfaces.
- No styled error pages; error bodies are plain `http.StatusText`.
- No `$sort` on the home page; storage order is kept as today.
- No retry/backoff policy beyond "try again on the next request".

## Decisions

### D1. Package layout (by responsibility)

```
cmd/tengrinews/        main.go (wiring, server lifecycle), config.go (env loading)
internal/news/         domain: Article, Category, Categories(), ParseCategory, ErrNotFound
internal/mongostore/   read-only Mongo v2 store: Open, Close, CountArticles, ListArticles, ArticleByID
internal/newsdata/     newsdata.io HTTP client: Latest(ctx, category)
internal/feed/         category cache: 1h TTL, stale-on-error, single-flight refresh; defines Fetcher
internal/web/          ServeMux routes, handlers, pagination, rendering, assets, middleware;
                       defines ArticleStore and CategoryFeed
ui/                    ui.go (`//go:embed templates all:assets` -> var Files embed.FS),
                       templates/{layout.html, partials/*.html, pages/*.html}, assets/ (unchanged)
```

Dependency direction: `cmd -> {web, feed, newsdata, mongostore, ui}`, `web -> news`, `feed -> news`, `newsdata -> news`, `mongostore -> news`. No package imports `web` or `cmd`. Config stays in `package main`: it is two env vars, so a package would add an import and nothing else.

Alternatives considered: keep `domain/usecase/handler`. Rejected because the usecase layer only forwards calls, and `domain` currently means "the implementation".

### D2. One domain type for articles

`news.Article{ID, Title, Description, Content, ImageURL, PubDate string; Category, Creator []string}` replaces `models.Article`, `models.Post`, `Result`, `ResultMongo` and `ArticleImages`. It holds only rendered fields; `Link`/`source_url` is gone. The wire formats (bson, newsdata JSON) are private structs inside `mongostore` and `newsdata`, mapped to `news.Article`. The newsdata mapping fills only `Title`, `ImageURL` and `Category`, the only fields the category page renders.

`news.ParseCategory(raw string) (Category, bool)` is the one place a requested category is checked. It applies `strings.ToLower`, matches the result exactly against the nine slugs, and returns the canonical `Category`. It deliberately does not use `strings.EqualFold`, which folds `ſ` (long s) to `s`. `strings.ToLower` itself maps two non-ASCII runes to ASCII: `İ` (U+0130) becomes `i` and Kelvin `K` becomes `k`. So `BUSİNESS` is accepted as `business`. That is harmless, because only the canonical slug ever leaves `ParseCategory`.

### D3. Mongo store (driver v2, read-only)

- `Open(ctx, uri, database)` runs `mongo.Connect(options.Client().ApplyURI(uri).SetTimeout(5*time.Second))` and then `Ping(ctx)`. The caller's ctx carries the 10 s startup deadline. If the ping fails, Open disconnects. For any `mongo.Connect` error, Open returns the fixed error `invalid MONGODB_URI or SRV lookup failed`, because driver text may echo URI parts. For `mongodb+srv://`, parsing includes the SRV/TXT DNS lookup, which is why the text says "or SRV lookup failed". Open wraps ping errors.
- Client-level `Timeout` (CSOT) bounds every operation to 5 s. Each call also gets the request context, so a client disconnect cancels it.
- `CountArticles`: `[{$group:{_id:null, n:{$sum:{$cond:[{$isArray:"$articles"},{$size:"$articles"},0]}}}}]`, returning 0 when there are no documents. This avoids unwinding the collection just to count.
- `ListArticles(skip, limit)`: `[{$unwind:"$articles"},{$replaceRoot:{newRoot:"$articles"}},{$skip},{$limit},{$project:{article_id,title,category,image_url}}]`. No `$sort`, matching today's order. `allowDiskUse` is dropped because no stage blocks.
- `ArticleByID(id)`: `FindOne({"articles.article_id": id}, projection {"articles.$": 1})` returns only the matching element. `mongo.ErrNoDocuments` or an empty array becomes `news.ErrNotFound`. The documented index `db.articles.createIndex({"articles.article_id": 1})` (db `news`) makes this an index lookup.
- The production database name `news` is a constant in `main`. The integration test passes a throwaway database.

Alternative: aggregation with `$match`/`$unwind`/`$match` for the lookup. Rejected because the positional projection is one round trip, easier to read, and uses the same index.

### D4. newsdata client

`newsdata.New(baseURL, apiKey string, hc *http.Client)` with `DefaultBaseURL = "https://newsdata.io/api/1/news"`; main passes `&http.Client{Timeout: 10*time.Second, CheckRedirect: …}`. `CheckRedirect` returns `http.ErrUseLastResponse`, because Go forwards custom headers such as `X-ACCESS-KEY` on cross-host redirects. A 3xx therefore fails the 200 check instead of being followed. `Latest(ctx, category)`:
1. Builds the query with `url.Values{"category", "language": "en,ru"}`, so the comma is sent encoded as `%2C`.
2. Sets `X-ACCESS-KEY`.
3. Requires HTTP 200.
4. Decodes at most 4 MiB as `{status string, results json.RawMessage}`.
5. Requires `status == "success"`.
6. Decodes `results` into `[]wireArticle`.

Errors carry the category, the HTTP status and the upstream `results.code` when present. They never carry the key, headers or body text. The key is not part of the URL, so `*url.Error` cannot leak it.

### D5. Category feed cache

`feed.New(f Fetcher, log *slog.Logger, now func() time.Time) *Cache`, `const TTL = time.Hour`, with `Fetcher` defined in `feed` and implemented by `*newsdata.Client`. `Articles(ctx, category)`:
1. `news.ParseCategory(category)`. If it fails, return `news.ErrNotFound`. No fetch happens and no entry is created, so the map can never hold more than nine keys. From here on, only the canonical lowercase slug is used, as the map key, the single-flight key and the `Latest` argument.
2. Read the entry under a `sync.Mutex`. If it exists and `now() - fetchedAt < TTL`, return it.
3. Otherwise run `singleflight.Group.Do(slug, refresh)`. `refresh` calls `Latest(context.WithoutCancel(ctx), slug)`, so one waiter disconnecting does not cancel the shared refresh; the 10 s HTTP client timeout still bounds it. `WithoutCancel` also drops the caller's deadline, so that client timeout is the only bound. `refresh` first re-checks the cache and returns a fresh entry without fetching, which closes the window where a caller saw an expired entry just as the previous flight finished. On success it stores `{articles, now()}` and logs one Info record `category refreshed` with `category` (slug), `articles` (count) and `duration`. That lets the owner track upstream calls against the newsdata.io free-tier quota (user decision 2026-10-22).
4. On refresh error with an existing entry, log a warning (category, error) and return the stale articles. With no entry, return the error.

Returned slices are shared and must be treated as read-only. That holds today: templates only read them.

`golang.org/x/sync/singleflight` is the one new direct import. Its module is already in the graph (driver v1 and v2 both require x/sync), and it saves a hand-rolled per-key wait group. The reason is the quota: without single-flight, a cold or expired category under load sends N upstream calls.

Alternative: one mutex held across the fetch. Rejected because it serializes all categories behind one slow upstream call.

### D6. Web layer

- `web.NewHandler(log, store ArticleStore, feed CategoryFeed, files fs.FS) (http.Handler, error)` parses templates (any failure means startup fails) and builds a `*http.ServeMux` with these patterns:
  - `GET /{$}` for home
  - `GET /category/{category}`
  - `GET /post/{id}`
  - `GET /assets/`

  A `GET` pattern also matches HEAD. Other methods get 405 with `Allow`, and unknown paths get 404, both from the stdlib mux.
- Consumer interfaces:
  - `ArticleStore{CountArticles; ListArticles; ArticleByID}`
  - `CategoryFeed{Articles(ctx, slug)}`
- Home: `total := CountArticles` → `p := paginate(r.URL.Query().Get("page"), total)` → `ListArticles((p.Current-1)*10, 10)` → render. `paginate` is a pure function: `totalPages = ceil(total/10)`, and an Atoi error, `< 1` or `> totalPages` gives 1.
- Category: `news.ParseCategory(r.PathValue("category"))`. A failure gives 404 before any I/O, and the raw value is never rendered. Otherwise the handler calls `feed.Articles(ctx, c.Slug)` and renders `c.Slug` in the breadcrumb.
- Category and post: `errors.Is(err, news.ErrNotFound)` gives 404. Any other error gives 500.
- Errors: `http.Error(w, http.StatusText(code), code)`. The underlying error is logged with `slog.Error` along with method and path.
- Middleware, outermost first:
  - request log: one `Info` record with method, `r.URL.Path`, status and duration.
  - recover: logs the panic and stack, and answers 500 if headers were not yet written. If the response had already started, it panics with `http.ErrAbortHandler` after logging, so net/http drops the connection and the client never sees a truncated page that looks complete. It re-panics `http.ErrAbortHandler`. The request-log record is deferred, so aborted requests are logged too. Both wrappers' `statusWriter` implements `Unwrap`, so `http.ResponseController` reaches the real writer.
- Assets: `http.StripPrefix("/assets", http.FileServerFS(noDirFS{sub}))`, where `sub` is `fs.Sub(files, "assets")` and `noDirFS.Open` returns `fs.ErrNotExist` for directories. Directory URLs therefore 404 and listings can't be produced.

Alternative: a suffix check for `/`. Rejected because `FileServer` would first redirect `/assets/css` to `/assets/css/`.

### D7. Templates and rendering

- `html/template`, parsed once in `NewHandler`. For each page: `template.ParseFS(files, "templates/layout.html", "templates/partials/*.html", "templates/pages/<page>.html")`, executing `"layout"`. The layout renders `{{template "header" .}}`, `{{template "main" .}}` and `{{template "footer" .}}`. Each page file defines only `"main"`; there are no per-page overrides of the frame.
- Render: execute into a `bytes.Buffer`. On error, log it and answer 500, so no partial page goes out. On success, set `Content-Type: text/html; charset=utf-8`, write the status and copy the buffer.
- View models: `homeView`, `categoryView` and `postView` keep today's field names (`LatestPosts`, `Articles`, `Post`, `HasNextPage`, …), so page bodies barely change. `Categories` becomes `[]news.Category`, and the nav renders `{{.Slug}}`/`{{.Name}}`, which drops the `lower` func map.
- Empty slices: `{{with .Category}}{{index . 0}}{{end}}` renders an empty label instead of failing.
- Commented-out markup that contains actions (`<!-- <h3><a href="/post/{{.ID}}">…`, `<!-- …{{.Description}}… -->`) is deleted. html/template strips comments from output but still evaluates the actions inside them, so they would pin unused fields and could fail.
- **Today's three copies have drifted:**

  | Fragment | index | category | post |
  |---|---|---|---|
  | `<title>` | Tengrinews | Tengrinews | Magazine News |
  | CSS `href`s in `<head>` | `assets/…` (relative) | `/assets/…` | `/assets/…` |
  | footer `.header-top` classes | `d-lg-block d-none` | `d-none d-lg-block` | `d-none d-lg-block` |
  | footer `.header-info-left` | `d-flex justify-content-center` | `d-flex` | `d-flex` |
  | footer `.header-bottom2` block | absent | present | present |
  | copyright text | "was my by" | "was by" | "was made by" |

  After stripping comments, the `<header>` blocks are already identical across the three pages.
- **Unified frame (user decision): the base is `ui/post_details.html`'s `<head>`, header and footer, with the title `Tengrinews`.** Why post_details:
  - It is the only copy whose copyright wording is correct ("This site was made by").
  - Its footer structure (`d-none d-lg-block`, `d-flex`, `header-bottom2` block) is the one two of three pages already use.
  - All its asset URLs are absolute.

  Why `Tengrinews`: it is the site name and is already on two pages; "Magazine News" is the name of the theme the markup came from (see the `281.Magazine_News` paths in `style.css`). The stray `</a>` at the end of every copyright line has no opening tag and is dropped as part of fixing the copyright typos.
- **Intended visible differences** (the complete list; the golden test allows nothing else):

  | # | Page | Before (normalized recording) | After |
  |---|---|---|---|
  | 1 | home | 14 × `href="assets/css/…"` | `href="/assets/css/…"` |
  | 2 | home | `this.src='assets/img/tengrinews_image.jpeg'` (once per article) | `this.src='/assets/img/tengrinews_image.jpeg'` |
  | 3 | home | footer `class="header-top d-lg-block d-none"` | `class="header-top d-none d-lg-block"` |
  | 4 | home | footer `class="header-info-left d-flex justify-content-center"` | `class="header-info-left d-flex"` |
  | 5 | home | footer without the `header-bottom2` block | footer with the post page's `header-bottom2` block |
  | 6 | home | `This site was my by Arshat Aitkozha</a>` | `This site was made by Arshat Aitkozha` |
  | 7 | category | `This site was by Arshat Aitkozha</a>` | `This site was made by Arshat Aitkozha` |
  | 8 | post | `<title>Magazine News</title>` | `<title>Tengrinews</title>` |
  | 9 | post | `This site was made by Arshat Aitkozha</a>` | `This site was made by Arshat Aitkozha` |
- **Golden test:**
  1. Before any template is touched, a recorder records today's output for fixed fixtures. The recorder is a test file behind the build tag `goldenrecord` that runs the current `ui/*.html` through `text/template`. The output goes to `internal/web/testdata/golden/{home,category,post}.html`.
  2. The recordings are never edited by hand and are never re-recorded after the cut-over, when the old templates are gone.
  3. The regular test normalizes both sides: strip `<!--…-->`, then drop whitespace adjacent to tags and collapse remaining whitespace runs. Comment stripping is forced by html/template; whitespace normalization is forced by moving markup into partials.
  4. The test holds the table above as code: per page, an ordered list of `{old, new string; count int}` on the normalized text. Each pair must occur exactly `count` times in the normalized recording, or the test fails, so the list can't go stale or match more than intended. All pairs are applied and the result must equal the normalized new rendering. On mismatch the test prints the first differing offset with context.
  5. Any drift not in the table therefore fails the test, from group 4 through the cut-over and after.
  6. The fixtures are "benign": no `<>&'"+`, plain `https` image URLs.
- Embedding: `ui/ui.go` uses `//go:embed templates all:assets`. `all:` keeps the 19 `_*.scss` files that are served today; plain `assets` would drop them.

### D8. Runtime

`main` builds `slog.New(slog.NewJSONHandler(os.Stdout, nil))` and calls `run(ctx, logger, os.Getenv)`. If `run` returns an error, main logs it and calls `os.Exit(1)`. `run` does the following, in order:
1. `loadConfig(getenv)` returns an error naming the missing vars. Values are never logged.
2. `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`, then `context.AfterFunc(ctx, stop)`. The first signal restores default handling, so a second Ctrl+C during the drain kills the process at once. `syscall.SIGTERM` also exists on Windows, so the build stays portable.
3. `mongostore.Open` with a 10 s context. A deferred `Close` uses a fresh 10 s context.
4. Build the newsdata client, then `feed.New(client, logger, time.Now)`, then `web.NewHandler(..., ui.Files)`.
5. `newServer(handler, logger)` returns `http.Server{ReadHeaderTimeout: 5s, ReadTimeout: 10s, WriteTimeout: 20s, IdleTimeout: 60s, ErrorLog: slog.NewLogLogger(h, slog.LevelWarn)}`. `WriteTimeout` is set above the 10 s upstream plus 5 s Mongo bounds.
6. `net.Listen("tcp", ":8080")`, then `serve(ctx, srv, ln, 10*time.Second)`. `serve` runs `srv.Serve(ln)` in a goroutine and waits for either its error or `ctx.Done()`. It is split out so tests can drive it on `127.0.0.1:0`.
7. On signal, `serve` calls `srv.Shutdown` with the shutdown timeout. A deadline overrun returns an error, which means a non-zero exit. The deferred `Close` then disconnects Mongo.

No value from config is ever passed to the logger.

### D9. Dependencies and CI

- Final go.mod:
  - adds `go.mongodb.org/mongo-driver/v2 v2.9.2` and `golang.org/x/sync` (direct)
  - removes `github.com/gorilla/mux` and `go.mongodb.org/mongo-driver` (v1)
  - selects `klauspost/compress v1.19.2` through MVS from driver v2.9.2, which satisfies ">= 1.18.7" with no explicit pin
  - sets the `go 1.26.0` directive. The driver's minimum is 1.25.0, but the user chose `golang.org/x/crypto` v0.56.0 (needs 1.26.0) and `golang.org/x/text` v0.41.0 to clear module-level advisories GO-2026-6355/6354/6303 and GO-2026-6629 (user decision 2026-10-22).
- `.github/workflows/ci.yml` runs on `push` and `pull_request` with `permissions: contents: read`, on `ubuntu-latest` with `actions/setup-go` `go-version: stable` and `check-latest: true`.
  - Service container (user decision): `services.mongodb` with
    - `image: mongo:8.0`
    - `ports: ["27017:27017"]`
    - a health check, `--health-cmd "mongosh --quiet --eval 'db.runCommand({ping:1})'"` with interval, timeout and retries, so the job starts only when Mongo answers

    The job env sets `TEST_MONGODB_URI: mongodb://localhost:27017`. The container needs no auth, because the test only writes to its own throwaway database.
  - Steps:
    1. `test -z "$(gofmt -l .)"` (prints the offending files on failure)
    2. `go vet ./...`
    3. `go test -race ./...`, which now includes the Mongo integration test
    4. `go run golang.org/x/vuln/cmd/govulncheck@<pinned release> ./...`
  - The race detector runs only here (user decision). The local gate is `go test ./...` without `-race`, because this Windows machine has no cgo/C compiler.

  It uses the latest stable Go, not `go-version-file`, because govulncheck checks the stdlib of the toolchain it runs on, and go.mod's 1.26.0 is an unpatched `.0` release.
- The integration test skips when `TEST_MONGODB_URI` is unset, unless the `CI` env var is set (GitHub Actions sets `CI=true`). In CI an unset URI is a test failure, so a broken workflow env can't turn the test into a silent skip.
- `.gitattributes` with `*.go text eol=lf`. Git stores LF, but this machine has `core.autocrlf=true`, so every Go file is checked out as CRLF and `gofmt -l .` lists all of them. That makes the local gofmt gate disagree with CI.

## Components and contracts

### HTTP surface (consumed by browsers)

| Route | Methods | 200 | 404 | 405 | 500 |
|---|---|---|---|---|---|
| `/` (`?page=` optional int) | GET, HEAD | home HTML | — | other methods | count/list/render failure |
| `/category/{category}` | GET, HEAD | category HTML (any letter case of a known slug) | lowercased value not one of the nine | other methods | feed failure with no cached copy, or render failure |
| `/post/{id}` | GET, HEAD | post HTML | no article with that id | other methods | store or render failure |
| `/assets/{path}` | GET, HEAD | file bytes, type by extension | missing file or directory | other methods | — |
| anything else | any | — | always | — | — |

HTML responses are `text/html; charset=utf-8`. Error bodies are `text/plain`, `http.StatusText(code)` from our handlers or the stdlib mux's default text. Neither contains input or internal text.

### Internal Go contracts

```go
// internal/web
type ArticleStore interface {
    CountArticles(ctx context.Context) (int, error)
    ListArticles(ctx context.Context, skip, limit int) ([]news.Article, error)
    ArticleByID(ctx context.Context, id string) (news.Article, error) // news.ErrNotFound when absent
}
type CategoryFeed interface {
    Articles(ctx context.Context, slug string) ([]news.Article, error) // news.ErrNotFound for unknown slug
}

// internal/feed
type Fetcher interface {
    Latest(ctx context.Context, category string) ([]news.Article, error)
}
```

### Data model (read only)

MongoDB `news.articles` documents. Only these fields are read; others, including `link`, are ignored. All strings may be missing, which decodes to `""`. Arrays may be missing or empty.

```
{ _id: ObjectId,
  articles: [ { article_id: string, title: string, description: string,
                content: string, pubDate: string, image_url: string,
                category: [string], creator: [string] } ] }
```

newsdata.io `GET /api/1/news?category=<slug>&language=en,ru` with header `X-ACCESS-KEY: <key>`:
- Success: `200 {"status":"success","totalResults":int,"results":[{"article_id":string,"title":string,"link":string,"image_url":string|null,"category":[string],…}],"nextPage":string|null}`.
- Error: non-200 (401, 422, 429, 5xx) with `{"status":"error","results":{"message":string,"code":string}}`.
- Only `title`, `image_url` and `category` are read. `null` decodes to `""`.

## Main flows and failure cases

| Situation | Result |
|---|---|
| Missing env var | Error naming the var(s); exit 1; never listens |
| Mongo unreachable at startup | Ping fails within 10 s; exit 1; error has no credentials |
| Template parse error | Exit 1 |
| Mongo slow or down during a request | CSOT 5 s → 500 generic; error logged |
| Client disconnects mid-request | Request ctx cancels the Mongo op; the feed refresh continues for other waiters |
| newsdata 401/429/5xx/timeout/bad JSON, cached copy exists | Stale copy served, 200, `Warn` logged |
| Same failure, no cached copy | 500 generic, `Error` logged |
| Unknown category / post id | 404, no upstream call, nothing reflected |
| Bad `?page=` | Page 1 |
| Handler panic | 500, stack logged, process continues |
| SIGINT/SIGTERM | Shutdown ≤ 10 s, Mongo disconnect, exit 0 (1 if the deadline is exceeded) |

## Security

- XSS: html/template contextual escaping applies everywhere. No `template.HTML` and no `safeHTML`. Unsafe URL schemes become `#ZgotmplZ`.
- Secrets:
  - The key travels only in the `X-ACCESS-KEY` header.
  - Neither the key nor the URI is ever logged.
  - The URI parse error is replaced with a fixed message.
  - Tests assert the absence of a marker secret in logs.
- Input:
  - The category is lowercased with `strings.ToLower` and must exactly match an allow-list entry before any I/O. Only the canonical slug, never the raw value, reaches the cache, newsdata.io or the page.
  - The post id is used only as a string equality value in a bson filter, so no operator injection is possible.
  - The page number is parsed as an int.
- Exposure: no directory listings, no internal error text, a bounded upstream body (4 MiB), and server timeouts against slowloris.
- Read-only DB user is sufficient. The integration test needs write access only to its own throwaway database.
- Out of code scope: the key and URI in git history before `a34419e` must be rotated by the owner.

## Risks / Trade-offs

- **Markup differs for non-benign data.** html/template escapes `'` `+` `&` (`&#39;` `&#43;` `&amp;`) and drops comments, so markup differs from today for such data. → Rendering is identical; the golden test pins benign output, and separate tests pin escaping.
- **The intended-difference list could hide drift.** A broad replacement pair could hide unintended changes. → Pairs are exact strings with exact occurrence counts, kept as small as the table rows. The one larger pair (row 5, the inserted `header-bottom2` block) is an exact literal.
- **Races only caught in CI.** A data race shows up only in CI, not in local runs. → The cache and store code is small and mutex-guarded; CI runs `-race` on every push and PR.
- **CI vs production MongoDB version.** CI uses `mongo:8.0`, and the user confirmed production runs 8.0 on 2026-10-22. → Only long-standing operators are used anyway (`$unwind`, `$replaceRoot`, `$isArray`, positional projection), so a minor version difference is harmless.
- **govulncheck depends on the toolchain.** Its result follows the stdlib of the running toolchain: local go1.27.0 may report stdlib CVEs fixed in 1.27.1-1.27.3. → Run the local gate as `GOTOOLCHAIN=go1.27.3 go run golang.org/x/vuln/cmd/govulncheck@<pinned> ./...` (or newer); CI uses `stable`.
- **Legacy files fail gofmt locally.** They stay CRLF in the working tree until deleted. → Per-group gofmt gates check only the new packages; the repo-wide `gofmt -l .` gate applies after the cut-over group deletes them.
- **Free-tier quota.** 9 categories × 24 refreshes/day = 216 calls worst case, above a 200/day free tier. → Single-flight prevents duplicates; once the quota is hit, 429 → stale copy is served.
- **Unstable home order.** Storage order without `$sort` is not guaranteed by MongoDB. → Kept for parity; sorting is listed under "can wait".
- **Count and list are not atomic.** If data changes between the two queries, a page may shift by an item. → Harmless for a read-only news list.
- **Comma encoding.** `language=en%2Cru` differs from today's literal comma. → Standard decoding makes them equivalent; manual smoke check against the real API in the cut-over group.
- **Binary size.** It grows by about 3.5 MB of embedded assets. → Acceptable.
- **No asset revalidation.** Embedded files have no modtime, so `Last-Modified`/304 revalidation for assets goes away. → Listed under "can wait" (ETag/Cache-Control).

## Migration Plan

1. (Optional, before deploy) The operator creates the index: `use news; db.articles.createIndex({"articles.article_id": 1})`.
2. Build `go build ./cmd/tengrinews` (it writes `tengrinews`, or `tengrinews.exe` on Windows) and run it with the same two env vars. The run command changes from `go run cmd/main.go` to `go run ./cmd/tengrinews`, and the README is updated. The process may run from any directory.
3. Rotate the newsdata key and Mongo credentials (owner).
4. Rollback: redeploy the previous commit. There is no data or schema migration and the app never writes.

## Can wait

- Styled HTML error pages.
- Refresh backoff during long outages.
- `$sort` by `pubDate`.
- Cache headers for assets.
- Security headers (CSP, etc.).
- The missing `owl.carousel.min.css` and `magnific-popup.css` (404 today, unchanged).
- Health endpoint.

## Resolved with the user (2026-10-22)

- **Q1** Unify the header and footer on the post page's version: title `Tengrinews`, fixed copyright, absolute `/assets/` URLs. The golden test allows only the D7 intended-difference list.
- **Q2** Drop `Link`/`source_url` and its projection.
- **Q3** Categories match case-insensitively: lowercase, then an exact allow-list match.
- **Q4** Remove `CutImageData`; no content cleaning.
- **Q5** The race detector runs in CI only; the local gate is `go test ./...`.
- **Q6** CI runs the integration test against a MongoDB service container.

## Open Questions

None. The production MongoDB version was resolved on 2026-10-22: it is 8.0, matching the CI image `mongo:8.0`.
