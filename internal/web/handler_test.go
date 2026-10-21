package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"tengrinews/internal/feed"
	"tengrinews/internal/news"
	"tengrinews/internal/newsdata"
	"tengrinews/ui"
)

// fakeStore serves a fixed article list. err fails every call; listErr fails only ListArticles.
type fakeStore struct {
	articles []news.Article
	err      error
	listErr  error
}

func (s *fakeStore) CountArticles(context.Context) (int, error) { return len(s.articles), s.err }

func (s *fakeStore) ListArticles(_ context.Context, skip, limit int) ([]news.Article, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	if skip > len(s.articles) {
		skip = len(s.articles)
	}
	return s.articles[skip:min(skip+limit, len(s.articles))], nil
}

func (s *fakeStore) ArticleByID(_ context.Context, id string) (news.Article, error) {
	if s.err != nil {
		return news.Article{}, s.err
	}
	for _, a := range s.articles {
		if a.ID == id {
			return a, nil
		}
	}
	return news.Article{}, news.ErrNotFound
}

// numbered returns n articles titled "Article 01", "Article 02", ... with ids "id01", ...
func numbered(n int) []news.Article {
	out := make([]news.Article, n)
	for i := range out {
		out[i] = news.Article{ID: fmt.Sprintf("id%02d", i+1), Title: fmt.Sprintf("Article %02d", i+1), Category: []string{"business"}}
	}
	return out
}

// fakeFeed records the slugs it is asked for.
type fakeFeed struct {
	calls    []string
	articles []news.Article
	err      error
}

func (f *fakeFeed) Articles(_ context.Context, slug string) ([]news.Article, error) {
	f.calls = append(f.calls, slug)
	return f.articles, f.err
}

// countingFetcher is a feed.Fetcher that records the categories it is asked for.
type countingFetcher struct{ calls []string }

func (f *countingFetcher) Latest(_ context.Context, category string) ([]news.Article, error) {
	f.calls = append(f.calls, category)
	return []news.Article{{Title: "From upstream"}}, nil
}

// panicStore panics once when armed, then behaves like the embedded store.
type panicStore struct {
	*fakeStore
	armed bool
}

func (s *panicStore) CountArticles(ctx context.Context) (int, error) {
	if s.armed {
		s.armed = false
		panic("boom")
	}
	return s.fakeStore.CountArticles(ctx)
}

// newTestHandler builds the site over ui.Files with a buffer-backed JSON logger.
func newTestHandler(t *testing.T, store ArticleStore, cf CategoryFeed) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	h, err := NewHandler(slog.New(slog.NewJSONHandler(&logs, nil)), store, cf, ui.Files)
	if err != nil {
		t.Fatal(err)
	}
	return h, &logs
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func logRecords(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func wantStatusText(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d; body %q", rec.Code, code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != http.StatusText(code) {
		t.Errorf("body = %q, want %q", got, http.StatusText(code))
	}
}

func TestMethodsAndPaths(t *testing.T) {
	h, _ := newTestHandler(t, &fakeStore{articles: numbered(3)}, &fakeFeed{})

	t.Run("GET and HEAD", func(t *testing.T) {
		srv := httptest.NewServer(h) // the HEAD body is dropped by net/http's server, not by the handler
		defer srv.Close()
		get, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer get.Body.Close()
		head, err := http.Head(srv.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer head.Body.Close()
		if get.StatusCode != 200 || head.StatusCode != 200 {
			t.Fatalf("status GET %d, HEAD %d, want 200", get.StatusCode, head.StatusCode)
		}
		if got := get.Header.Get("Content-Type"); got != "text/html; charset=utf-8" || head.Header.Get("Content-Type") != got {
			t.Errorf("Content-Type GET %q, HEAD %q", got, head.Header.Get("Content-Type"))
		}
		var body bytes.Buffer
		if _, err := body.ReadFrom(head.Body); err != nil || body.Len() != 0 {
			t.Errorf("HEAD body = %d bytes, err %v, want empty", body.Len(), err)
		}
	})

	for _, tt := range []struct{ method, target string }{
		{"POST", "/"}, {"PUT", "/category/business"}, {"DELETE", "/post/abc"}, {"POST", "/assets/css/style.css"},
	} {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			rec := do(h, tt.method, tt.target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			allow := rec.Header().Get("Allow")
			if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
				t.Errorf("Allow = %q, want GET and HEAD", allow)
			}
		})
	}

	for _, target := range []string{"/nope", "/category/", "/post/a/b"} {
		t.Run("404 "+target, func(t *testing.T) {
			if rec := do(h, "GET", target); rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
		})
	}
}

