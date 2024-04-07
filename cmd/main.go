package main

import (
	"net/http"

	"tengrinews/internal/handler"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", handler.IndexHandler)

	fs := http.FileServer(http.Dir("ui/assets/"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	http.Handle("/", r)

	http.ListenAndServe(":8080", nil)
}
