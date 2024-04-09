package helpers

import (
	"html/template"
	"strings"
)

const (
	APIKey         = "pub_4152413e257988a1239359e23c019ac57c79e"
	APIURL         = "https://newsdata.io/api/1/news"
	APILang        = "language=en,ru"
	APIFullContent = "full_content=1"
)

var Categories = []string{
	"Business", "Education", "Environment",
	"Food", "Health", "Lifestyle",
	"Science", "Sports", "Technology",
}

var FuncsForTemplate = template.FuncMap{
	"lower": strings.ToLower,
}
