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
						Description: "Cost ceiling in USD. Enforced only when pricing data is available; unpriced models are never terminated by it.",
					},
					"workspace_type": {
						Type:        "string",
						Description: "Workspace backend: local, worktree, container, or vm. Omit to run in the daemon's own workspace. Use worktree to keep a delegated write off the caller's checkout.",
					},
					"extra_dirs": {
						Type:        "array",
						Description: "Absolute directory roots the run may read beyond its workspace",
					},
					"allowed_tools": {
						Type:        "array",
						Description: "Restrict the run to these tool names. Empty means no restriction.",
					},
					"denied_tools": {
						Type:        "array",
						Description: "Remove these tool names from the run",
					},
					"profile": {
						Type:        "string",
						Description: "Server-side profile selecting tool set, isolation, and limits",
					},
					"system_prompt": {
						Type:        "string",
						Description: "Override the system prompt for this run",
					},
					"provider_name": {
						Type:        "string",
						Description: "Force a specific catalog provider instead of resolving from the model",
					},
					"reasoning_effort": {
						Type:        "string",
						Description: "Thinking budget: low, medium, or high",
					},
					"max_turns": {
						Type:        "integer",
						Description: "Cap on assistant turns",
					},
					"plan_mode": {
						Type:        "boolean",
						Description: "Start read-only in planning mode; mutation limited to the plan file until approved",
					},
					"plan_file": {
						Type:        "string",
						Description: "Workspace-relative plan artifact for plan_mode (default .harness/plan.md)",
					},
					"agent_intent": {
						Type:        "string",
						Description: "Intent overlay for the system prompt",
					},
					"task_context": {
						Type:        "string",
						Description: "Extra context appended to the prompt",
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
			Name:        "cancel_run",
			Description: "Cancel an in-flight run.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID to cancel"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "approve_run",
			Description: "Approve a run that is paused awaiting approval.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID to approve"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "deny_run",
			Description: "Deny a run that is paused awaiting approval.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID to deny"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "steer_run",
			Description: "Inject a guidance message into an in-flight run without restarting it.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"run_id": {Type: "string", Description: "Run ID to steer"},
					"prompt": {Type: "string", Description: "Guidance to inject"},
				},
				Required: []string{"run_id", "prompt"},
			},
		},
		{
			Name:        "tail_run_events",
			Description: "Poll a run's event stream for progress. Pass last_event_id back as after_event_id to get only newer events. Returns promptly when the run is quiet.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"run_id":         {Type: "string", Description: "Run ID to tail"},
					"after_event_id": {Type: "string", Description: "Cursor from the previous poll's last_event_id; omit to start from the beginning"},
					"max_events":     {Type: "integer", Description: "Max events to return (default 100)"},
					"wait_seconds":   {Type: "integer", Description: "How long to wait for events before returning (default 2)"},
				},
				Required: []string{"run_id"},
			},
		},
		{
			Name:        "get_run_input",
			Description: "Read the question a run is waiting on when its status is waiting_for_user.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "submit_user_input",
			Description: "Answer a run waiting on a question, so it resumes instead of being cancelled.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"run_id":  {Type: "string", Description: "Run ID"},
					"answers": {Type: "object", Description: "One entry per question: key is the question text exactly as returned by get_run_input, value is one of that question's option labels"},
				},
				Required: []string{"run_id", "answers"},
			},
		},
		{
			Name:        "get_run_todos",
			Description: "Read a run's todo list — what it has done and what it plans to do next.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "get_run_summary",
			Description: "Read a run's summary.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "get_run_context",
			Description: "Read a run's context-window usage.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "compact_run",
			Description: "Compact a run's context to free space.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{"run_id": {Type: "string", Description: "Run ID"}},
				Required:   []string{"run_id"},
			},
		},
		{
			Name:        "list_models",
			Description: "List models this daemon can route to, with provider and context window.",
			InputSchema: InputSchema{Type: "object", Properties: map[string]Property{}},
		},
		{
			Name:        "list_providers",
			Description: "List providers with whether a credential is configured and whether it is known to work (health: ok, unverified, failed, unconfigured).",
			InputSchema: InputSchema{Type: "object", Properties: map[string]Property{}},
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
		// Mirrors StartRunRequest; unset fields stay zero so they are omitted
		// from the posted body and behavior is unchanged for prompt-only calls.
		var params StartRunRequest
		if err := json.Unmarshal(args, &params); err != nil {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}}}, nil
		}
		if params.Prompt == "" {
			return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: "prompt is required"}}}, nil
		}

		resp, err := client.StartRun(ctx, params)
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
			"status": status.Status,
			// output is the run's result text — the whole point of delegating.
			// It was previously dropped entirely (issue #1314).
			"output":   status.Output,
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
			// Terminal per the runner (isTerminalRunStatus: completed, failed,
			// cancelled) plus waiting_for_user, which blocks on input and so
			// ends a blocking wait. "cancelled" was missing, so cancelling a run
			// left this polling until its timeout (issue #1319).
			case "completed", "failed", "cancelled", "waiting_for_user":
				result, err := json.Marshal(map[string]any{
					"status": status.Status,
					// output is the run's result text (issue #1314).
					"output":   status.Output,
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

// runControlHandler builds a handler for a run-control tool that takes only a
// run_id and returns success or the server's error.
func runControlHandler(action func(context.Context, string) error) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if params.RunID == "" {
			return errorResult("run_id is required"), nil
		}
		if err := action(ctx, params.RunID); err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"run_id": params.RunID, "ok": true})
	}
}

