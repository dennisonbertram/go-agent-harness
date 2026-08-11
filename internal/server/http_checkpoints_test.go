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

func TestHandleCheckpointResumeAlreadyResolvedIsConflictAndDoesNotMutate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		resolve func(*checkpoints.Service, string) error
	}{
		{name: "resumed", resolve: func(s *checkpoints.Service, id string) error {
			return s.Resume(context.Background(), id, map[string]any{"decision": "first"})
		}},
		{name: "expired", resolve: func(s *checkpoints.Service, id string) error {
			return s.Expire(context.Background(), id)
		}},
		{name: "denied", resolve: func(s *checkpoints.Service, id string) error {
			return s.Deny(context.Background(), id)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
			service := checkpoints.NewService(checkpoints.NewMemoryStore(), func() time.Time { return now })
			record, err := service.Create(context.Background(), checkpoints.CreateRequest{
				Kind: checkpoints.KindExternalResume, DeadlineAt: now.Add(time.Minute),
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if err := tt.resolve(service, record.ID); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			before, err := service.Get(context.Background(), record.ID)
			if err != nil {
				t.Fatalf("Get before: %v", err)
			}

			ts := httptest.NewServer(NewWithOptions(ServerOptions{
				AuthDisabled: true,
				Checkpoints:  service,
			}))
			defer ts.Close()
			resp, err := http.Post(
				ts.URL+"/v1/checkpoints/"+record.ID+"/resume",
				"application/json",
				bytes.NewBufferString(`{"payload":{"decision":"second"}}`),
			)
			if err != nil {
				t.Fatalf("POST second resume: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode conflict: %v", err)
			}
			if body.Error.Code != "already_resolved" {
				t.Fatalf("code = %q, want already_resolved", body.Error.Code)
			}
			after, err := service.Get(context.Background(), record.ID)
			if err != nil {
				t.Fatalf("Get after: %v", err)
			}
			if after.Status != before.Status || after.ResumePayload != before.ResumePayload || !after.UpdatedAt.Equal(before.UpdatedAt) {
				t.Fatalf("checkpoint mutated: before=%+v after=%+v", before, after)
			}
		})
	}
}
