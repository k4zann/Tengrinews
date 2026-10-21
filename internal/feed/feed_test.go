package feed

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"tengrinews/internal/news"
)

var errUpstream = errors.New("upstream 500")

type result struct {
	articles []news.Article
	err      error
}

// fakeFetcher answers from a queue, one result per call, and records what it was asked.
type fakeFetcher struct {
	mu      sync.Mutex
	calls   []string
	queue   []result
	release <-chan struct{} // when set, Latest blocks on it before answering
	ctxErr  error           // ctx.Err() seen after release
	onFetch func()          // when set, runs inside every Latest call (e.g. to move the fake clock)
}

func (f *fakeFetcher) Latest(ctx context.Context, category string) ([]news.Article, error) {
	f.mu.Lock()
	f.calls = append(f.calls, category)
	r := result{err: errors.New("unexpected fetch")}
	if len(f.queue) > 0 {
		r, f.queue = f.queue[0], f.queue[1:]
	}
	release, onFetch := f.release, f.onFetch
	f.mu.Unlock()

	if onFetch != nil {
		onFetch()
	}
	if release != nil {
		<-release
		f.mu.Lock()
		f.ctxErr = ctx.Err()
		f.mu.Unlock()
	}
	return r.articles, r.err
}

func (f *fakeFetcher) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	cache   *Cache
	fetcher *fakeFetcher
	clock   *fakeClock
	logs    *bytes.Buffer
}

func newEnv(queue ...result) *env {
	e := &env{
		fetcher: &fakeFetcher{queue: queue},
		clock:   &fakeClock{t: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)},
		logs:    &bytes.Buffer{},
	}
	e.cache = New(e.fetcher, slog.New(slog.NewTextHandler(e.logs, nil)), e.clock.Now)
	return e
}

func arts(title string) []news.Article { return []news.Article{{Title: title}} }

func mustArticles(t *testing.T, e *env, category string) []news.Article {
	t.Helper()
	got, err := e.cache.Articles(context.Background(), category)
	if err != nil {
		t.Fatalf("Articles(%q): %v", category, err)
	}
	return got
}

func TestFreshAndExpiredCopy(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		wantTitle string
		wantCalls int
	}{
		{"fresh at 59m", 59 * time.Minute, "old", 1},
		{"expired at 61m", 61 * time.Minute, "new", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(result{articles: arts("old")}, result{articles: arts("new")})
			mustArticles(t, e, "business")
			e.clock.Advance(tt.age)

			got := mustArticles(t, e, "business")
			if !reflect.DeepEqual(got, arts(tt.wantTitle)) {
				t.Errorf("got %v, want %q", got, tt.wantTitle)
			}
			if n := len(e.fetcher.called()); n != tt.wantCalls {
				t.Errorf("fetches = %d, want %d", n, tt.wantCalls)
			}

			// Whatever was served is now cached.
			mustArticles(t, e, "business")
			if n := len(e.fetcher.called()); n != tt.wantCalls {
				t.Errorf("fetches after repeat = %d, want %d", n, tt.wantCalls)
			}
		})
	}
}

// refresh re-checks the cache itself: a caller that saw an expired entry just as the previous
// flight finished must get that flight's fresh entry, not trigger a second upstream call.
func TestRefreshRechecksCache(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		wantTitle string
		wantCalls int
	}{
		{"fresh entry, no fetch", 59 * time.Minute, "kept", 0},
		{"expired entry, one fetch", 61 * time.Minute, "fetched", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(result{articles: arts("fetched")})
			e.cache.entries["business"] = entry{articles: arts("kept"), fetchedAt: e.clock.Now().Add(-tt.age)}

			got, err := e.cache.refresh(context.Background(), "business")
			if err != nil || !reflect.DeepEqual(got, arts(tt.wantTitle)) {
				t.Errorf("refresh = %v, %v; want %q", got, err, tt.wantTitle)
			}
			if n := len(e.fetcher.called()); n != tt.wantCalls {
				t.Errorf("fetches = %d, want %d", n, tt.wantCalls)
			}
			if n := strings.Count(e.logs.String(), `msg="category refreshed"`); n != tt.wantCalls {
				t.Errorf("refreshed records = %d, want %d", n, tt.wantCalls)
			}
		})
	}
}

func TestRefreshIsLoggedOncePerUpstreamCall(t *testing.T) {
	const msg = `msg="category refreshed"`
	e := newEnv(result{articles: arts("old")}, result{err: errUpstream})
	e.fetcher.onFetch = func() { e.clock.Advance(3 * time.Second) } // the duration comes from the injected clock

	mustArticles(t, e, "business") // cold: one fetch, one record
	for _, want := range []string{"level=INFO", msg, "category=business", "articles=1", "duration=3s"} {
		if !strings.Contains(e.logs.String(), want) {
			t.Errorf("log %q lacks %q", e.logs.String(), want)
		}
	}
	if n := strings.Count(e.logs.String(), msg); n != 1 {
		t.Errorf("records after a cold request = %d, want 1", n)
	}

	mustArticles(t, e, "Business") // cached hit: no fetch, no record
	if n := strings.Count(e.logs.String(), msg); n != 1 {
		t.Errorf("records after a cached hit = %d, want still 1", n)
	}

	e.clock.Advance(2 * time.Hour)
	mustArticles(t, e, "business") // refresh fails, stale copy served: a Warn, not a refreshed record
	if n := strings.Count(e.logs.String(), msg); n != 1 {
		t.Errorf("records after a failed refresh = %d, want still 1", n)
	}
}

