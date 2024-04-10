// models/article.go
package models

type Article struct {
	ID          string   `json:"article_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Category    []string `json:"category"`
	ImageURL    string   `json:"image_url"`
	Link        string   `json:"source_url"`
}

type ArticleImages struct {
	Context string `json:"@context"`
	Type    string `json:"@type"`
	URL     string `json:"@url"`
}

type IndexPageData struct {
	Categories  []string
	LatestPosts []Article
	CurrentPage int  // Current page number
	TotalPages  int  // Total number of pages
	HasNextPage bool // Whether there is a next page
	HasPrevPage bool // Whether there is a previous page
	NextPageNum int  // Next page number
	PrevPageNum int  // Previous page number
}

type CategoryPageData struct {
	Categories []string
	Category   string
	Articles   []Article
}

type PostDetailesPageData struct {
	Categories []string
	Post       Article
}

type Result struct {
	Posts []Article `json:"results"`
}
