package main

import (
	"log"
	"net/http"
	"tengrinews/internal/domain"
	"tengrinews/internal/handler"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	repo := &domain.MockArticleRepository{}

	uc := usecase.NewArticleUsecase(repo)

	h := &handler.Handler{
		ArticleUseCase: *uc,
	}

	r.HandleFunc("/", h.IndexHandler)
	r.HandleFunc("/category/{category}", h.CategoryHandler)

	fs := http.FileServer(http.Dir("ui/assets/"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	err := http.ListenAndServe(":8000", r)
	if err != nil {
		log.Fatal(err)
	}
}
