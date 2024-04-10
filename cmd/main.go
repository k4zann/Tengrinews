package main

import (
	"context"
	"log"
	"net/http"
	"tengrinews/internal/domain"
	"tengrinews/internal/handler"
	"tengrinews/internal/usecase"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client *mongo.Client
)

func main() {
	r := mux.NewRouter()

	initMongoDB()

	repo := &domain.MockArticleRepository{}

	uc := usecase.NewArticleUsecase(repo, client)

	h := &handler.Handler{
		ArticleUseCase: *uc,
	}

	r.HandleFunc("/", h.IndexHandler)
	r.HandleFunc("/category/{category}", h.CategoryHandler)
	r.HandleFunc("/post/{id}", h.PostDetailsHandler)
	fs := http.FileServer(http.Dir("ui/assets/"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		log.Fatal(err)
	}
}

func initMongoDB() {
	clientOptions := options.Client().ApplyURI("mongodb+srv://arshataitkozha:010arshat@tengrinews.t5rzs40.mongodb.net/?retryWrites=true&w=majority&appName=Tengrinews")
	var err error
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}

	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("Error pinging MongoDB:", err)
	}

	log.Println("Connected to MongoDB successfully")
}
