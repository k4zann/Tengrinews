package handler

import (
	"net/http"
	"tengrinews/internal/models"
	"tengrinews/internal/render"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

type Handler struct {
	ArticleUseCase usecase.ArticleUsecase
}

var categoryCache = make(map[string]models.Result)

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := models.Result{}
	articles, err := h.ArticleUseCase.GetAllArticles(&result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	render.RenderIndexPage(w, articles)
}

func (h *Handler) CategoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	category := vars["category"]

	if cachedResult, ok := categoryCache[category]; ok {
		render.RenderCategoryPage(w, category, cachedResult.Posts)
		return
	}

	result := models.Result{}
	articles, err := h.ArticleUseCase.GetArticlesByCategory(&result, category)
	if err != nil {
		if err.Error() == "not found" {
			http.Error(w, "Category not found", http.StatusNotFound)
		} else if err.Error() == "forbidden" {
			http.Error(w, "Access forbidden", http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	categoryCache[category] = result

	render.RenderCategoryPage(w, category, articles)
}

func (h *Handler) PostDetailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "ID not provided", http.StatusBadRequest)
		return
	}

	result := models.Article{}

	post, err := h.ArticleUseCase.GetArticleByID(&result, id)
	if err != nil {
		if err.Error() == "not found" {
			http.Error(w, "Post not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	render.RenderPostDetailsPage(w, *post)
}
