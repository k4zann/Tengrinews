package handler

import (
	"context"
	"net/http"
	"strconv"
	"tengrinews/internal/helpers"
	"tengrinews/internal/models"
	"tengrinews/internal/render"
	"tengrinews/internal/usecase"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Handler struct {
	ArticleUseCase usecase.ArticleUsecase
}

var categoryCache = make(map[string]models.Result)

func (h *Handler) Pagination(page, pageSize int) ([]models.Article, error) {
	skip := (page - 1) * pageSize

	collection, err := h.ArticleUseCase.GetCollection()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Aggregate()
	opts.SetAllowDiskUse(true)

	cursor, err := collection.Aggregate(ctx, mongo.Pipeline{
		{{"$unwind", "$articles"}},
		{{"$project", bson.D{
			{"article_id", "$articles.article_id"},
			{"title", "$articles.title"},
			{"description", "$articles.description"},
			{"content", "$articles.content"},
			{"category", "$articles.category"},
			{"image_url", "$articles.image_url "},
			{"source_url", "$articles.link"},
		}}},
		{{"$skip", int64(skip)}},      // Skip records based on page number
		{{"$limit", int64(pageSize)}}, // Limit records per page
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var articles []models.Article
	for cursor.Next(ctx) {
		var article models.Article
		if err := cursor.Decode(&article); err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return articles, nil
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize := 10

	articles, err := h.Pagination(page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPages := 5
	pageData := models.IndexPageData{
		Categories:  helpers.Categories,
		LatestPosts: articles,
		CurrentPage: page,
		TotalPages:  totalPages,
		HasNextPage: page < totalPages,
		HasPrevPage: page > 1,
		NextPageNum: page + 1,
		PrevPageNum: page - 1,
	}

	render.RenderIndexPage(w, pageData)
}

func (h *Handler) CategoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/category/"+mux.Vars(r)["category"] {
		http.Error(w, "Not found", http.StatusNotFound)
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
