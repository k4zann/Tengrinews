// handler/render.go
package render

import (
	"net/http"
	"strings"
	"tengrinews/internal/helpers"
	"tengrinews/internal/models"
	"text/template"
)

func RenderCategoryPage(w http.ResponseWriter, category string, result []models.Article) {
	tmpl := template.Must(template.ParseFiles("ui/category.html")).Funcs(
		template.FuncMap{
			"lower": strings.ToLower,
		},
	)

	data := models.CategoryPageData{
		Categories: helpers.Categories,
		Category:   category,
		Articles:   result,
	}

	tmpl.Execute(w, data)
}

func RenderIndexPage(w http.ResponseWriter, latestPosts []models.Article) {
	tmpl := template.Must(template.ParseFiles("ui/index.html")).Funcs(
		template.FuncMap{
			"lower": strings.ToLower,
		},
	)
	data := models.IndexPageData{
		Categories:  helpers.Categories,
		LatestPosts: latestPosts,
	}
	tmpl.Execute(w, data)
}
