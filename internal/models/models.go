// models/article.go
package models

type Article struct {
	ID          string   `json:"article_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    []string `json:"category"`
	ImageURL    string   `json:"image_url"`
	Link        string   `json:"source_url"`
}

type IndexPageData struct {
	Categories  []string
	LatestPosts []Article // Corrected field name
	Articles    []Article
}

type CategoryPageData struct {
	Categories []string
	Category   string
	Articles   []Article
}

type Result struct {
	Posts []Article `json:"results"`
}
