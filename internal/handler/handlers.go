// handler/handler.go
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
	if r.Method == http.MethodGet {
		result := models.Result{}
		articles, err := h.ArticleUseCase.GetAllArticles(&result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		render.RenderIndexPage(w, articles)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) CategoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	category := vars["category"]

	if cachedResult, ok := categoryCache[category]; ok {
		render.RenderCategoryPage(w, category, cachedResult.Posts) // Pass cachedResult.Posts
		return
	}

	result := models.Result{} // Initialize a new result
	articles, err := h.ArticleUseCase.GetArticlesByCategory(&result, category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categoryCache[category] = result

	render.RenderCategoryPage(w, category, articles) // Pass articles
}
