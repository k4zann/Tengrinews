package web

import "tengrinews/internal/news"

// Benign fixtures for the golden test: no `<>&'"+` anywhere and plain https
// image URLs. The recordings in testdata/golden were made from these with the
// old text/template pages, which are gone; the recordings are never re-recorded.
//
// fixtureHomeArticles is page 2 of 3 on the home page (both links shown).
// fixtureCategoryArticles is what the feed returns: only Title, ImageURL and
// Category are set (design D2).
const (
	fixtureHomePage  = 2
	fixtureHomeTotal = 3
	fixtureCategory  = "business"
)

var (
	fixtureHomeArticles = []news.Article{
		{
			ID:          "abc123",
			Title:       "Central bank holds the key rate steady",
			Description: "The central bank left its key rate unchanged for the third month in a row.",
			ImageURL:    "https://example.com/img/abc123.jpg",
			Category:    []string{"business"},
		},
		{
			ID:          "def456",
			Title:       "Local team wins the national cup",
			Description: "A late goal decided the final in front of a full stadium.",
			ImageURL:    "https://example.com/img/def456.jpg",
			Category:    []string{"sports", "top"},
		},
		{
			ID:          "ghi789",
			Title:       "New science lab opens in Astana",
			Description: "The lab will study materials for solar panels.",
			ImageURL:    "https://example.com/img/ghi789.jpg",
			Category:    []string{"science"},
		},
	}

	fixtureCategoryArticles = []news.Article{
		{
			Title:    "Exports grow for the second quarter",
			ImageURL: "https://example.com/img/exports.jpg",
			Category: []string{"business"},
		},
		{
			Title:    "Startups raise record funding this year",
			ImageURL: "https://example.com/img/startups.jpg",
			Category: []string{"business", "technology"},
		},
	}

	fixturePost = news.Article{
		ID:          "abc123",
		Title:       "Central bank holds the key rate steady",
		Description: "The central bank left its key rate unchanged for the third month in a row.",
		Content:     "Analysts expected the decision. Inflation slowed again last month and the bank said it will keep watching prices before any change.",
		ImageURL:    "https://example.com/img/abc123.jpg",
		PubDate:     "2026-10-20 08:30:00",
		Category:    []string{"business"},
		Creator:     []string{"Aida Nurlan"},
	}
)
