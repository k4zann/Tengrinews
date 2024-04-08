package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"tengrinews/internal/api"
	"tengrinews/internal/models"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

var categoryCache = make(map[string]models.Result)

type Handlers struct {
	ArticleUsecase *usecase.ArticleUsecase
}

var Categories []string = []string{
	"Business", "Education", "Environment",
	"Food", "Health", "Lifestyle",
	"Science", "Sports", "Technology",
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		result := models.Result{}
		api.FetchData(&result)

		tmpl := template.Must(template.ParseFiles("ui/index.html")).Funcs(
			template.FuncMap{
				"lower": strings.ToLower,
			},
		)
		data := models.IndexPageData{
			Categories:  Categories,
			LatestPosts: result.Posts,
		}
		tmpl.Execute(w, data)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func CategoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	category := vars["category"]

	if cachedResult, ok := categoryCache[category]; ok {
		renderCategoryPage(w, category, cachedResult)
		return
	}

	apiURL := fmt.Sprintf("https://newsdata.io/api/1/news?apikey=pub_4152413e257988a1239359e23c019ac57c79e&category=%s", strings.ToLower(category))
	fmt.Println(apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Error fetching data", http.StatusInternalServerError)
		return
	}

	var result models.Result
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categoryCache[category] = result

	renderCategoryPage(w, category, result)
}

func renderCategoryPage(w http.ResponseWriter, category string, result models.Result) {
	tmpl := template.Must(template.ParseFiles("ui/category.html")).Funcs(
		template.FuncMap{
			"lower": strings.ToLower,
		},
	)

	data := models.CategoryPageData{
		Categories: Categories,
		Category:   category,
		Articles:   result.Posts,
	}

	tmpl.Execute(w, data)
}

func (h *Handlers) ArticleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	article, err := h.ArticleUsecase.GetArticleByID(id)
	if err != nil {
		http.Error(w, "Article not found", http.StatusNotFound)
		return
	}

	tmpl := template.Must(template.ParseFiles("ui/article.html"))
	tmpl.Execute(w, article)
}
