package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"tengrinews/internal/news"
)

// ArticleStore reads the stored articles.
type ArticleStore interface {
	CountArticles(ctx context.Context) (int, error)
	ListArticles(ctx context.Context, skip, limit int) ([]news.Article, error)
	ArticleByID(ctx context.Context, id string) (news.Article, error) // news.ErrNotFound when absent
}

// CategoryFeed provides the current articles of a category.
type CategoryFeed interface {
	Articles(ctx context.Context, slug string) ([]news.Article, error) // news.ErrNotFound for an unknown slug
}

const pageSize = 10

type handler struct {
	log        *slog.Logger
	store      ArticleStore
	feed       CategoryFeed
	pages      pages
	categories []news.Category
}

// NewHandler parses the page templates from files and returns the site: the three pages,
// the static assets under /assets/, and the request log and panic recovery around them.
// files holds templates/ and assets/ (ui.Files).
func NewHandler(log *slog.Logger, store ArticleStore, feed CategoryFeed, files fs.FS) (http.Handler, error) {
	p, err := parsePages(files)
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(files, "assets")
	if err != nil {
		return nil, fmt.Errorf("open assets: %w", err)
	}
	h := &handler{log: log, store: store, feed: feed, pages: p, categories: news.Categories()}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /category/{category}", h.category)
	mux.HandleFunc("GET /post/{id}", h.post)
	mux.Handle("GET /assets/", http.StripPrefix("/assets", http.FileServerFS(noDirFS{assets})))
	return logRequests(log, recoverPanics(log, mux)), nil
}

func (h *handler) home(w http.ResponseWriter, r *http.Request) {
	total, err := h.store.CountArticles(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := paginate(r.URL.Query().Get("page"), total)
	articles, err := h.store.ListArticles(r.Context(), (p.current-1)*pageSize, pageSize)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, h.pages.home, homeView{
		Categories:  h.categories,
		LatestPosts: articles,
		HasPrevPage: p.hasPrev,
		PrevPageNum: p.current - 1,
		HasNextPage: p.hasNext,
		NextPageNum: p.current + 1,
	})
}

func (h *handler) category(w http.ResponseWriter, r *http.Request) {
	c, ok := news.ParseCategory(r.PathValue("category"))
	if !ok {
		notFound(w)
		return
	}
	articles, err := h.feed.Articles(r.Context(), c.Slug)
	if errors.Is(err, news.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, h.pages.category, categoryView{Categories: h.categories, Category: c.Slug, Articles: articles})
}

func (h *handler) post(w http.ResponseWriter, r *http.Request) {
	a, err := h.store.ArticleByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, news.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, h.pages.post, postView{Categories: h.categories, Post: a})
}

func (h *handler) render(w http.ResponseWriter, r *http.Request, t *template.Template, data any) {
	if err := render(w, t, data); err != nil {
		h.fail(w, r, err)
	}
}

func notFound(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

// fail logs err and answers 500 with the status text only: clients never see internal error text.
func (h *handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

type pagination struct {
	current          int
	hasPrev, hasNext bool
}

// paginate resolves the ?page value against total stored articles. A value that is not an
// integer, or is outside 1..number of pages, falls back to page 1.
func paginate(raw string, total int) pagination {
	totalPages := (total + pageSize - 1) / pageSize
	current, err := strconv.Atoi(raw)
	if err != nil || current < 1 || current > totalPages {
		current = 1
	}
	return pagination{current: current, hasPrev: current > 1, hasNext: current < totalPages}
}

// noDirFS hides directories, so /assets/ and /assets/css/ are 404 instead of a file listing.
type noDirFS struct{ fs.FS }

func (n noDirFS) Open(name string) (fs.File, error) {
	f, err := n.FS.Open(name)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err == nil && fi.IsDir() {
		err = fs.ErrNotExist
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
