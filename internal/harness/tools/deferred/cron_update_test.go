package deferred

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

type recordingCronUpdateClient struct {
	tools.CronClient
	lastID  string
	lastReq tools.CronUpdateJobRequest
	err     error
}

func (c *recordingCronUpdateClient) UpdateJob(_ context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error) {
	c.lastID = id
	c.lastReq = req
	if c.err != nil {
		return tools.CronJob{}, c.err
	}
	return tools.CronJob{ID: id, Schedule: "0 * * * *", UpdatedAt: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)}, nil
}

func TestCronUpdateChangesOnlyTheFieldsGiven(t *testing.T) {
	client := &recordingCronUpdateClient{}
	tool := CronUpdateTool(client)

	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","schedule":"0 * * * *"}`))
	if err != nil {
		t.Fatalf("cron_update: %v", err)
	}
	if client.lastID != "job-1" {
		t.Fatalf("updated %q, want job-1", client.lastID)
	}
	if client.lastReq.Schedule == nil || *client.lastReq.Schedule != "0 * * * *" {
		t.Fatal("schedule was not forwarded")
	}
	if client.lastReq.ExecConfig != nil || client.lastReq.TimeoutSec != nil || client.lastReq.Tags != nil {
		t.Fatal("omitted fields were forwarded and could overwrite existing values")
	}
	if !strings.Contains(result, "job-1") {
		t.Fatalf("result did not contain updated job: %s", result)
	}
}

func TestCronUpdateAcceptsCommandAndExpectedTimestamp(t *testing.T) {
	client := &recordingCronUpdateClient{}
	tool := CronUpdateTool(client)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","command":"echo ready","expected_updated_at":"2026-07-30T23:00:00Z"}`))
	if err != nil {
		t.Fatalf("cron_update: %v", err)
	}
	if client.lastReq.ExecConfig == nil || *client.lastReq.ExecConfig != `{"command":"echo ready"}` {
		t.Fatalf("command was not encoded as execution config: %#v", client.lastReq.ExecConfig)
	}
	want := time.Date(2026, 7, 30, 23, 0, 0, 0, time.UTC)
	if client.lastReq.ExpectedUpdatedAt == nil || !client.lastReq.ExpectedUpdatedAt.Equal(want) {
		t.Fatalf("expected timestamp = %v, got %#v", want, client.lastReq.ExpectedUpdatedAt)
	}
}

func TestCronUpdateRejectsNoOpAndInvalidInput(t *testing.T) {
	tool := CronUpdateTool(&recordingCronUpdateClient{})
	for _, tc := range []struct {
		name string
		args string
		want string
	}{
		{name: "missing id", args: `{"schedule":"0 * * * *"}`, want: "id is required"},
		{name: "no-op", args: `{"id":"job-1"}`, want: "at least one"},
		{name: "invalid timestamp", args: `{"id":"job-1","schedule":"0 * * * *","expected_updated_at":"later"}`, want: "expected_updated_at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tool.Handler(context.Background(), json.RawMessage(tc.args))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCronUpdateReturnsClientError(t *testing.T) {
	tool := CronUpdateTool(&recordingCronUpdateClient{err: errors.New("conflict")})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","tags":"prod"}`))
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("error = %v, want client error", err)
	}
}
