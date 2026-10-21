// Package web serves the Tengrinews pages.
package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"tengrinews/internal/news"
)

// homeView, categoryView and postView keep the field names the page templates use.
type homeView struct {
	Categories  []news.Category
	LatestPosts []news.Article
	HasPrevPage bool
	PrevPageNum int
	HasNextPage bool
	NextPageNum int
}

type categoryView struct {
	Categories []news.Category
	Category   string // canonical slug
	Articles   []news.Article
}

type postView struct {
	Categories []news.Category
	Post       news.Article
}

// pages holds one parsed template set per page: the shared layout and partials plus that page's "main".
type pages struct {
	home, category, post *template.Template
}

func parsePages(files fs.FS) (pages, error) {
	var (
		p   pages
		err error
	)
	if p.home, err = parsePage(files, "home"); err != nil {
		return pages{}, err
	}
	if p.category, err = parsePage(files, "category"); err != nil {
		return pages{}, err
	}
	if p.post, err = parsePage(files, "post"); err != nil {
		return pages{}, err
	}
	return p, nil
}

func parsePage(files fs.FS, name string) (*template.Template, error) {
	t, err := template.ParseFS(files, "templates/layout.html", "templates/partials/*.html", "templates/pages/"+name+".html")
	if err != nil {
		return nil, fmt.Errorf("parse %s page: %w", name, err)
	}
	return t, nil
}

// render executes the "layout" template into a buffer and only then writes the 200 response,
// so a failing template leaves w untouched. The caller answers 500 on error.
func render(w http.ResponseWriter, t *template.Template, data any) error {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return fmt.Errorf("execute layout: %w", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w) // a write error means the client went away; nothing left to do
	return nil
}
