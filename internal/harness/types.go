package harness

import (
	"context"
	"encoding/json"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/systemprompt"
)

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type CompletionRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type CompletionResult struct {
	Content   string           `json:"content"`
	ToolCalls []ToolCall       `json:"tool_calls,omitempty"`
	Usage     *CompletionUsage `json:"usage,omitempty"`
	CostUSD   *float64         `json:"cost_usd,omitempty"`
}

type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}

type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

type Event struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type RunStatus string

const (
	RunStatusQueued         RunStatus = "queued"
	RunStatusRunning        RunStatus = "running"
	RunStatusWaitingForUser RunStatus = "waiting_for_user"
	RunStatusCompleted      RunStatus = "completed"
	RunStatusFailed         RunStatus = "failed"
)

type Run struct {
	ID             string    `json:"id"`
	Prompt         string    `json:"prompt"`
	Model          string    `json:"model"`
	Status         RunStatus `json:"status"`
	Output         string    `json:"output,omitempty"`
	Error          string    `json:"error,omitempty"`
	TenantID       string    `json:"tenant_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	AgentID        string    `json:"agent_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type RunRequest struct {
	Prompt           string            `json:"prompt"`
	Model            string            `json:"model,omitempty"`
	SystemPrompt     string            `json:"system_prompt,omitempty"`
	TenantID         string            `json:"tenant_id,omitempty"`
	ConversationID   string            `json:"conversation_id,omitempty"`
	AgentID          string            `json:"agent_id,omitempty"`
	AgentIntent      string            `json:"agent_intent,omitempty"`
	TaskContext      string            `json:"task_context,omitempty"`
	PromptProfile    string            `json:"prompt_profile,omitempty"`
	PromptExtensions *PromptExtensions `json:"prompt_extensions,omitempty"`
}

type PromptExtensions struct {
	Behaviors []string `json:"behaviors,omitempty"`
	Talents   []string `json:"talents,omitempty"`
	Skills    []string `json:"skills,omitempty"`
	Custom    string   `json:"custom,omitempty"`
}

type RunnerConfig struct {
	DefaultModel        string
	DefaultSystemPrompt string
	DefaultAgentIntent  string
	MaxSteps            int
	AskUserTimeout      time.Duration
	AskUserBroker       htools.AskUserQuestionBroker
	MemoryManager       om.Manager
	PromptEngine        systemprompt.Engine
	PreMessageHooks     []PreMessageHook
	PostMessageHooks    []PostMessageHook
	HookFailureMode     HookFailureMode
	ToolApprovalMode    ToolApprovalMode
	ToolPolicy          ToolPolicy
}

type ToolApprovalMode string

const (
	ToolApprovalModeFullAuto    ToolApprovalMode = "full_auto"
	ToolApprovalModePermissions ToolApprovalMode = "permissions"
)

type ToolPolicyInput struct {
	ToolName  string          `json:"tool_name"`
	Action    string          `json:"action"`
	Path      string          `json:"path,omitempty"`
	Arguments json.RawMessage `json:"arguments"`
	Mutating  bool            `json:"mutating"`
}

type ToolPolicyDecision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

type ToolPolicy interface {
	Allow(ctx context.Context, in ToolPolicyInput) (ToolPolicyDecision, error)
}

type HookAction string

const (
	HookActionContinue HookAction = "continue"
	HookActionBlock    HookAction = "block"
)

type HookFailureMode string

const (
	HookFailureModeFailClosed HookFailureMode = "fail_closed"
	HookFailureModeFailOpen   HookFailureMode = "fail_open"
)

type PreMessageHookInput struct {
	RunID   string
	Step    int
	Request CompletionRequest
}

type PreMessageHookResult struct {
	Action         HookAction
	Reason         string
	MutatedRequest *CompletionRequest
}

type PostMessageHookInput struct {
	RunID     string
	Step      int
	Request   CompletionRequest
	Response  CompletionResult
	ToolCalls []ToolCall
}

type PostMessageHookResult struct {
	Action          HookAction
	Reason          string
	MutatedResponse *CompletionResult
}

type PreMessageHook interface {
	Name() string
	BeforeMessage(ctx context.Context, in PreMessageHookInput) (PreMessageHookResult, error)
}

type PostMessageHook interface {
	Name() string
	AfterMessage(ctx context.Context, in PostMessageHookInput) (PostMessageHookResult, error)
}
