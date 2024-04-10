package main

import (
	"log"
	"net/http"
	"tengrinews/internal/domain"
	"tengrinews/internal/handler"
	"tengrinews/internal/helpers"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	// helpers.LoadEnv()
	client, collection, err := helpers.InitMongoDB()
	if err != nil {
		log.Fatal("Error initializing MongoDB:", err)
	}

	repo := &domain.MockArticleRepository{}
	uc := usecase.NewArticleUsecase(repo, client, collection)

	h := &handler.Handler{
		ArticleUseCase: *uc,
	}

	r.HandleFunc("/", h.IndexHandler)
	r.HandleFunc("/category/{category}", h.CategoryHandler)
	r.HandleFunc("/post/{id}", h.PostDetailsHandler)
	fs := http.FileServer(http.Dir("ui/assets/"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	err = http.ListenAndServe(helpers.PORT, r)
	if err != nil {
		log.Fatal(err)
	}
}
