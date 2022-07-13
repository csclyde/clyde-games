package main

import (
	"net/http"
)

func main() {
	http.HandleFunc("/api/ety/", ety);
	http.HandleFunc("/api/feedback", feedback)

	http.ListenAndServe(":9990", nil);
}
