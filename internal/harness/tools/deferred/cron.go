package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

func strPtr(s string) *string { return &s }

func harnessExecutionConfig(prompt string, metadata tools.RunMetadata) map[string]any {
	config := map[string]any{"prompt": prompt}
	if model := strings.TrimSpace(metadata.Model); model != "" {
		config["model"] = model
	}
	if providerName := strings.TrimSpace(metadata.ProviderName); providerName != "" {
		config["provider_name"] = providerName
	}
	if metadata.AllowFallback {
		config["allow_fallback"] = true
	}
	if len(metadata.FallbackProviders) > 0 {
		config["fallback_providers"] = append([]string(nil), metadata.FallbackProviders...)
	}
	return config
}

func requireCronJobID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

func requiredExpectedUpdatedAt(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, fmt.Errorf("expected_updated_at is required; call cron_get first")
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, fmt.Errorf("expected_updated_at must be an RFC3339 timestamp: %w", err)
	}
	return &parsed, nil
}

// CronCreateTool returns a deferred tool for creating cron jobs.
func CronCreateTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:        "cron_create",
		Description: descriptions.Load("cron_create"),
		Action:      tools.ActionExecute,
		Mutating:    true,
		Tier:        tools.TierDeferred,
		Tags:        []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":            map[string]any{"type": "string", "description": "Unique name for the cron job"},
				"schedule":        map[string]any{"type": "string", "description": "Standard 5-field cron expression: <minute> <hour> <day-of-month> <month> <day-of-week>. All times are UTC. Must be a literal string — no shell substitutions or variables. Examples: \"*/5 * * * *\" = every 5 minutes, \"0 * * * *\" = every hour on the hour, \"30 2 * * *\" = daily at 02:30 UTC, \"0 9 * * 1-5\" = weekdays at 09:00 UTC, \"0 0 1 * *\" = first of every month at midnight UTC. To schedule relative to 'now', first run the bash tool to get the current UTC time, then compute the desired cron fields yourself."},
				"execution_type":  map[string]any{"type": "string", "enum": []string{"shell", "harness"}, "description": "shell for a legacy command or harness for a conversational prompt"},
				"command":         map[string]any{"type": "string", "description": "Shell command to execute on each trigger; valid for execution_type shell"},
				"prompt":          map[string]any{"type": "string", "description": "Prompt to send to the current conversation on each trigger; valid for execution_type harness"},
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "description": "Max execution time in seconds (default 30); must be positive. The job is killed if it exceeds this."},
			},
			"required": []string{"name", "schedule"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name           string `json:"name"`
			Schedule       string `json:"schedule"`
			ExecutionType  string `json:"execution_type"`
			Command        string `json:"command"`
			Prompt         string `json:"prompt"`
			TimeoutSeconds *int   `json:"timeout_seconds"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_create args: %w", err)
		}
		timeoutSeconds := 30
		if args.TimeoutSeconds != nil {
			if *args.TimeoutSeconds <= 0 {
				return "", fmt.Errorf("timeout_seconds must be positive")
			}
			timeoutSeconds = *args.TimeoutSeconds
		}

		executionType := strings.TrimSpace(args.ExecutionType)
		if executionType == "" {
			executionType = "shell"
		}
		metadata, _ := tools.RunMetadataFromContext(ctx)
		var execCfg any
		switch executionType {
		case "shell":
			if strings.TrimSpace(args.Command) == "" || strings.TrimSpace(args.Prompt) != "" {
				return "", fmt.Errorf("shell cron_create requires a non-empty command and does not accept prompt")
			}
			execCfg = map[string]string{"command": args.Command}
		case "harness":
			if strings.TrimSpace(args.Prompt) == "" || strings.TrimSpace(args.Command) != "" {
				return "", fmt.Errorf("harness cron_create requires prompt and does not accept command")
			}
			execCfg = harnessExecutionConfig(args.Prompt, metadata)
		default:
			return "", fmt.Errorf("execution_type must be shell or harness")
		}
		execConfig, err := json.Marshal(execCfg)
		if err != nil {
			return "", fmt.Errorf("marshal exec config: %w", err)
		}
		job, err := client.CreateJob(ctx, tools.CronCreateJobRequest{
			Name:           args.Name,
			Schedule:       args.Schedule,
			ExecType:       executionType,
			ExecConfig:     string(execConfig),
			TimeoutSec:     timeoutSeconds,
			TenantID:       strings.TrimSpace(metadata.TenantID),
			ConversationID: strings.TrimSpace(metadata.ConversationID),
			AgentID:        strings.TrimSpace(metadata.AgentID),
		})
		if err != nil {
			return "", fmt.Errorf("cron_create failed: %w", err)
		}
		return tools.MarshalToolResult(job)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// CronListTool returns a deferred tool for listing cron jobs.
func CronListTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:         "cron_list",
		Description:  descriptions.Load("cron_list"),
		Action:       tools.ActionList,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		jobs, err := client.ListJobs(ctx)
		if err != nil {
			return "", fmt.Errorf("cron_list failed: %w", err)
		}
		return tools.MarshalToolResult(jobs)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// CronGetTool returns a deferred tool for getting a cron job's details.
func CronGetTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:         "cron_get",
		Description:  descriptions.Load("cron_get"),
		Action:       tools.ActionRead,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Job ID only; names are not accepted"},
			},
			"required": []string{"id"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_get args: %w", err)
		}
		if err := requireCronJobID(args.ID); err != nil {
			return "", err
		}

		job, err := client.GetJob(ctx, args.ID)
		if err != nil {
			return "", fmt.Errorf("cron_get failed: %w", err)
		}

		execs, execErr := client.ListExecutions(ctx, args.ID, 5, 0)
		historyAvailable := execErr == nil
		if execErr != nil {
			execs = []tools.CronExecution{}
		}

		result := map[string]any{
			"job":                         job,
			"recent_executions":           execs,
			"recent_executions_available": historyAvailable,
		}
		if execErr != nil {
			// Keep the job readable and preserve the established [] result shape,
			// but never present an unavailable history query as proof that the job
			// has not run. Models use this distinction to diagnose automations.
			result["recent_executions_warning"] = fmt.Sprintf("recent execution history unavailable: %v", execErr)
		}
		return tools.MarshalToolResult(result)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// CronDeleteTool returns a deferred tool for deleting a cron job.
func CronDeleteTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:        "cron_delete",
		Description: descriptions.Load("cron_delete"),
		Action:      tools.ActionExecute,
		Mutating:    true,
		Tier:        tools.TierDeferred,
		Tags:        []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":                  map[string]any{"type": "string", "description": "Job ID only; names are not accepted"},
				"expected_updated_at": map[string]any{"type": "string", "format": "date-time", "description": "updated_at from cron_get; rejects stale delete requests"},
			},
			"required": []string{"id", "expected_updated_at"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ID                string  `json:"id"`
			ExpectedUpdatedAt *string `json:"expected_updated_at"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_delete args: %w", err)
		}
		if err := requireCronJobID(args.ID); err != nil {
			return "", err
		}
		expectedUpdatedAt, err := requiredExpectedUpdatedAt(args.ExpectedUpdatedAt)
		if err != nil {
			return "", err
		}

		if err := client.DeleteJobCAS(ctx, args.ID, *expectedUpdatedAt); err != nil {
			return "", fmt.Errorf("cron_delete failed: %w", err)
		}

		return tools.MarshalToolResult(map[string]any{
			"deleted": true,
			"id":      args.ID,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// CronPauseTool returns a deferred tool for pausing a cron job.
func CronPauseTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:        "cron_pause",
		Description: descriptions.Load("cron_pause"),
		Action:      tools.ActionExecute,
		Mutating:    true,
		Tier:        tools.TierDeferred,
		Tags:        []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":                  map[string]any{"type": "string", "description": "Job ID only; names are not accepted"},
				"expected_updated_at": map[string]any{"type": "string", "format": "date-time", "description": "updated_at from cron_get; rejects stale pause requests"},
			},
			"required": []string{"id", "expected_updated_at"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ID                string  `json:"id"`
			ExpectedUpdatedAt *string `json:"expected_updated_at"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_pause args: %w", err)
		}
		if err := requireCronJobID(args.ID); err != nil {
			return "", err
		}
		expectedUpdatedAt, err := requiredExpectedUpdatedAt(args.ExpectedUpdatedAt)
		if err != nil {
			return "", err
		}

		job, err := client.UpdateJob(ctx, args.ID, tools.CronUpdateJobRequest{
			Status:            strPtr("paused"),
			ExpectedUpdatedAt: expectedUpdatedAt,
		})
		if err != nil {
			return "", fmt.Errorf("cron_pause failed: %w", err)
		}
		return tools.MarshalToolResult(job)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// CronResumeTool returns a deferred tool for resuming a paused cron job.
func CronResumeTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:        "cron_resume",
		Description: descriptions.Load("cron_resume"),
		Action:      tools.ActionExecute,
		Mutating:    true,
		Tier:        tools.TierDeferred,
		Tags:        []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":                  map[string]any{"type": "string", "description": "Job ID only; names are not accepted"},
				"expected_updated_at": map[string]any{"type": "string", "format": "date-time", "description": "updated_at from cron_get; rejects stale resume requests"},
			},
			"required": []string{"id", "expected_updated_at"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ID                string  `json:"id"`
			ExpectedUpdatedAt *string `json:"expected_updated_at"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_resume args: %w", err)
		}
		if err := requireCronJobID(args.ID); err != nil {
			return "", err
		}
		expectedUpdatedAt, err := requiredExpectedUpdatedAt(args.ExpectedUpdatedAt)
		if err != nil {
			return "", err
		}

		job, err := client.UpdateJob(ctx, args.ID, tools.CronUpdateJobRequest{
			Status:            strPtr("active"),
			ExpectedUpdatedAt: expectedUpdatedAt,
		})
		if err != nil {
			return "", fmt.Errorf("cron_resume failed: %w", err)
		}
		return tools.MarshalToolResult(job)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

func CronHistoryTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{Name: "cron_history", Description: descriptions.Load("cron_history"), Action: tools.ActionRead, ParallelSafe: true, Tier: tools.TierDeferred, Tags: []string{"cron", "schedule", "automation"}, Parameters: map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string", "description": "Job ID only; names are not accepted"}, "limit": map[string]any{"type": "integer"}, "offset": map[string]any{"type": "integer"}}, "required": []string{"id"}}}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ID     string `json:"id"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_history args: %w", err)
		}
		if err := requireCronJobID(args.ID); err != nil {
			return "", err
		}
		if args.Limit <= 0 {
			args.Limit = 20
		}
		if args.Limit > 100 {
			args.Limit = 100
		}
		if args.Offset < 0 {
			args.Offset = 0
		}
		execs, err := client.ListExecutions(ctx, args.ID, args.Limit, args.Offset)
		if err != nil {
			return "", fmt.Errorf("cron_history failed: %w", err)
		}
		return tools.MarshalToolResult(map[string]any{"job_id": args.ID, "executions": execs, "count": len(execs)})
	}
	return tools.Tool{Definition: def, Handler: handler}
}

