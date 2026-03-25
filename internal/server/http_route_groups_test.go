package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func passthroughMiddleware(next http.Handler) http.Handler {
	return next
}

func TestRegisterRunRoutes(t *testing.T) {
	t.Parallel()

	s := Server{runner: testRunnerForModels(t)}
	mux := http.NewServeMux()

	s.registerRunRoutes(mux, passthroughMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("GET /v1/runs: expected 501 from run route group, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/runs/missing", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/runs/missing: expected 404 from run route group, got %d", rec.Code)
	}
}

func TestRegisterConversationRoutes(t *testing.T) {
	t.Parallel()

	s := Server{runner: testRunnerForModels(t)}
	mux := http.NewServeMux()

	s.registerConversationRoutes(mux, passthroughMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/v1/conversations/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("GET /v1/conversations/: expected 501 from conversation route group, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/conversations/search?q=test", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("GET /v1/conversations/search: expected 501 from conversation route group, got %d", rec.Code)
	}
}

func TestRegisterCatalogRoutes(t *testing.T) {
	t.Parallel()

	s := Server{
		runner:  testRunnerForModels(t),
		catalog: testCatalog(),
	}
	mux := http.NewServeMux()

	s.registerCatalogRoutes(
		mux,
		passthroughMiddleware,
		passthroughMiddleware,
		passthroughMiddleware,
		passthroughMiddleware,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/models: expected 200 from catalog route group, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/providers: expected 200 from catalog route group, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/summarize", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/summarize: expected 405 from catalog route group, got %d", rec.Code)
	}
}
