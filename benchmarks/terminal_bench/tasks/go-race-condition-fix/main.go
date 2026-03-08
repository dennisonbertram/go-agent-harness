package main

import (
	"fmt"
	"net/http"
)

var globalCounter = NewCounter()

func main() {
	http.HandleFunc("/inc", func(w http.ResponseWriter, r *http.Request) {
		globalCounter.Increment()
		fmt.Fprintf(w, "incremented\n")
	})
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "count: %d\n", globalCounter.Value())
	})
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		globalCounter.Reset()
		fmt.Fprintf(w, "reset\n")
	})
	http.ListenAndServe(":8080", nil)
}
