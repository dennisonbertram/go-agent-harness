package deferred

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/cron"
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

func TestCronUpdateChangesOnlyTheFieldsGivenWithExpectedVersion(t *testing.T) {
	client := &recordingCronUpdateClient{}
	tool := CronUpdateTool(client)

	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","schedule":"0 * * * *","expected_updated_at":"2026-07-31T00:00:00Z"}`))
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

func TestCronUpdateAcceptsHarnessPrompt(t *testing.T) {
	client := &recordingCronUpdateClient{}
	tool := CronUpdateTool(client)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","prompt":"check the updated deployment","expected_updated_at":"2026-07-30T23:00:00Z"}`))
	if err != nil {
		t.Fatalf("cron_update: %v", err)
	}
	if client.lastReq.ExecConfig == nil || *client.lastReq.ExecConfig != `{"prompt":"check the updated deployment"}` {
		t.Fatalf("prompt was not encoded as harness execution config: %#v", client.lastReq.ExecConfig)
	}
	properties, ok := tool.Definition.Parameters["properties"].(map[string]any)
	if !ok || properties["prompt"] == nil {
		t.Fatalf("cron_update schema does not expose prompt: %#v", tool.Definition.Parameters)
	}
	timeoutSchema, ok := properties["timeout_seconds"].(map[string]any)
	if !ok || timeoutSchema["minimum"] != 1 {
		t.Fatalf("cron_update schema does not require a positive timeout: %#v", properties["timeout_seconds"])
	}
}

func TestCronUpdateHarnessPromptRefreshesRoutingFromActiveRun(t *testing.T) {
	client := &recordingCronUpdateClient{}
	tool := CronUpdateTool(client)
	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		Model: "fixture-model", ProviderName: "effective-provider", AllowFallback: true,
		FallbackProviders: []string{"secondary", "tertiary"},
	})
	_, err := tool.Handler(ctx, json.RawMessage(`{"id":"job-1","prompt":"updated","expected_updated_at":"2026-07-30T23:00:00Z"}`))
	if err != nil {
		t.Fatalf("cron_update: %v", err)
	}
	var config struct {
		Model             string   `json:"model"`
		ProviderName      string   `json:"provider_name"`
		AllowFallback     bool     `json:"allow_fallback"`
		FallbackProviders []string `json:"fallback_providers"`
	}
	if client.lastReq.ExecConfig == nil || json.Unmarshal([]byte(*client.lastReq.ExecConfig), &config) != nil {
		t.Fatalf("execution config = %#v", client.lastReq.ExecConfig)
	}
	if config.Model != "fixture-model" || config.ProviderName != "effective-provider" ||
		!config.AllowFallback || !slices.Equal(config.FallbackProviders, []string{"secondary", "tertiary"}) {
		t.Fatalf("updated routing config = %#v", config)
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
		{name: "missing version", args: `{"id":"job-1","schedule":"0 * * * *"}`, want: "expected_updated_at is required"},
		{name: "no-op", args: `{"id":"job-1","expected_updated_at":"2026-07-31T00:00:00Z"}`, want: "at least one"},
		{name: "invalid timestamp", args: `{"id":"job-1","schedule":"0 * * * *","expected_updated_at":"later"}`, want: "expected_updated_at"},
		{name: "multiple execution inputs", args: `{"id":"job-1","command":"echo hi","prompt":"check it","expected_updated_at":"2026-07-31T00:00:00Z"}`, want: "only one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tool.Handler(context.Background(), json.RawMessage(tc.args))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCronUpdateRejectsUnsafeTimeout(t *testing.T) {
	client := &recordingCronUpdateClient{}
	tool := CronUpdateTool(client)
	for _, timeout := range []string{"0", "-1"} {
		_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","timeout_seconds":`+timeout+`,"expected_updated_at":"2026-07-31T00:00:00Z"}`))
		if err == nil || !strings.Contains(err.Error(), "timeout_seconds must be positive") {
			t.Fatalf("timeout %s error = %v, want actionable validation", timeout, err)
		}
	}
}

func TestCronUpdateSchemaRequiresVersion(t *testing.T) {
	tool := CronUpdateTool(&recordingCronUpdateClient{})
	required, ok := tool.Definition.Parameters["required"].([]string)
	if !ok {
		t.Fatal("required schema is not []string")
	}
	for _, field := range required {
		if field == "expected_updated_at" {
			return
		}
	}
	t.Fatal("expected_updated_at must be required for cron_update")
}

func TestCronUpdateReturnsClientError(t *testing.T) {
	tool := CronUpdateTool(&recordingCronUpdateClient{err: errors.New("conflict")})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","tags":"prod","expected_updated_at":"2026-07-31T00:00:00Z"}`))
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("error = %v, want client error", err)
	}
}

