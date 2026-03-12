package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/forensics/redaction"
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
	MessageID        string     `json:"message_id,omitempty"`
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
	IsMeta           bool       `json:"is_meta,omitempty"`
	IsCompactSummary bool       `json:"is_compact_summary,omitempty"`
	// CorrelationID links messages across turns within a conversation.
	CorrelationID string `json:"correlation_id,omitempty"`
	// ConversationID is stable across ContinueRun restarts.
	ConversationID string `json:"conversation_id,omitempty"`
}

type CompletionRequest struct {
	Model           string                `json:"model"`
	Messages        []Message             `json:"messages"`
	Tools           []ToolDefinition      `json:"tools,omitempty"`
	Stream          func(CompletionDelta) `json:"-"`
	// ReasoningEffort controls the thinking budget for reasoning models.
	// For OpenAI o-series, valid values are "low", "medium", "high".
	// Empty means the provider default (field omitted from the API request).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
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
	ProviderName   string          `json:"provider_name,omitempty"`
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
	AllowFallback    bool              `json:"allow_fallback,omitempty"`
	SystemPrompt     string            `json:"system_prompt,omitempty"`
	TenantID         string            `json:"tenant_id,omitempty"`
	ConversationID   string            `json:"conversation_id,omitempty"`
	AgentID          string            `json:"agent_id,omitempty"`
	AgentIntent      string            `json:"agent_intent,omitempty"`
	TaskContext      string            `json:"task_context,omitempty"`
	PromptProfile    string            `json:"prompt_profile,omitempty"`
	PromptExtensions *PromptExtensions `json:"prompt_extensions,omitempty"`
	// MaxSteps caps the number of LLM turns for this run.
	// 0 means use the runner's config default (which may itself be 0 = unlimited).
	// Negative values are rejected at StartRun time.
	MaxSteps int `json:"max_steps,omitempty"`
	// MaxCostUSD is a per-run spending ceiling in US dollars.
	// After each LLM turn, if the cumulative cost (when pricing is available) is
	// >= MaxCostUSD the run is terminated with a run.cost_limit_reached event and
	// completed status (not failed — the run did work, it just hit its budget).
	// 0 means no ceiling (unlimited). Negative values are rejected at StartRun time.
	// The ceiling is only enforced when cost data is available (CostStatusAvailable);
	// unpriced models are never terminated by this check.
	MaxCostUSD float64 `json:"max_cost_usd,omitempty"`
	// ReasoningEffort controls the thinking budget forwarded to the provider.
	// For OpenAI o-series models, valid values are "low", "medium", "high".
	// Empty string means no preference (provider default).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// Permissions configures the two-axis permission model for this run.
	// If nil, DefaultPermissionConfig() is used (unrestricted sandbox, no approval).
	Permissions *PermissionConfig `json:"permissions,omitempty"`
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
	PreToolUseHooks     []PreToolUseHook
	PostToolUseHooks    []PostToolUseHook
	HookFailureMode     HookFailureMode
	ToolApprovalMode    ToolApprovalMode
	ToolPolicy          ToolPolicy
	ProviderRegistry    *catalog.ProviderRegistry `json:"-"`
	ConversationStore   ConversationStore         `json:"-"`
	Logger              Logger                    `json:"-"`
	Activations      *ActivationTracker        `json:"-"` // shared tracker for deferred tools
	SkillConstraints *SkillConstraintTracker   `json:"-"` // shared tracker for skill tool constraints
	// RolloutDir is the root directory for JSONL rollout files. When set, every
	// run's events are recorded to <RolloutDir>/<YYYY-MM-DD>/<run_id>.jsonl.
	// Leave empty to disable rollout recording.
	RolloutDir string
	// RedactionPipeline is an optional PII/secret redaction pipeline. When set,
	// every event payload is filtered through the pipeline before being appended
	// to the run's event list and before being written to JSONL rollouts.
	// A nil pipeline means no redaction is applied.
	RedactionPipeline *redaction.Pipeline `json:"-"`
}

// Logger is a minimal logging interface for the runner.
type Logger interface {
	Error(msg string, keysAndValues ...any)
}

type ToolApprovalMode string

const (
	ToolApprovalModeFullAuto    ToolApprovalMode = "full_auto"
	ToolApprovalModePermissions ToolApprovalMode = "permissions"
	// ToolApprovalModeAll requires policy approval for every tool call, including reads.
	ToolApprovalModeAll ToolApprovalMode = "all"
)

// SandboxScope controls what the agent is allowed to access.
type SandboxScope string

const (
	// SandboxScopeWorkspace: agent can only access within the workspace directory.
	SandboxScopeWorkspace SandboxScope = "workspace"
	// SandboxScopeLocal: agent can access local filesystem and network, no sudo.
	SandboxScopeLocal SandboxScope = "local"
	// SandboxScopeUnrestricted: no filesystem restrictions (existing behavior).
	SandboxScopeUnrestricted SandboxScope = "unrestricted"
)

// ApprovalPolicy controls when the agent must ask for approval.
type ApprovalPolicy string

