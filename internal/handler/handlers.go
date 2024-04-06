package handler

import (
	"net/http"
	"strconv"
	"text/template"

	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

type Handlers struct {
	ArticleUsecase *usecase.ArticleUsecase
}

func (h *Handlers) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		articles, err := h.ArticleUsecase.GetAllArticles()
		if err != nil {
			http.Error(w, "Error fetching articles", http.StatusInternalServerError)
			return
		}

		tmpl := template.Must(template.ParseFiles("ui/index.html"))
		tmpl.Execute(w, articles)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) ArticleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
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
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
