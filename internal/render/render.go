// handler/render.go
package render

import (
	"net/http"
	"tengrinews/internal/helpers"
	"tengrinews/internal/models"
	"text/template"
)

func RenderCategoryPage(w http.ResponseWriter, category string, result []models.Article) {
	tmpl := template.Must(template.New("category.html").Funcs(helpers.FuncsForTemplate).ParseFiles("ui/category.html"))
	data := models.CategoryPageData{
		Categories: helpers.Categories,
		Category:   category,
		Articles:   result,
	}

	tmpl.Execute(w, data)
}

func RenderIndexPage(w http.ResponseWriter, latestPosts []models.Article) {
	tmpl := template.Must(template.New("index.html").Funcs(helpers.FuncsForTemplate).ParseFiles("ui/index.html"))
	data := models.IndexPageData{
		Categories:  helpers.Categories,
		LatestPosts: latestPosts,
	}
	tmpl.Execute(w, data)
}

func RenderPostDetailsPage(w http.ResponseWriter, post models.Article) {
	tmpl := template.Must(template.New("post_details.html").Funcs(helpers.FuncsForTemplate).ParseFiles("ui/post_details.html"))

	data := models.PostDetailesPageData{
		Categories: helpers.Categories,
		Post:       post,
	}
	tmpl.Execute(w, data)
}
