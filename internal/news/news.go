// Package news holds the domain types shared by the store, the feed and the web layer.
package news

import (
	"errors"
	"slices"
	"strings"
)

// ErrNotFound reports that a requested article or category does not exist.
var ErrNotFound = errors.New("not found")

// Article holds only the fields the pages render.
type Article struct {
	ID          string
	Title       string
	Description string
	Content     string
	ImageURL    string
	PubDate     string
	Category    []string
	Creator     []string
}

// Category is a newsdata.io category: Slug is the lowercase API value, Name the display label.
type Category struct {
	Slug string
	Name string
}

var categories = []Category{
	{"business", "Business"},
	{"education", "Education"},
	{"environment", "Environment"},
	{"food", "Food"},
	{"health", "Health"},
	{"lifestyle", "Lifestyle"},
	{"science", "Science"},
	{"sports", "Sports"},
	{"technology", "Technology"},
}

// Categories returns the nine known categories in display order. The caller owns the slice.
func Categories() []Category {
	return slices.Clone(categories)
}

// ParseCategory lowercases raw and matches it exactly against the known slugs.
// It does not use strings.EqualFold: Unicode folding would accept spellings
// such as "ſports" (long s) as "sports".
func ParseCategory(raw string) (Category, bool) {
	lower := strings.ToLower(raw)
	for _, c := range categories {
		if c.Slug == lower {
			return c, true
		}
	}
	return Category{}, false
}