func TestCronPauseResumeRequireAndForwardExpectedVersion(t *testing.T) {
	want := time.Date(2026, 7, 31, 12, 34, 56, 789, time.UTC)
	for _, tc := range []struct {
		name   string
		status string
		tool   func(tools.CronClient) tools.Tool
	}{
		{name: "pause", status: "paused", tool: CronPauseTool},
		{name: "resume", status: "active", tool: CronResumeTool},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &recordingCronUpdateClient{}
			tool := tc.tool(client)
			required, ok := tool.Definition.Parameters["required"].([]string)
			if !ok || !slices.Contains(required, "expected_updated_at") {
				t.Fatalf("required schema = %#v, want expected_updated_at", tool.Definition.Parameters["required"])
			}
			if _, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`)); err == nil || !strings.Contains(err.Error(), "expected_updated_at is required") {
				t.Fatalf("missing version error = %v", err)
			}
			if _, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","expected_updated_at":"not-a-time"}`)); err == nil || !strings.Contains(err.Error(), "RFC3339") {
				t.Fatalf("invalid version error = %v", err)
			}
			args := `{"id":"job-1","expected_updated_at":"` + want.Format(time.RFC3339Nano) + `"}`
			if _, err := tool.Handler(context.Background(), json.RawMessage(args)); err != nil {
				t.Fatalf("versioned %s: %v", tc.name, err)
			}
			if client.lastReq.Status == nil || *client.lastReq.Status != tc.status {
				t.Fatalf("status = %#v, want %q", client.lastReq.Status, tc.status)
			}
			if client.lastReq.ExpectedUpdatedAt == nil || !client.lastReq.ExpectedUpdatedAt.Equal(want) {
				t.Fatalf("expected version = %#v, want %s", client.lastReq.ExpectedUpdatedAt, want)
			}
		})
	}
}

func TestNewScopedCronClientIsIdempotent(t *testing.T) {
	raw := &recordingCronUpdateClient{}
	first := NewScopedCronClient(raw)
	second := NewScopedCronClient(first)
	if first != second {
		t.Fatal("model registry wrapping an already-scoped cron client added a second wrapper")
	}
}

type recordingScopedDeleteClient struct {
	tools.CronClient
	job         tools.CronJob
	getID       string
	deleteID    string
	getScope    cron.Scope
	deleteScope cron.Scope
}

func (c *recordingScopedDeleteClient) GetJob(ctx context.Context, id string) (tools.CronJob, error) {
	c.getID = id
	c.getScope, _ = cron.ScopeFromContext(ctx)
	return c.job, nil
}

func (c *recordingScopedDeleteClient) DeleteJob(ctx context.Context, id string) error {
	c.deleteID = id
	c.deleteScope, _ = cron.ScopeFromContext(ctx)
	return nil
}

func TestScopedCronClientUnversionedDeleteEnforcesScope(t *testing.T) {
	metadata := tools.RunMetadata{TenantID: "tenant-a", ConversationID: "conversation-a", AgentID: "agent-a"}
	ctx := context.WithValue(context.Background(), tools.ContextKeyRunMetadata, metadata)
	wantScope := cron.Scope{TenantID: metadata.TenantID, ConversationID: metadata.ConversationID, AgentID: metadata.AgentID}
	raw := &recordingScopedDeleteClient{job: tools.CronJob{
		ID:             "job-a",
		TenantID:       metadata.TenantID,
		ConversationID: metadata.ConversationID,
		AgentID:        metadata.AgentID,
	}}
	scoped := NewScopedCronClient(raw)
	if err := scoped.DeleteJob(ctx, "job-a"); err != nil {
		t.Fatalf("delete owned job: %v", err)
	}
	if raw.getID != "job-a" || raw.deleteID != "job-a" {
		t.Fatalf("get/delete IDs = %q/%q, want job-a/job-a", raw.getID, raw.deleteID)
	}
	if raw.getScope != wantScope || raw.deleteScope != wantScope {
		t.Fatalf("forwarded scopes = %#v/%#v, want %#v", raw.getScope, raw.deleteScope, wantScope)
	}

	raw.job.TenantID = "tenant-b"
	raw.deleteID = ""
	if err := scoped.DeleteJob(ctx, "job-b"); !errors.Is(err, tools.ErrCronJobNotFound) {
		t.Fatalf("cross-scope delete error = %v, want not found", err)
	}
	if raw.deleteID != "" {
		t.Fatalf("cross-scope delete reached underlying mutation for %q", raw.deleteID)
	}
}
