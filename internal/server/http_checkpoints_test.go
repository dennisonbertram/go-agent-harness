package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/checkpoints"
)

func TestHandleCheckpointResume(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	checkpointSvc := checkpoints.NewService(checkpoints.NewMemoryStore(), func() time.Time { return now })
	record, err := checkpointSvc.Create(context.Background(), checkpoints.CreateRequest{
		Kind:          checkpoints.KindExternalResume,
		WorkflowRunID: "wf-1",
		DeadlineAt:    now.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	handler := NewWithOptions(ServerOptions{
		AuthDisabled: true,
		Checkpoints:  checkpointSvc,
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Post(
		ts.URL+"/v1/checkpoints/"+record.ID+"/resume",
		"application/json",
		bytes.NewBufferString(`{"payload":{"decision":"continue"}}`),
	)
	if err != nil {
		t.Fatalf("POST resume: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	loaded, err := checkpointSvc.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != checkpoints.StatusResumed {
		t.Fatalf("status = %q, want %q", loaded.Status, checkpoints.StatusResumed)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "resumed" {
		t.Fatalf("response status = %v, want resumed", body["status"])
	}
}
