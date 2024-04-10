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

type Post struct {
	ID          string   `bson:"article_id"`
	Title       string   `bson:"title"`
	Description string   `bson:"description"`
	Content     string   `bson:"content"`
	Category    []string `bson:"category"`
	ImageURL    string   `bson:"image_url"`
	Link        string   `bson:"source_url"`
	PubDate     string   `bson:"pubDate"`
	Creator     []string `bson:"creator"`
}

type ArticleImages struct {
	Context string `json:"@context"`
	Type    string `json:"@type"`
	URL     string `json:"@url"`
}

type IndexPageData struct {
	Categories  []string
	LatestPosts []Post
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
	Post       Post
}

type Result struct {
	Posts []Article `json:"results"`
}

type ResultMongo struct {
	Posts []Post `bson:"articles"`
}
