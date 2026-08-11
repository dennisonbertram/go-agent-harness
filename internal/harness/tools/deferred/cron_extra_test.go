package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

type blankIDRecordingClient struct {
	tools.CronClient
	calls int
}

func (c *blankIDRecordingClient) GetJob(context.Context, string) (tools.CronJob, error) {
	c.calls++
	return tools.CronJob{}, nil
}

func (c *blankIDRecordingClient) UpdateJob(context.Context, string, tools.CronUpdateJobRequest) (tools.CronJob, error) {
	c.calls++
	return tools.CronJob{}, nil
}

func (c *blankIDRecordingClient) DeleteJobCAS(context.Context, string, time.Time) error {
	c.calls++
	return nil
}

func (c *blankIDRecordingClient) ListExecutions(context.Context, string, int, int) ([]tools.CronExecution, error) {
	c.calls++
	return nil, nil
}

type recordingCronClient struct {
	tools.CronClient
	lastUpdateID  string
	lastUpdate    tools.CronUpdateJobRequest
	lastExecLimit int
	lastExecOff   int
}

func (c *recordingCronClient) UpdateJob(
	_ context.Context, id string, req tools.CronUpdateJobRequest,
) (tools.CronJob, error) {
	c.lastUpdateID = id
	c.lastUpdate = req
	return tools.CronJob{ID: id}, nil
}

func (c *recordingCronClient) ListExecutions(
	_ context.Context, _ string, limit, offset int,
) ([]tools.CronExecution, error) {
	c.lastExecLimit = limit
	c.lastExecOff = offset
	return []tools.CronExecution{{}}, nil
}

// Editing in place is the point: delete-then-create gives the job a new ID and
// discards the execution history, which is the only record of whether it ever
// worked.
func TestCronUpdateChangesOnlyTheFieldsGiven(t *testing.T) {
	client := &recordingCronClient{}
	tool := CronUpdateTool(client)

	_, err := tool.Handler(context.Background(),
		json.RawMessage(`{"id":"job-1","schedule":"0 * * * *","expected_updated_at":"2026-08-01T00:00:00Z"}`))
	if err != nil {
		t.Fatalf("cron_update: %v", err)
	}
	if client.lastUpdateID != "job-1" {
		t.Errorf("updated %q, want job-1", client.lastUpdateID)
	}
	if client.lastUpdate.Schedule == nil || *client.lastUpdate.Schedule != "0 * * * *" {
		t.Error("schedule was not passed through")
	}
	// Omitted fields must stay nil so the server leaves them alone.
	if client.lastUpdate.ExecConfig != nil || client.lastUpdate.TimeoutSec != nil {
		t.Error("omitted fields were sent, which would overwrite them")
	}
}

// A call that changes nothing almost certainly means the caller used a field
// name this tool does not accept; reporting success would hide that.
func TestCronUpdateRejectsANoOp(t *testing.T) {
	tool := CronUpdateTool(&recordingCronClient{})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1","expected_updated_at":"2026-08-01T00:00:00Z"}`))
	if err == nil {
		t.Fatal("a no-op update was accepted")
	}
	if !strings.Contains(err.Error(), "schedule") {
		t.Errorf("error %q does not say which fields it accepts", err)
	}
}

func TestCronHistoryClampsPaging(t *testing.T) {
	client := &recordingCronClient{}
	tool := CronHistoryTool(client)

	// Default when unset.
	if _, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"j"}`)); err != nil {
		t.Fatalf("cron_history: %v", err)
	}
	if client.lastExecLimit != 20 {
		t.Errorf("default limit %d, want 20", client.lastExecLimit)
	}

	// Clamped when absurd, so one call cannot pull the whole table.
	if _, err := tool.Handler(context.Background(),
		json.RawMessage(`{"id":"j","limit":10000,"offset":-5}`)); err != nil {
		t.Fatalf("cron_history: %v", err)
	}
	if client.lastExecLimit != 100 {
		t.Errorf("clamped limit %d, want 100", client.lastExecLimit)
	}
	if client.lastExecOff != 0 {
		t.Errorf("negative offset became %d, want 0", client.lastExecOff)
	}
}

func TestCronCRUDSchemasRequireJobIDAndDoNotAdvertiseNames(t *testing.T) {
	client := &recordingCronClient{}
	for _, tool := range []tools.Tool{CronGetTool(client), CronUpdateTool(client), CronPauseTool(client), CronResumeTool(client), CronDeleteTool(client), CronHistoryTool(client)} {
		props := tool.Definition.Parameters["properties"].(map[string]any)
		if _, ok := props["name"]; ok {
			t.Fatalf("%s advertises forbidden name lookup", tool.Definition.Name)
		}
		id := props["id"].(map[string]any)
		description, _ := id["description"].(string)
		if !strings.Contains(strings.ToLower(description), "id only") {
			t.Fatalf("%s id description = %q, want explicit ID-only contract", tool.Definition.Name, description)
		}
	}
}

func TestCronExistingJobToolsRejectBlankIDsBeforeClientCalls(t *testing.T) {
	const version = "2026-08-01T00:00:00Z"
	for _, id := range []string{"", " \t "} {
		for _, tc := range []struct {
			name string
			tool func(tools.CronClient) tools.Tool
			args func(string) string
		}{
			{name: "get", tool: CronGetTool, args: func(id string) string { return fmt.Sprintf(`{"id":%q}`, id) }},
			{name: "delete", tool: CronDeleteTool, args: func(id string) string {
				return fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, id, version)
			}},
			{name: "pause", tool: CronPauseTool, args: func(id string) string {
				return fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, id, version)
			}},
			{name: "resume", tool: CronResumeTool, args: func(id string) string {
				return fmt.Sprintf(`{"id":%q,"expected_updated_at":%q}`, id, version)
			}},
			{name: "update", tool: CronUpdateTool, args: func(id string) string {
				return fmt.Sprintf(`{"id":%q,"tags":"changed","expected_updated_at":%q}`, id, version)
			}},
			{name: "history", tool: CronHistoryTool, args: func(id string) string { return fmt.Sprintf(`{"id":%q}`, id) }},
		} {
			t.Run(fmt.Sprintf("%s/%q", tc.name, id), func(t *testing.T) {
				client := &blankIDRecordingClient{}
				_, err := tc.tool(client).Handler(context.Background(), json.RawMessage(tc.args(id)))
				if err == nil || !strings.Contains(err.Error(), "id is required") {
					t.Fatalf("error = %v, want id is required", err)
				}
				if client.calls != 0 {
					t.Fatalf("client calls = %d, want 0", client.calls)
				}
			})
		}
	}
}
