package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("api/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World - test")
	})

	http.HandleFunc("api/greet/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/greet/"):]
		fmt.Fprintf(w, "Hello Mr. %s\n", name)
	})

	http.ListenAndServe(":9990", nil)
}