const (
	// ApprovalPolicyNone: never ask for approval (full auto).
	ApprovalPolicyNone ApprovalPolicy = "none"
	// ApprovalPolicyDestructive: ask before destructive/mutating operations.
	ApprovalPolicyDestructive ApprovalPolicy = "destructive"
	// ApprovalPolicyAll: ask before every tool call.
	ApprovalPolicyAll ApprovalPolicy = "all"
)

// PermissionConfig combines sandbox scope and approval policy.
type PermissionConfig struct {
	Sandbox  SandboxScope   `json:"sandbox"`
	Approval ApprovalPolicy `json:"approval"`
}

// DefaultPermissionConfig returns the default (unrestricted, no approval required).
func DefaultPermissionConfig() PermissionConfig {
	return PermissionConfig{
		Sandbox:  SandboxScopeUnrestricted,
		Approval: ApprovalPolicyNone,
	}
}

// ToLegacy converts PermissionConfig to the legacy approval mode.
// This preserves backward compatibility with existing ToolApprovalMode usage.
func (p PermissionConfig) ToLegacy() ToolApprovalMode {
	switch p.Approval {
	case ApprovalPolicyNone:
		return ToolApprovalModeFullAuto
	case ApprovalPolicyDestructive:
		return ToolApprovalModePermissions
	case ApprovalPolicyAll:
		return ToolApprovalModeAll
	default:
		return ToolApprovalModeFullAuto
	}
}

// ValidatePermissionConfig checks that all fields in PermissionConfig are valid.
func ValidatePermissionConfig(p PermissionConfig) error {
	switch p.Sandbox {
	case SandboxScopeWorkspace, SandboxScopeLocal, SandboxScopeUnrestricted:
		// valid
	case "":
		// empty defaults to unrestricted — also valid at validation time
	default:
		return fmt.Errorf("invalid sandbox scope %q: must be one of workspace, local, unrestricted", p.Sandbox)
	}
	switch p.Approval {
	case ApprovalPolicyNone, ApprovalPolicyDestructive, ApprovalPolicyAll:
		// valid
	case "":
		// empty defaults to none — also valid at validation time
	default:
		return fmt.Errorf("invalid approval policy %q: must be one of none, destructive, all", p.Approval)
	}
	return nil
}

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

// ToolHookDecision controls whether a tool call proceeds.
type ToolHookDecision int

const (
	// ToolHookAllow permits tool execution (zero value = allow by default).
	ToolHookAllow ToolHookDecision = iota
	// ToolHookDeny blocks tool execution and returns an error result to the LLM.
	ToolHookDeny
)

// PreToolUseEvent is passed to PreToolUseHooks before a tool executes.
type PreToolUseEvent struct {
	// ToolName is the name of the tool about to execute.
	ToolName string
	// CallID is the tool_call_id from the LLM response.
	CallID string
	// Args is the raw JSON arguments as provided by the LLM (possibly
	// modified by an earlier hook in the chain).
	Args json.RawMessage
	// RunID is the active run identifier.
	RunID string
}

// PreToolUseResult is returned from PreToolUseHooks.
type PreToolUseResult struct {
	// Decision controls whether the tool is allowed to execute.
	// A zero value (ToolHookAllow) permits execution.
	Decision ToolHookDecision
	// Reason is a human-readable explanation used when Decision is Deny
	// or when emitting hook events.
	Reason string
	// ModifiedArgs replaces the LLM-provided args passed to the tool handler.
	// If nil, the previous args (original or from a prior hook) are used.
	ModifiedArgs json.RawMessage
}

// PostToolUseEvent is passed to PostToolUseHooks after a tool executes.
type PostToolUseEvent struct {
	// ToolName is the name of the tool that executed.
	ToolName string
	// CallID is the tool_call_id from the LLM response.
	CallID string
	// Args is the raw JSON arguments that were passed to the tool handler
	// (after any pre-tool-use hook modifications).
	Args json.RawMessage
	// Result is the output string returned by the tool handler.
	// Empty when Error is non-nil.
	Result string
	// Duration is the wall-clock time the tool handler took to execute.
	Duration time.Duration
	// Error is non-nil if the tool handler returned an error.
	Error error
	// RunID is the active run identifier.
	RunID string
}

// PostToolUseResult is returned from PostToolUseHooks.
type PostToolUseResult struct {
	// ModifiedResult replaces the tool output passed to the LLM.
	// If empty, the original tool result is used unchanged.
	ModifiedResult string
}

// PreToolUseHook intercepts tool calls before execution.
type PreToolUseHook interface {
	Name() string
	// PreToolUse is called before the tool handler executes.
	// Return nil result (with nil error) to allow with no modification.
	PreToolUse(ctx context.Context, ev PreToolUseEvent) (*PreToolUseResult, error)
}

// PostToolUseHook intercepts tool calls after execution.
type PostToolUseHook interface {
	Name() string
	// PostToolUse is called after the tool handler executes (even on error).
	PostToolUse(ctx context.Context, ev PostToolUseEvent) (*PostToolUseResult, error)
}
