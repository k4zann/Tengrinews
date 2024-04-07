package models

type Article struct {
	ID      int
	Title   string
	Content string
}

type Post struct {
	ID          string   `json:"article_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    []string `json:"category"`
	ImageURL    string   `json:"image_url"`
	Link        string   `json:"source_url"`
}

type PageData struct {
	Categories  []string
	Title       string
	LatestPosts []Post
	Articles    []Post
}

type Result struct {
	Posts []Post `json:"results"`
}
