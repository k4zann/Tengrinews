package main

import (
	"log"
	"net/http"
	"tengrinews/internal/domain"
	"tengrinews/internal/handler"
	"tengrinews/internal/models"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	articleRepo := &domain.MockArticleRepository{
		Articles: []models.Article{
			{ID: 1, Title: "Breaking News 1", Content: "This is the content of Breaking News 1."},
			{ID: 2, Title: "Important Update", Content: "An important update is here."},
		},
	}

	articleUsecase := usecase.NewArticleUsecase(articleRepo)
	handlers := &handler.Handlers{ArticleUsecase: articleUsecase}

	r.HandleFunc("/", handlers.IndexHandler)
	r.HandleFunc("/article/", handlers.ArticleHandler)
	fs := http.FileServer(http.Dir("ui/static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
