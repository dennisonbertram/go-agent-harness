package main

import (
	"fmt"
	"net/http"
)

func main() {
	repo := NewUserRepo()
	handler := NewUserHandler(repo)

	http.HandleFunc("/get", handler.HandleGet)
	http.HandleFunc("/set", handler.HandleSet)
	http.HandleFunc("/count", handler.HandleCount)

	fmt.Println("listening on :8080")
	http.ListenAndServe(":8080", nil)
}