func TestPaginate(t *testing.T) {
	tests := []struct {
		raw   string
		total int
		want  pagination
	}{
		{"", 25, pagination{1, false, true}},
		{"3", 25, pagination{3, true, false}},
		{"2", 20, pagination{2, true, false}},
		{"2", 25, pagination{2, true, true}},
		{"abc", 25, pagination{1, false, true}},
		{"0", 25, pagination{1, false, true}},
		{"-2", 25, pagination{1, false, true}},
		{"4", 25, pagination{1, false, true}},
		{"99999999999999999999", 25, pagination{1, false, true}},
		{"", 0, pagination{1, false, false}},
		{"2", 0, pagination{1, false, false}},
		{"", 10, pagination{1, false, false}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q of %d", tt.raw, tt.total), func(t *testing.T) {
			if got := paginate(tt.raw, tt.total); got != tt.want {
				t.Errorf("paginate = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHome(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		target      string
		first, last int    // listed articles, 1-based; 0 when none
		prev, next  string // page number the link points to; "" when there is no link
	}{
		{name: "first page", total: 25, target: "/", first: 1, last: 10, next: "2"},
		{name: "last partial page", total: 25, target: "/?page=3", first: 21, last: 25, prev: "2"},
		{name: "exact multiple", total: 20, target: "/?page=2", first: 11, last: 20, prev: "1"},
		{name: "middle page", total: 25, target: "/?page=2", first: 11, last: 20, prev: "1", next: "3"},
		{name: "not a number", total: 25, target: "/?page=abc", first: 1, last: 10, next: "2"},
		{name: "zero", total: 25, target: "/?page=0", first: 1, last: 10, next: "2"},
		{name: "negative", total: 25, target: "/?page=-2", first: 1, last: 10, next: "2"},
		{name: "out of range", total: 25, target: "/?page=4", first: 1, last: 10, next: "2"},
		{name: "nothing stored", total: 0, target: "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(t, &fakeStore{articles: numbered(tt.total)}, &fakeFeed{})
			rec := do(h, "GET", tt.target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			for i := 1; i <= tt.total; i++ {
				listed := strings.Contains(body, fmt.Sprintf(`<a href="/post/id%02d">Article %02[1]d</a>`, i))
				if want := i >= tt.first && i <= tt.last; listed != want {
					t.Errorf("article %d listed = %v, want %v", i, listed, want)
				}
			}
			for label, page := range map[string]string{"Previous": tt.prev, "Next": tt.next} {
				link := `href="?page=` + page + `"><strong>` + label + `</strong>`
				if has := strings.Contains(body, "<strong>"+label+"</strong>"); has != (page != "") {
					t.Errorf("%s link shown = %v, want %v", label, has, page != "")
				} else if page != "" && !strings.Contains(body, link) {
					t.Errorf("%s link does not point to page %s", label, page)
				}
			}
		})
	}
}

func TestHomeErrors(t *testing.T) {
	tests := []struct {
		name  string
		store *fakeStore
	}{
		{"count fails", &fakeStore{articles: numbered(25), err: errors.New("db password leaked here")}},
		{"count succeeds, list fails", &fakeStore{articles: numbered(25), listErr: errors.New("db password leaked here")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, logs := newTestHandler(t, tt.store, &fakeFeed{})
			rec := do(h, "GET", "/")
			wantStatusText(t, rec, http.StatusInternalServerError)
			if strings.Contains(rec.Body.String(), "leaked") {
				t.Error("error text reached the client")
			}
			if !strings.Contains(logs.String(), "db password leaked here") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
				t.Errorf("error not logged: %s", logs)
			}
		})
	}
}

func TestCategory(t *testing.T) {
	cf := &fakeFeed{articles: []news.Article{{Title: "Exports grow", Category: []string{"business"}}}}
	h, _ := newTestHandler(t, &fakeStore{}, cf)

	for _, target := range []string{"/category/business", "/category/Business", "/category/BUSINESS"} {
		t.Run(target, func(t *testing.T) {
			rec := do(h, "GET", target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `<li class="breadcrumb-item"><a href="#">business</a></li>`) || !strings.Contains(body, "Exports grow") {
				t.Error("page lacks the breadcrumb or the feed article")
			}
			if got := cf.calls[len(cf.calls)-1]; got != "business" {
				t.Errorf("feed asked for %q, want business", got)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		before := len(cf.calls)
		for _, tt := range []struct{ target, input string }{
			{"/category/politics", "politics"},
			{"/category/%3Cscript%3Ealert(1)%3C%2Fscript%3E", "script"},
			{"/category/%C5%BFports", "ports"}, // long s
		} {
			rec := do(h, "GET", tt.target)
			wantStatusText(t, rec, http.StatusNotFound)
			if strings.Contains(rec.Body.String(), tt.input) {
				t.Errorf("%s: body reflects the input: %q", tt.target, rec.Body.String())
			}
		}
		if len(cf.calls) != before {
			t.Errorf("feed called for unknown categories: %v", cf.calls[before:])
		}
	})

	t.Run("feed error", func(t *testing.T) {
		h, logs := newTestHandler(t, &fakeStore{}, &fakeFeed{err: errors.New("upstream key=SECRET")})
		rec := do(h, "GET", "/category/business")
		wantStatusText(t, rec, http.StatusInternalServerError)
		if strings.Contains(rec.Body.String(), "SECRET") || !strings.Contains(logs.String(), "upstream key=SECRET") {
			t.Errorf("error text in body or missing from log; body %q", rec.Body.String())
		}
	})

	t.Run("feed not found", func(t *testing.T) {
		h, _ := newTestHandler(t, &fakeStore{}, &fakeFeed{err: news.ErrNotFound})
		wantStatusText(t, do(h, "GET", "/category/business"), http.StatusNotFound)
	})
}

func TestPost(t *testing.T) {
	store := &fakeStore{articles: []news.Article{fixturePost}}
	h, _ := newTestHandler(t, store, &fakeFeed{})

	rec := do(h, "GET", "/post/abc123")
	if rec.Code != http.StatusOK {
		t.Fatalf("found: status %d, want 200", rec.Code)
	}
	for _, want := range []string{fixturePost.Title, fixturePost.PubDate, fixturePost.ImageURL, fixturePost.Description, fixturePost.Content} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("post page lacks %q", want)
		}
	}
	wantStatusText(t, do(h, "GET", "/post/missing"), http.StatusNotFound)

	store.err = errors.New("db down")
	rec = do(h, "GET", "/post/abc123")
	wantStatusText(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "db down") {
		t.Error("error text reached the client")
	}
}

func TestRenderingFailure(t *testing.T) {
	// The page fails after it has produced output, at an index that does not exist.
	files := fstest.MapFS{
		"templates/layout.html":         {Data: []byte(`{{define "layout"}}PARTIAL-MARKUP {{index .Categories 99}}{{end}}`)},
		"templates/partials/p.html":     {Data: []byte(`{{define "p"}}x{{end}}`)},
		"templates/pages/home.html":     {Data: []byte(`{{define "main"}}{{end}}`)},
		"templates/pages/category.html": {Data: []byte(`{{define "main"}}{{end}}`)},
		"templates/pages/post.html":     {Data: []byte(`{{define "main"}}{{end}}`)},
	}
	var logs bytes.Buffer
	h, err := NewHandler(slog.New(slog.NewJSONHandler(&logs, nil)), &fakeStore{}, &fakeFeed{}, files)
	if err != nil {
		t.Fatal(err)
	}
	rec := do(h, "GET", "/")
	wantStatusText(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "PARTIAL-MARKUP") {
		t.Error("partial markup reached the client")
	}
	var errRec map[string]any
	for _, r := range logRecords(t, &logs) {
		if r["level"] == "ERROR" {
			errRec = r
		}
	}
	if errRec == nil || errRec["method"] != "GET" || errRec["path"] != "/" || !strings.Contains(fmt.Sprint(errRec["error"]), "execute layout") {
		t.Errorf("error record = %v, want method, path and the render error", errRec)
	}
}

func TestNewHandlerBadTemplates(t *testing.T) {
	if _, err := NewHandler(slog.New(slog.DiscardHandler), &fakeStore{}, &fakeFeed{}, fstest.MapFS{}); err == nil {
		t.Error("NewHandler succeeded without templates")
	}
}

func TestPanic(t *testing.T) {
	store := &panicStore{fakeStore: &fakeStore{articles: numbered(1)}, armed: true}
	h, logs := newTestHandler(t, store, &fakeFeed{})

	wantStatusText(t, do(h, "GET", "/"), http.StatusInternalServerError)
	if !strings.Contains(logs.String(), "boom") || !strings.Contains(logs.String(), "stack") {
		t.Errorf("panic and stack not logged: %s", logs)
	}
	if rec := do(h, "GET", "/"); rec.Code != http.StatusOK {
		t.Errorf("next request status = %d, want 200", rec.Code)
	}
}

func TestRecoverPanics(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	t.Run("aborted response is passed on", func(t *testing.T) {
		h := recoverPanics(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
		defer func() {
			if v := recover(); v != http.ErrAbortHandler {
				t.Errorf("recovered %v, want http.ErrAbortHandler", v)
			}
		}()
		do(h, "GET", "/")
	})

	t.Run("panic after the response started is logged and aborts the connection", func(t *testing.T) {
		var logs bytes.Buffer
		h := recoverPanics(slog.New(slog.NewJSONHandler(&logs, nil)), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("partial"))
			panic("late")
		}))
		rec := httptest.NewRecorder()
		func() {
			defer func() {
				if v := recover(); v != http.ErrAbortHandler {
					t.Errorf("recovered %v, want http.ErrAbortHandler", v)
				}
			}()
			h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		}()
		if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
			t.Errorf("status %d body %q, want only the bytes already written", rec.Code, rec.Body.String())
		}
		if !strings.Contains(logs.String(), "late") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
			t.Errorf("panic not logged: %s", logs.String())
		}
	})
}