// CronUpdateTool returns a deferred tool for editing an existing cron job in place.
// Omitted fields are preserved by the pointer-based update request. Status is
// deliberately kept behind the explicit pause/resume tools.
func CronUpdateTool(client tools.CronClient) tools.Tool {
	def := tools.Definition{
		Name:        "cron_update",
		Description: descriptions.Load("cron_update"),
		Action:      tools.ActionExecute,
		Mutating:    true,
		Tier:        tools.TierDeferred,
		Tags:        []string{"cron", "schedule", "automation"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":                  map[string]any{"type": "string", "description": "Job ID only; names are not accepted"},
				"schedule":            map[string]any{"type": "string", "description": "New 5-field UTC cron expression"},
				"command":             map[string]any{"type": "string", "description": "New shell command; encoded as execution_config"},
				"prompt":              map[string]any{"type": "string", "description": "New harness prompt; encoded as execution_config"},
				"execution_config":    map[string]any{"type": "string", "description": "New execution config JSON"},
				"timeout_seconds":     map[string]any{"type": "integer", "minimum": 1, "description": "New positive timeout in seconds"},
				"tags":                map[string]any{"type": "string", "description": "Replacement comma-separated tags"},
				"expected_updated_at": map[string]any{"type": "string", "format": "date-time", "description": "updated_at from cron_get; rejects stale writes"},
			},
			"required": []string{"id", "expected_updated_at"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ID                string  `json:"id"`
			Schedule          *string `json:"schedule"`
			Command           *string `json:"command"`
			Prompt            *string `json:"prompt"`
			ExecConfig        *string `json:"execution_config"`
			TimeoutSec        *int    `json:"timeout_seconds"`
			Tags              *string `json:"tags"`
			ExpectedUpdatedAt *string `json:"expected_updated_at"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cron_update args: %w", err)
		}
		if err := requireCronJobID(args.ID); err != nil {
			return "", err
		}
		expectedUpdatedAt, err := requiredExpectedUpdatedAt(args.ExpectedUpdatedAt)
		if err != nil {
			return "", err
		}
		executionInputs := 0
		for _, present := range []bool{args.Command != nil, args.Prompt != nil, args.ExecConfig != nil} {
			if present {
				executionInputs++
			}
		}
		if executionInputs > 1 {
			return "", fmt.Errorf("provide only one of command, prompt, or execution_config")
		}
		if args.Command != nil {
			encoded, err := json.Marshal(map[string]string{"command": *args.Command})
			if err != nil {
				return "", fmt.Errorf("encode command: %w", err)
			}
			config := string(encoded)
			args.ExecConfig = &config
		}
		if args.Prompt != nil {
			metadata, _ := tools.RunMetadataFromContext(ctx)
			encoded, err := json.Marshal(harnessExecutionConfig(*args.Prompt, metadata))
			if err != nil {
				return "", fmt.Errorf("encode prompt: %w", err)
			}
			config := string(encoded)
			args.ExecConfig = &config
		}
		if args.TimeoutSec != nil && *args.TimeoutSec <= 0 {
			return "", fmt.Errorf("timeout_seconds must be positive")
		}
		if args.Schedule == nil && args.ExecConfig == nil && args.TimeoutSec == nil && args.Tags == nil {
			return "", fmt.Errorf("cron_update needs at least one of schedule, command, prompt, execution_config, timeout_seconds or tags; use cron_pause/cron_resume to change status")
		}

		job, err := client.UpdateJob(ctx, args.ID, tools.CronUpdateJobRequest{
			Schedule:          args.Schedule,
			ExecConfig:        args.ExecConfig,
			TimeoutSec:        args.TimeoutSec,
			Tags:              args.Tags,
			ExpectedUpdatedAt: expectedUpdatedAt,
		})
		if err != nil {
			return "", fmt.Errorf("cron_update failed: %w", err)
		}
		return tools.MarshalToolResult(job)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
