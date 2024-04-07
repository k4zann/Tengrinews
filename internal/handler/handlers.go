package handler

import (
	"net/http"
	"strconv"
	"text/template"

	"tengrinews/internal/api"
	"tengrinews/internal/models"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

type Handlers struct {
	ArticleUsecase *usecase.ArticleUsecase
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		Categories := []string{"Lifestyle", "Business", "Fashion", "Design", "Health", "Harmful", "Technology", "Travel", "Food", "Creative"}

		result := models.Result{}
		api.FetchData(&result)

		tmpl := template.Must(template.ParseFiles("ui/index.html"))
		data := models.PageData{
			Categories:  Categories,
			Title:       "Magazine News",
			LatestPosts: result.Posts,
		}
		tmpl.Execute(w, data)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