func TestStaleCopyOnRefreshError(t *testing.T) {
	e := newEnv(result{articles: arts("old")}, result{err: errUpstream})
	mustArticles(t, e, "food")
	e.clock.Advance(3 * time.Hour)

	got := mustArticles(t, e, "food")
	if !reflect.DeepEqual(got, arts("old")) {
		t.Errorf("got %v, want the stale copy", got)
	}
	for _, want := range []string{"level=WARN", "category=food", "upstream 500"} {
		if !strings.Contains(e.logs.String(), want) {
			t.Errorf("log %q lacks %q", e.logs.String(), want)
		}
	}
}

func TestNoCopyAndRefreshError(t *testing.T) {
	e := newEnv(result{err: errUpstream}, result{articles: arts("later")})

	_, err := e.cache.Articles(context.Background(), "food")
	if !errors.Is(err, errUpstream) {
		t.Fatalf("err = %v, want the upstream error", err)
	}
	if len(e.cache.entries) != 0 {
		t.Errorf("a failed first fetch left %d entries", len(e.cache.entries))
	}

	// No negative caching: the next request tries again.
	if got := mustArticles(t, e, "food"); !reflect.DeepEqual(got, arts("later")) {
		t.Errorf("got %v after recovery", got)
	}
}

func TestRecoveryAfterStale(t *testing.T) {
	e := newEnv(result{articles: arts("old")}, result{err: errUpstream}, result{articles: arts("new")})
	mustArticles(t, e, "food")
	e.clock.Advance(3 * time.Hour)
	mustArticles(t, e, "food") // stale copy, refresh failed

	got := mustArticles(t, e, "food")
	if !reflect.DeepEqual(got, arts("new")) {
		t.Errorf("got %v, want the refreshed articles", got)
	}
	if n := len(e.fetcher.called()); n != 3 {
		t.Errorf("fetches = %d, want 3", n)
	}
}

func TestLetterCaseSharesOneEntry(t *testing.T) {
	e := newEnv(result{articles: arts("a")}, result{articles: arts("b")})
	mustArticles(t, e, "sports")
	if got := mustArticles(t, e, "Sports"); !reflect.DeepEqual(got, arts("a")) {
		t.Errorf("Sports after sports got %v, want the cached copy", got)
	}

	e.clock.Advance(2 * time.Hour)
	mustArticles(t, e, "SPORTS") // the refresh sends the canonical slug

	if got, want := e.fetcher.called(), []string{"sports", "sports"}; !reflect.DeepEqual(got, want) {
		t.Errorf("fetcher received %v, want %v", got, want)
	}
	if len(e.cache.entries) != 1 {
		t.Errorf("entries = %d, want 1", len(e.cache.entries))
	}
}

func TestUnknownCategory(t *testing.T) {
	for _, raw := range []string{"politics", "../x", "ſports", "", "business ", "<script>"} {
		t.Run(raw, func(t *testing.T) {
			e := newEnv(result{articles: arts("never")})

			_, err := e.cache.Articles(context.Background(), raw)
			if !errors.Is(err, news.ErrNotFound) {
				t.Errorf("err = %v, want news.ErrNotFound", err)
			}
			if n := len(e.fetcher.called()); n != 0 {
				t.Errorf("fetches = %d, want 0", n)
			}
			if len(e.cache.entries) != 0 {
				t.Errorf("entries = %d, want 0", len(e.cache.entries))
			}
		})
	}
}

// The synctest bubble lets the test wait until every goroutine is parked, so the
// requests really overlap with the one in-flight fetch without any sleeping.
func TestConcurrentColdRequestsShareOneFetch(t *testing.T) {
	const n = 50
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		e := newEnv(result{articles: arts("science")})
		e.fetcher.release = release

		got := make([][]news.Article, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() { got[i], errs[i] = e.cache.Articles(context.Background(), "science") })
		}
		synctest.Wait() // one goroutine is in the fetch, the others wait for it
		close(release)
		wg.Wait()

		if calls := e.fetcher.called(); len(calls) != 1 {
			t.Errorf("fetches = %v, want exactly one", calls)
		}
		for i := range n {
			if errs[i] != nil || !reflect.DeepEqual(got[i], arts("science")) {
				t.Errorf("request %d: got %v, %v", i, got[i], errs[i])
			}
		}
	})
}

func TestSharedRefreshSurvivesCallerCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		e := newEnv(result{articles: arts("shared")})
		e.fetcher.release = release

		ctxA, cancelA := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = e.cache.Articles(ctxA, "health") })
		synctest.Wait() // A is inside the fetch

		var gotB []news.Article
		var errB error
		wg.Go(func() { gotB, errB = e.cache.Articles(context.Background(), "health") })
		synctest.Wait() // B waits on A's refresh

		cancelA() // A's client disconnects mid-refresh
		close(release)
		wg.Wait()

		if errB != nil || !reflect.DeepEqual(gotB, arts("shared")) {
			t.Errorf("waiting request got %v, %v", gotB, errB)
		}
		if calls := e.fetcher.called(); len(calls) != 1 {
			t.Errorf("fetches = %v, want exactly one", calls)
		}
		e.fetcher.mu.Lock()
		defer e.fetcher.mu.Unlock()
		if e.fetcher.ctxErr != nil {
			t.Errorf("fetch saw ctx error %v, want the caller's cancellation not to reach it", e.fetcher.ctxErr)
		}
	})
}
