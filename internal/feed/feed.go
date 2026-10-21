// Package feed caches newsdata.io category feeds. A copy is fresh for TTL; when a
// refresh fails the last good copy is served, whatever its age.
package feed

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"tengrinews/internal/news"
)

// TTL is how long a successful refresh is served without contacting upstream.
const TTL = time.Hour

// Fetcher gets the latest articles of a category from upstream.
type Fetcher interface {
	Latest(ctx context.Context, category string) ([]news.Article, error)
}

type entry struct {
	articles  []news.Article
	fetchedAt time.Time
}

// Cache is safe for concurrent use. It holds at most one entry per known category.
// The returned slices are shared: callers must not modify them.
type Cache struct {
	fetcher Fetcher
	log     *slog.Logger
	now     func() time.Time

	mu      sync.Mutex
	entries map[string]entry
	flights singleflight.Group
}

// New returns an empty Cache. now is injected so tests can move the clock.
func New(f Fetcher, log *slog.Logger, now func() time.Time) *Cache {
	return &Cache{fetcher: f, log: log, now: now, entries: make(map[string]entry)}
}

// Articles returns the articles of a category. Any letter case of a known slug is
// accepted; everything else is news.ErrNotFound, with no fetch and no cache entry.
func (c *Cache) Articles(ctx context.Context, category string) ([]news.Article, error) {
	cat, ok := news.ParseCategory(category)
	if !ok {
		return nil, news.ErrNotFound
	}
	slug := cat.Slug

	if e, ok := c.lookup(slug); ok && c.fresh(e) {
		return e.articles, nil
	}
	v, err, _ := c.flights.Do(slug, func() (any, error) { return c.refresh(ctx, slug) })
	if err != nil {
		return nil, err
	}
	return v.([]news.Article), nil
}

func (c *Cache) lookup(slug string) (entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[slug]
	return e, ok
}

func (c *Cache) fresh(e entry) bool { return c.now().Sub(e.fetchedAt) < TTL }

// refresh runs once per category at a time. It ignores the cancellation of the
// caller that started it, so one client disconnecting does not fail the clients
// waiting on the same refresh; the fetcher's own timeout still bounds it.
func (c *Cache) refresh(ctx context.Context, slug string) ([]news.Article, error) {
	// A refresh that finished while this caller was on its way in already did the work.
	old, hadOld := c.lookup(slug)
	if hadOld && c.fresh(old) {
		return old.articles, nil
	}

	start := c.now()
	articles, err := c.fetcher.Latest(context.WithoutCancel(ctx), slug)
	if err != nil {
		if !hadOld {
			return nil, err
		}
		c.log.Warn("category refresh failed, serving stale copy", "category", slug, "error", err)
		return old.articles, nil
	}

	done := c.now()
	c.mu.Lock()
	c.entries[slug] = entry{articles: articles, fetchedAt: done}
	c.mu.Unlock()
	// One record per upstream call, so the owner can count them against the newsdata.io quota.
	c.log.Info("category refreshed", "category", slug, "articles", len(articles), "duration", done.Sub(start))
	return articles, nil
}
