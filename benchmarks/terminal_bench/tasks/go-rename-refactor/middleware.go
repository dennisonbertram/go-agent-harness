package main

import (
	"context"
	"net/http"
)

type contextKey string

const userRepoKey contextKey = "userRepo"

// WithUserRepo injects a UserRepo into the request context.
func WithUserRepo(repo *UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), userRepoKey, repo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserRepoFromContext extracts the UserRepo from context.
func GetUserRepoFromContext(ctx context.Context) *UserRepo {
	repo, _ := ctx.Value(userRepoKey).(*UserRepo)
	return repo
}