func TestAbortedRequestIsLogged(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := logRequests(log, recoverPanics(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic("late")
	})))
	func() {
		defer func() { // net/http's server turns this into a dropped connection
			if v := recover(); v != http.ErrAbortHandler {
				t.Errorf("recovered %v, want http.ErrAbortHandler", v)
			}
		}()
		do(h, "GET", "/post/abc")
	}()

	var infos []map[string]any
	for _, r := range logRecords(t, &logs) {
		if r["level"] == "INFO" {
			infos = append(infos, r)
		}
	}
	if len(infos) != 1 || infos[0]["method"] != "GET" || infos[0]["path"] != "/post/abc" {
		t.Errorf("request records = %v, want one for GET /post/abc", infos)
	}
}

func TestMiddlewareKeepsFlusher(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	h := logRequests(log, recoverPanics(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through the middleware: %v", err)
		}
	})))
	if rec := do(h, "GET", "/"); !rec.Flushed {
		t.Error("Flush did not reach the real ResponseWriter")
	}
}

func TestAssets(t *testing.T) {
	h, _ := newTestHandler(t, &fakeStore{}, &fakeFeed{})

	rec := do(h, "GET", "/assets/css/style.css")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") || rec.Body.Len() == 0 {
		t.Errorf("style.css: status %d, type %q, %d bytes", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}

	for _, target := range []string{"/assets/", "/assets/css/", "/assets/css", "/assets/fonts/", "/assets/css/owl.carousel.min.css"} {
		t.Run(target, func(t *testing.T) {
			rec := do(h, "GET", target)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "style.css") || strings.Contains(rec.Body.String(), "<a href=") {
				t.Errorf("body looks like a listing: %q", rec.Body.String())
			}
		})
	}
}

