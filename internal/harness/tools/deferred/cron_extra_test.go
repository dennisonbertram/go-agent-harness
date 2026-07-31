package deferred

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

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
		json.RawMessage(`{"id":"job-1","schedule":"0 * * * *"}`))
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
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"job-1"}`))
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
