package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSetAndGet(t *testing.T) {
	repo := NewUserRepo()
	handler := NewUserHandler(repo)

	// Set a user
	req := httptest.NewRequest("GET", "/set?id=1&name=Alice", nil)
	w := httptest.NewRecorder()
	handler.HandleSet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Get the user
	req = httptest.NewRequest("GET", "/get?id=1", nil)
	w = httptest.NewRecorder()
	handler.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleGetNotFound(t *testing.T) {
	repo := NewUserRepo()
	handler := NewUserHandler(repo)

	req := httptest.NewRequest("GET", "/get?id=999", nil)
	w := httptest.NewRecorder()
	handler.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleCount(t *testing.T) {
	repo := NewUserRepo()
	handler := NewUserHandler(repo)
	repo.SetUser("1", "Alice")
	repo.SetUser("2", "Bob")

	req := httptest.NewRequest("GET", "/count", nil)
	w := httptest.NewRecorder()
	handler.HandleCount(w, req)

	if w.Body.String() != "count: 2\n" {
		t.Fatalf("expected 'count: 2', got %q", w.Body.String())
	}
}

func TestMiddlewareInjectsRepo(t *testing.T) {
	repo := NewUserRepo()
	repo.SetUser("1", "Test")

	handler := WithUserRepo(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := GetUserRepoFromContext(r.Context())
		if got == nil {
			t.Fatal("repo not found in context")
		}
		if _, ok := got.GetUser("1"); !ok {
			t.Fatal("expected user '1' in repo")
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}
