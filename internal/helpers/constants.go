package helpers

import (
	"html/template"
	"strings"
)

const (
	APIKey     = "pub_4152413e257988a1239359e23c019ac57c79e"
	APIURL     = "https://newsdata.io/api/1/news"
	APILang    = "language=en,ru"
	MongoDBUri = "mongodb+srv://arshataitkozha:010arshat@tengrinews.t5rzs40.mongodb.net/?retryWrites=true&w=majority&appName=Tengrinews"
	PORT       = ":8080"
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
