package render

import (
	"net/http"
	"tengrinews/internal/helpers"
	"tengrinews/internal/models"
	"text/template"
)

var templates map[string]*template.Template

func init() {
	templates = make(map[string]*template.Template)
	loadTemplates()
}

func loadTemplates() {
	templates["index.html"] = template.Must(template.New("index.html").Funcs(helpers.FuncsForTemplate).ParseFiles("ui/index.html"))
	templates["category.html"] = template.Must(template.New("category.html").Funcs(helpers.FuncsForTemplate).ParseFiles("ui/category.html"))
	templates["post_details.html"] = template.Must(template.New("post_details.html").Funcs(
		helpers.FuncsForSafeHTML,
	).ParseFiles("ui/post_details.html"))
}

func RenderCategoryPage(w http.ResponseWriter, category string, result []models.Article) {
	tmpl := templates["category.html"]
	data := models.CategoryPageData{
		Categories: helpers.Categories,
		Category:   category,
		Articles:   result,
	}
	tmpl.Execute(w, data)
}

func RenderIndexPage(w http.ResponseWriter, data models.IndexPageData) {
	tmpl := templates["index.html"]
	
	tmpl.Execute(w, data)
}

func RenderPostDetailsPage(w http.ResponseWriter, post models.Article) {
	tmpl := templates["post_details.html"]

	data := models.PostDetailesPageData{
		Categories: helpers.Categories,
		Post:       post,
	}
	tmpl.Execute(w, data)
}
