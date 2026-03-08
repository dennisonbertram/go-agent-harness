package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// UserHandler provides HTTP endpoints backed by a UserRepo.
type UserHandler struct {
	repo *UserRepo
}

// NewUserHandler creates a handler with the given UserRepo.
func NewUserHandler(repo *UserRepo) *UserHandler {
	return &UserHandler{repo: repo}
}

// HandleGet returns user info by ID.
func (h *UserHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	name, ok := h.repo.GetUser(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": name})
}

// HandleSet creates or updates a user.
func (h *UserHandler) HandleSet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	name := r.URL.Query().Get("name")
	h.repo.SetUser(id, name)
	fmt.Fprintf(w, "ok\n")
}

// HandleCount returns the total user count.
func (h *UserHandler) HandleCount(w http.ResponseWriter, r *http.Request) {
	count := h.repo.CountUsers()
	fmt.Fprintf(w, "count: %d\n", count)
}
