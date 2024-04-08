package main

import (
	"log"
	"net/http"
	"tengrinews/internal/handler"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", handler.IndexHandler)
	r.HandleFunc("/category/{category}", handler.CategoryHandler)

	fs := http.FileServer(http.Dir("ui/assets/"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	err := http.ListenAndServe(":8000", r)
	if err != nil {
		log.Fatal(err)
	}
}
