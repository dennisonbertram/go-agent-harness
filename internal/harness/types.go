package harness

import (
	"context"
	"encoding/json"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
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
	Model    string                `json:"model"`
	Messages []Message             `json:"messages"`
	Tools    []ToolDefinition      `json:"tools,omitempty"`
	Stream   func(CompletionDelta) `json:"-"`
}

type CompletionResult struct {
	Content     string            `json:"content"`
	ToolCalls   []ToolCall        `json:"tool_calls,omitempty"`
	Deltas      []CompletionDelta `json:"-"`
	Usage       *CompletionUsage  `json:"usage,omitempty"`
	CostUSD     *float64          `json:"cost_usd,omitempty"`
	Cost        *CompletionCost   `json:"cost,omitempty"`
	UsageStatus UsageStatus       `json:"usage_status,omitempty"`
	CostStatus  CostStatus        `json:"cost_status,omitempty"`
}

type CompletionDelta struct {
	Content   string        `json:"content,omitempty"`
	Reasoning string        `json:"reasoning,omitempty"`
	ToolCall  ToolCallDelta `json:"tool_call,omitempty"`
}

type ToolCallDelta struct {
	Index     int    `json:"index"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type CompletionUsage struct {
	PromptTokens       int  `json:"prompt_tokens"`
	CompletionTokens   int  `json:"completion_tokens"`
	TotalTokens        int  `json:"total_tokens"`
	CachedPromptTokens *int `json:"cached_prompt_tokens,omitempty"`
	ReasoningTokens    *int `json:"reasoning_tokens,omitempty"`
	InputAudioTokens   *int `json:"input_audio_tokens,omitempty"`
	OutputAudioTokens  *int `json:"output_audio_tokens,omitempty"`
}

type CompletionCost struct {
	InputUSD       float64 `json:"input_usd"`
	OutputUSD      float64 `json:"output_usd"`
	CacheReadUSD   float64 `json:"cache_read_usd"`
	CacheWriteUSD  float64 `json:"cache_write_usd"`
	TotalUSD       float64 `json:"total_usd"`
	PricingVersion string  `json:"pricing_version,omitempty"`
	Estimated      bool    `json:"estimated"`
}

type UsageStatus string

const (
	UsageStatusProviderReported   UsageStatus = "provider_reported"
	UsageStatusProviderUnreported UsageStatus = "provider_unreported"
)

type CostStatus string

const (
	CostStatusAvailable          CostStatus = "available"
	CostStatusUnpricedModel      CostStatus = "unpriced_model"
	CostStatusProviderUnreported CostStatus = "provider_unreported"
	CostStatusPending            CostStatus = "pending"
)

type RunUsageTotals struct {
	PromptTokensTotal     int `json:"prompt_tokens_total"`
	CompletionTokensTotal int `json:"completion_tokens_total"`
	TotalTokens           int `json:"total_tokens"`
	LastTurnTokens        int `json:"last_turn_tokens"`
}

type RunCostTotals struct {
	CostUSDTotal    float64    `json:"cost_usd_total"`
	LastTurnCostUSD float64    `json:"last_turn_cost_usd"`
	CostStatus      CostStatus `json:"cost_status"`
	PricingVersion  string     `json:"pricing_version,omitempty"`
}

// RunSummary contains post-run telemetry for benchmarking and analysis.
type RunSummary struct {
	RunID                 string            `json:"run_id"`
	Status                RunStatus         `json:"status"`
	StepsTaken            int               `json:"steps_taken"`
	TotalPromptTokens     int               `json:"total_prompt_tokens"`
	TotalCompletionTokens int               `json:"total_completion_tokens"`
	TotalCostUSD          float64           `json:"total_cost_usd"`
	CostStatus            CostStatus        `json:"cost_status"`
	ToolCalls             []ToolCallSummary `json:"tool_calls"`
	CacheHitRate          float64           `json:"cache_hit_rate"`
	Error                 string            `json:"error,omitempty"`
}

// ToolCallSummary records a single tool invocation within a run.
type ToolCallSummary struct {
	ToolName string `json:"tool_name"`
	Step     int    `json:"step"`
}

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}

type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

type Event struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	Type      EventType      `json:"type"`
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
	ID             string          `json:"id"`
	Prompt         string          `json:"prompt"`
	Model          string          `json:"model"`
	Status         RunStatus       `json:"status"`
	Output         string          `json:"output,omitempty"`
	Error          string          `json:"error,omitempty"`
	UsageTotals    *RunUsageTotals `json:"usage_totals,omitempty"`
	CostTotals     *RunCostTotals  `json:"cost_totals,omitempty"`
	TenantID       string          `json:"tenant_id,omitempty"`
	ConversationID string          `json:"conversation_id,omitempty"`
	AgentID        string          `json:"agent_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
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
	ProviderRegistry    *catalog.ProviderRegistry `json:"-"`
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
