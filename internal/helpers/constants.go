package helpers

import (
	"html/template"
	"os"
	"strings"
)

const (
	APIURL  = "https://newsdata.io/api/1/news"
	APILang = "language=en,ru"
	PORT    = ":8080"
)

var (
	APIKey     = os.Getenv("NEWSDATA_API_KEY")
	MongoDBUri = os.Getenv("MONGODB_URI")
)

var Categories = []string{
	"Business", "Education", "Environment",
	"Food", "Health", "Lifestyle",
	"Science", "Sports", "Technology",
}

var FuncsForTemplate = template.FuncMap{
	"lower": strings.ToLower,
}

var FuncsForSafeHTML = template.FuncMap{
	"safeHTML": func(content string) template.HTML {
		return template.HTML(content)
	},
	"lower": strings.ToLower,
}