// newCancelRunHandler returns a ToolHandler for the cancel_run tool.
func newCancelRunHandler(client *HarnessClient) ToolHandler {
	return runControlHandler(client.CancelRun)
}

// newApproveRunHandler returns a ToolHandler for the approve_run tool.
func newApproveRunHandler(client *HarnessClient) ToolHandler {
	return runControlHandler(client.ApproveRun)
}

// newDenyRunHandler returns a ToolHandler for the deny_run tool.
func newDenyRunHandler(client *HarnessClient) ToolHandler {
	return runControlHandler(client.DenyRun)
}

// newSteerRunHandler returns a ToolHandler for the steer_run tool.
func newSteerRunHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID  string `json:"run_id"`
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if params.RunID == "" || params.Prompt == "" {
			return errorResult("run_id and prompt are required"), nil
		}
		if err := client.SteerRun(ctx, params.RunID, params.Prompt); err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"run_id": params.RunID, "ok": true})
	}
}

// newListModelsHandler returns a ToolHandler for the list_models tool.
func newListModelsHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, _ json.RawMessage) (ToolResult, error) {
		models, err := client.ListModels(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"models": models})
	}
}

// newListProvidersHandler returns a ToolHandler for the list_providers tool.
func newListProvidersHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, _ json.RawMessage) (ToolResult, error) {
		providers, err := client.ListProviders(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"providers": providers})
	}
}

// errorResult wraps a message as a failed ToolResult.
func errorResult(msg string) ToolResult {
	return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: msg}}}
}

// jsonResult marshals v into a text ToolResult.
func jsonResult(v any) (ToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return ToolResult{Content: []ContentBlock{{Type: "text", Text: string(b)}}}, nil
}

// newTailRunEventsHandler returns a ToolHandler for the tail_run_events tool.
//
// Polling with a cursor is how a delegating agent sees progress: an MCP tool call
// is request/response, so it cannot stream into an in-flight call (issue #1323).
func newTailRunEventsHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID        string `json:"run_id"`
			AfterEventID string `json:"after_event_id,omitempty"`
			MaxEvents    int    `json:"max_events,omitempty"`
			WaitSeconds  int    `json:"wait_seconds,omitempty"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if params.RunID == "" {
			return errorResult("run_id is required"), nil
		}
		window := time.Duration(params.WaitSeconds) * time.Second
		events, cursor, err := client.TailRunEvents(ctx, params.RunID, params.AfterEventID, params.MaxEvents, window)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{
			"events": events,
			// Pass this back as after_event_id next poll to avoid replaying.
			"last_event_id": cursor,
			"count":         len(events),
		})
	}
}

// newGetRunInputHandler returns a ToolHandler for the get_run_input tool.
func newGetRunInputHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		runID, errRes := requireRunID(args)
		if errRes != nil {
			return *errRes, nil
		}
		pending, err := client.PendingInput(ctx, runID)
		if err != nil {
			// "not waiting for input" is an ordinary state, not a fault; the
			// server's message already says so, so pass it through plainly.
			return errorResult(err.Error()), nil
		}
		return jsonResult(pending)
	}
}

// newSubmitUserInputHandler returns a ToolHandler for the submit_user_input tool.
func newSubmitUserInputHandler(client *HarnessClient) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var params struct {
			RunID   string            `json:"run_id"`
			Answers map[string]string `json:"answers"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if params.RunID == "" {
			return errorResult("run_id is required"), nil
		}
		if len(params.Answers) == 0 {
			return errorResult("answers is required — map each question id to its answer"), nil
		}
		if err := client.SubmitInput(ctx, params.RunID, params.Answers); err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"run_id": params.RunID, "ok": true})
	}
}

// newGetRunTodosHandler returns a ToolHandler for the get_run_todos tool.
func newGetRunTodosHandler(client *HarnessClient) ToolHandler {
	return runSubresourceHandler(client.RunTodos)
}

// newGetRunSummaryHandler returns a ToolHandler for the get_run_summary tool.
func newGetRunSummaryHandler(client *HarnessClient) ToolHandler {
	return runSubresourceHandler(client.RunSummary)
}

// newGetRunContextHandler returns a ToolHandler for the get_run_context tool.
func newGetRunContextHandler(client *HarnessClient) ToolHandler {
	return runSubresourceHandler(client.RunContext)
}

// newCompactRunHandler returns a ToolHandler for the compact_run tool.
func newCompactRunHandler(client *HarnessClient) ToolHandler {
	return runControlHandler(client.CompactRun)
}

// runSubresourceHandler adapts a run subresource fetch into a ToolHandler.
func runSubresourceHandler(fetch func(context.Context, string) (any, error)) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		runID, errRes := requireRunID(args)
		if errRes != nil {
			return *errRes, nil
		}
		out, err := fetch(ctx, runID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(out)
	}
}

// requireRunID parses and validates the common {"run_id": "..."} argument shape.
func requireRunID(args json.RawMessage) (string, *ToolResult) {
	var params struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		res := errorResult(fmt.Sprintf("invalid arguments: %v", err))
		return "", &res
	}
	if params.RunID == "" {
		res := errorResult("run_id is required")
		return "", &res
	}
	return params.RunID, nil
}
