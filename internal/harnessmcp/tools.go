package harnessmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ToolHandler is a function that handles a tool call.
type ToolHandler func(ctx context.Context, args json.RawMessage) (ToolResult, error)

// Clock is an interface for time operations, enabling deterministic testing.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// RealClock is a Clock that uses the real system time.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time { return time.Now() }

// After returns a channel that fires after duration d.
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// toolDefs returns the list of all 5 MCP tools exposed by this server.
func toolDefs() []Tool {
	return []Tool{
		{
			Name:        "start_run",
			Description: "Start a new agent run with the given prompt. Returns the run_id for tracking.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"prompt": {
						Type:        "string",
						Description: "The prompt to run",
					},
					"model": {
						Type:        "string",
						Description: "Model override (e.g. gpt-4.1-mini)",
					},
					"conversation_id": {
						Type:        "string",
						Description: "Conversation to attach run to",
					},
					"max_steps": {
						Type:        "integer",
						Description: "Maximum steps before stopping",
					},
					"max_cost_usd": {
						Type:        "number",
						Description: "Cost ceiling in USD",
					},
				},
				Required: []string{"prompt"},
			},
		},
		{
			Name:        "get_run_status",
			Description: "Get the current status of a run by ID. Returns status, messages, cost, and any error.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"run_id": {
						Type:        "string",
						Description: "Run ID returned by start_run",
					},
				},
				Required: []string{"run_id"},
			},
		},
		{
			Name:        "wait_for_run",
			Description: "Poll a run until it completes, fails, or times out. Blocks until the run reaches a terminal state.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"run_id": {
						Type:        "string",
						Description: "Run ID returned by start_run",
					},
					"timeout_seconds": {
						Type:        "integer",
						Description: "Max seconds to wait (default: 300)",
					},
				},
				Required: []string{"run_id"},
			},
		},
		{
			Name:        "continue_run",
			Description: "Continue an existing conversation by sending a follow-up prompt. Creates a new run in the same conversation.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"run_id": {
						Type:        "string",
						Description: "Run ID of the previous run to continue from",
					},
					"prompt": {
						Type:        "string",
						Description: "Follow-up prompt",
					},
				},
				Required: []string{"run_id", "prompt"},
			},
		},
		{
			Name:        "list_runs",
			Description: "List recent runs, optionally filtered by conversation.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"conversation_id": {
						Type:        "string",
						Description: "Filter by conversation ID",
					},
					"limit": {
						Type:        "integer",
						Description: "Max results (default: 20)",
					},
				},
			},
		},
	}
}

// newStartRunHandler returns a ToolHandler for the start_run tool.
func newStartRunHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			Prompt         string  `json:"prompt"`
			Model          string  `json:"model,omitempty"`
			ConversationID string  `json:"conversation_id,omitempty"`
			MaxSteps       int     `json:"max_steps,omitempty"`
			MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}}}, nil
		}
		if params.Prompt == "" {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "prompt is required"}}}, nil
		}

		resp, err := client.StartRun(ctx, StartRunRequest{
			Prompt:         params.Prompt,
			Model:          params.Model,
			ConversationID: params.ConversationID,
			MaxSteps:       params.MaxSteps,
			MaxCostUSD:     params.MaxCostUSD,
		})
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}

		result, err := json.Marshal(map[string]string{"run_id": resp.RunID})
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}
		return ToolResult{Content: []ContentBlock{{Type: "text", Text: string(result)}}}, nil
	}
}

// newGetRunStatusHandler returns a ToolHandler for the get_run_status tool.
func newGetRunStatusHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}}}, nil
		}
		if params.RunID == "" {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "run_id is required"}}}, nil
		}

		status, err := client.GetRun(ctx, params.RunID)
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}

		result, err := json.Marshal(map[string]any{
			"status":   status.Status,
			"messages": status.Messages,
			"cost_usd": status.CostUSD,
			"error":    status.Error,
		})
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}
		return ToolResult{Content: []ContentBlock{{Type: "text", Text: string(result)}}}, nil
	}
}

// newWaitForRunHandler returns a ToolHandler for the wait_for_run tool.
func newWaitForRunHandler(client *HarnessClient, clock Clock) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID          string `json:"run_id"`
			TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}}}, nil
		}
		if params.RunID == "" {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "run_id is required"}}}, nil
		}

		timeout := params.TimeoutSeconds
		if timeout <= 0 {
			timeout = 300
		}
		timeoutDur := time.Duration(timeout) * time.Second
		timeoutCh := clock.After(timeoutDur)

		for {
			status, err := client.GetRun(ctx, params.RunID)
			if err != nil {
				return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
			}

			switch status.Status {
			case "completed", "failed", "waiting_for_user":
				result, err := json.Marshal(map[string]any{
					"status":   status.Status,
					"messages": status.Messages,
					"cost_usd": status.CostUSD,
					"error":    status.Error,
				})
				if err != nil {
					return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
				}
				return ToolResult{Content: []ContentBlock{{Type: "text", Text: string(result)}}}, nil
			}

			// Wait 2 seconds before polling again.
			pollCh := clock.After(2 * time.Second)
			select {
			case <-ctx.Done():
				return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "cancelled"}}}, nil
			case <-timeoutCh:
				return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("timed out waiting for run %s", params.RunID)}}}, nil
			case <-pollCh:
				// Poll again.
			}
		}
	}
}

// newContinueRunHandler returns a ToolHandler for the continue_run tool.
func newContinueRunHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID  string `json:"run_id"`
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}}}, nil
		}
		if params.RunID == "" {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "run_id is required"}}}, nil
		}
		if params.Prompt == "" {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "prompt is required"}}}, nil
		}

		// Fetch the previous run to get its conversation_id.
		prevRun, err := client.GetRun(ctx, params.RunID)
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("get run: %v", err)}}}, nil
		}

		// Start a new run in the same conversation.
		resp, err := client.StartRun(ctx, StartRunRequest{
			Prompt:         params.Prompt,
			ConversationID: prevRun.ConversationID,
		})
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("start run: %v", err)}}}, nil
		}

		result, err := json.Marshal(map[string]string{"run_id": resp.RunID})
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}
		return ToolResult{Content: []ContentBlock{{Type: "text", Text: string(result)}}}, nil
	}
}

// newListRunsHandler returns a ToolHandler for the list_runs tool.
func newListRunsHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			ConversationID string `json:"conversation_id,omitempty"`
			Limit          int    `json:"limit,omitempty"`
		}
		if args != nil {
			if err := json.Unmarshal(args, &params); err != nil {
				return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}}}, nil
			}
		}

		limit := params.Limit
		if limit <= 0 {
			limit = 20
		}

		runs, err := client.ListRuns(ctx, ListRunsParams{
			ConversationID: params.ConversationID,
			Limit:          limit,
		})
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}

		result, err := json.Marshal(runs)
		if err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
		}
		return ToolResult{Content: []ContentBlock{{Type: "text", Text: string(result)}}}, nil
	}
}
