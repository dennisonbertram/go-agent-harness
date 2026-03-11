package symphd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleState(t *testing.T) {
	o := NewOrchestrator(DefaultConfig())
	h := NewHandler(o)

	req := httptest.NewRequest("GET", "/api/v1/state", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v", resp["status"])
	}
	if resp["state"] == nil {
		t.Error("state is nil")
	}
}

func TestHandleIssues(t *testing.T) {
	o := NewOrchestrator(DefaultConfig())
	h := NewHandler(o)

	req := httptest.NewRequest("GET", "/api/v1/issues", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v", resp["status"])
	}
	if resp["issues"] == nil {
		t.Error("issues is nil")
	}
}

func TestHandleRefresh(t *testing.T) {
	o := NewOrchestrator(DefaultConfig())
	h := NewHandler(o)

	req := httptest.NewRequest("POST", "/api/v1/refresh", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v", resp["status"])
	}
	if !strings.Contains(resp["message"].(string), "refresh") {
		t.Errorf("message = %v", resp["message"])
	}
}

func TestHandler_ContentType(t *testing.T) {
	o := NewOrchestrator(DefaultConfig())
	h := NewHandler(o)
	req := httptest.NewRequest("GET", "/api/v1/state", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}
