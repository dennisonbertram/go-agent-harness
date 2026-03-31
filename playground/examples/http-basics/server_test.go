package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rw := httptest.NewRecorder()
	healthHandler(rw, req)
	resp := rw.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	err := json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		t.Fatalf("Failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", body["status"])
	}
}

func TestEchoHandler_Post(t *testing.T) {
	payload := map[string]string{"msg": "hello"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	echoHandler(rw, req)
	resp := rw.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var out map[string]string
	err := json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}
	if out["msg"] != "hello" {
		t.Errorf("Expected msg=hello, got %v", out["msg"])
	}
}

func TestEchoHandler_Get_Disallowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/echo", nil)
	rw := httptest.NewRecorder()
	echoHandler(rw, req)
	resp := rw.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("Expected 405, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(bytes.TrimSpace(b)) != "method not allowed" {
		t.Errorf("Expected 'method not allowed', got '%s'", string(b))
	}
}
