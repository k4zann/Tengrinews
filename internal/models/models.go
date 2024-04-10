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
	CurrentPage int
	TotalPages  int
	HasNextPage bool
	HasPrevPage bool
	NextPageNum int
	PrevPageNum int
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