func TestRequestLog(t *testing.T) {
	h, logs := newTestHandler(t, &fakeStore{articles: numbered(25)}, &fakeFeed{})
	do(h, "GET", "/?page=2")

	recs := logRecords(t, logs)
	if len(recs) != 1 {
		t.Fatalf("%d log records, want 1: %s", len(recs), logs)
	}
	r := recs[0]
	if r["level"] != "INFO" || r["method"] != "GET" || r["path"] != "/" || r["status"] != float64(200) {
		t.Errorf("record = %v, want INFO GET / 200 without the query", r)
	}
	if _, ok := r["duration"]; !ok {
		t.Error("record has no duration")
	}

	logs.Reset()
	do(h, "GET", "/post/missing")
	if recs := logRecords(t, logs); len(recs) != 1 || recs[0]["status"] != float64(404) {
		t.Errorf("404 request logged as %v, want one record with status 404", recs)
	}
}

// blockingStore holds CountArticles until its context ends, like a MongoDB that stops answering.
type blockingStore struct {
	*fakeStore
	entered chan struct{}
}

func (s *blockingStore) CountArticles(ctx context.Context) (int, error) {
	close(s.entered)
	<-ctx.Done()
	return 0, ctx.Err()
}

// The request's own context reaches the store, so a disconnecting client cancels the query.
func TestClientDisconnectCancelsStore(t *testing.T) {
	store := &blockingStore{fakeStore: &fakeStore{}, entered: make(chan struct{})}
	h, _ := newTestHandler(t, store, &fakeFeed{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
		done <- rec
	}()
	<-store.entered // the handler is inside the store call
	cancel()        // the client disconnects

	select {
	case rec := <-done:
		wantStatusText(t, rec, http.StatusInternalServerError)
	case <-time.After(10 * time.Second):
		t.Fatal("the store call was not cancelled when the request context ended")
	}
}

// A 401 from upstream, which here even echoes the key, must reach neither the client nor the logs.
func TestUpstreamFailureHidesKey(t *testing.T) {
	const key = "k3y-marker"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"status":"error","results":{"message":"bad key %s","code":"Unauthorized"}}`, key)
	}))
	defer upstream.Close()

	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	c := feed.New(newsdata.New(upstream.URL, key, upstream.Client()), log, time.Now)
	h, err := NewHandler(log, &fakeStore{}, c, ui.Files)
	if err != nil {
		t.Fatal(err)
	}

	rec := do(h, "GET", "/category/business")
	wantStatusText(t, rec, http.StatusInternalServerError)
	if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), "status 401") {
		t.Errorf("failure not logged: %s", logs.String())
	}
	if strings.Contains(rec.Body.String(), key) || strings.Contains(logs.String(), key) {
		t.Errorf("key leaked: body %q, logs %s", rec.Body.String(), logs.String())
	}
}

func TestRealFeed(t *testing.T) {
	f := &countingFetcher{}
	var logs bytes.Buffer
	c := feed.New(f, slog.New(slog.NewJSONHandler(&logs, nil)), time.Now)
	h, _ := newTestHandler(t, &fakeStore{}, c)

	for _, target := range []string{"/category/BUSINESS", "/category/business"} {
		if rec := do(h, "GET", target); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "From upstream") {
			t.Fatalf("%s: status %d", target, rec.Code)
		}
	}
	if want := []string{"business"}; !slices.Equal(f.calls, want) {
		t.Errorf("fetches = %v, want %v", f.calls, want)
	}
	// One upstream call, one "category refreshed" record: the cached second request logs none.
	var refreshed []map[string]any
	for _, r := range logRecords(t, &logs) {
		if r["msg"] == "category refreshed" {
			refreshed = append(refreshed, r)
		}
	}
	if len(refreshed) != 1 || refreshed[0]["level"] != "INFO" || refreshed[0]["category"] != "business" || refreshed[0]["articles"] != float64(1) {
		t.Fatalf("refresh records = %v, want one INFO for business with 1 article", refreshed)
	}
	if _, ok := refreshed[0]["duration"]; !ok {
		t.Error("refresh record has no duration")
	}
	wantStatusText(t, do(h, "GET", "/category/politics"), http.StatusNotFound)
	if len(f.calls) != 1 {
		t.Errorf("fetches after an unknown category = %v, want still one", f.calls)
	}
}
