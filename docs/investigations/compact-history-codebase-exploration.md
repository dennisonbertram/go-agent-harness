# Compact History & Context Status — Codebase Exploration

**Date**: 2026-03-10
**Purpose**: Document all relevant files for implementing `compact_history` and `context_status` tools.

---

## Table of Contents

1. [internal/harness/tools/types.go](#1-internalharnesstools-typesgo)
2. [internal/harness/tools/catalog.go](#2-internalharnesstools-cataloggo)
3. [internal/harness/runner.go (key sections)](#3-internalharnessrunnergo)
4. [internal/harness/events.go](#4-internalharnesseventsgo)
5. [internal/systemprompt/types.go](#5-internalsystemprompt-typesgo)
6. [internal/systemprompt/runtime_context.go](#6-internalsystemprompt-runtime_contextgo)
7. [internal/observationalmemory/token_estimator.go](#7-internalobservationalmemory-token_estimatorgo)
8. [internal/harness/tools/descriptions/embed.go](#8-internalharnesstools-descriptions-embedgo)
9. [internal/harness/tools/read.go (example tool)](#9-internalharnesstools-readgo)
10. [internal/harness/types.go (Message type with IsCompactSummary)](#10-internalharness-typesgo)
11. [internal/harness/conversation_store.go (ConversationStore interface)](#11-internalharness-conversation_storego)
12. [Key Observations](#12-key-observations)

---

## 1. internal/harness/tools/types.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/types.go`

```go
package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
)

type Action string

const (
	ActionRead     Action = "read"
	ActionWrite    Action = "write"
	ActionList     Action = "list"
	ActionExecute  Action = "execute"
	ActionFetch    Action = "fetch"
	ActionDownload Action = "download"
)

type ApprovalMode string

const (
	ApprovalModeFullAuto    ApprovalMode = "full_auto"
	ApprovalModePermissions ApprovalMode = "permissions"
)

type PolicyInput struct {
	ToolName  string          `json:"tool_name"`
	Action    Action          `json:"action"`
	Path      string          `json:"path,omitempty"`
	Arguments json.RawMessage `json:"arguments"`
	Mutating  bool            `json:"mutating"`
}

type PolicyDecision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

type Policy interface {
	Allow(ctx context.Context, in PolicyInput) (PolicyDecision, error)
}

// ToolTier classifies a tool as core (always visible) or deferred (hidden until activated).
type ToolTier string

const (
	// TierCore tools are always sent to the LLM.
	TierCore ToolTier = "core"
	// TierDeferred tools are hidden until activated via find_tool.
	TierDeferred ToolTier = "deferred"
)

type Definition struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Parameters   map[string]any `json:"parameters"`
	Action       Action         `json:"-"`
	Mutating     bool           `json:"-"`
	ParallelSafe bool           `json:"-"`
	Tags         []string       `json:"-"` // search tags for tool discovery
	Tier         ToolTier       `json:"-"` // core or deferred
}

// ActivationTrackerInterface tracks which deferred tools have been activated per run.
type ActivationTrackerInterface interface {
	Activate(runID string, toolNames ...string)
	IsActive(runID string, toolName string) bool
}

type Handler func(ctx context.Context, args json.RawMessage) (string, error)

type Tool struct {
	Definition Definition
	Handler    Handler
}

// PromptExtensionDirs holds the resolved absolute paths to the prompt extension directories.
type PromptExtensionDirs struct {
	BehaviorsDir string
	TalentsDir   string
}

type BuildOptions struct {
	WorkspaceRoot  string
	ApprovalMode   ApprovalMode
	Policy         Policy
	HTTPClient     *http.Client
	Now            func() time.Time
	AskUserBroker  AskUserQuestionBroker
	AskUserTimeout time.Duration
	MemoryManager  om.Manager

	MCPRegistry         MCPRegistry
	AgentRunner         AgentRunner
	WebFetcher          WebFetcher
	Sourcegraph         SourcegraphConfig
	EnableTodos         bool
	EnableLSP           bool
	EnableMCP           bool
	EnableAgent         bool
	EnableWebOps        bool
	EnableSkills        bool
	SkillLister         SkillLister
	SkillVerifier       SkillVerifier
	ModelCatalog        *catalog.Catalog
	EnableCron          bool
	CronClient          CronClient
	CallbackManager     *CallbackManager
	EnableCallbacks     bool
	EnableRecipes       bool
	RecipesDir          string
	ConversationStore   ConversationReader
	EnableConversations bool

	// PromptExtensionDirs provides the extension directories for the create_prompt_extension tool.
	// If empty, that tool returns an error indicating it is not configured.
	PromptExtensionDirs PromptExtensionDirs
}

// ConversationSummary holds lightweight metadata about a conversation.
type ConversationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	MsgCount  int    `json:"message_count"`
}

// ConversationSearchResult is a single result from a full-text search over conversations.
type ConversationSearchResult struct {
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role"`
	Snippet        string `json:"snippet"`
}

// ConversationReader provides read-only access to conversation history.
// Implementations must be safe for concurrent use.
type ConversationReader interface {
	ListConversations(ctx context.Context, limit, offset int) ([]ConversationSummary, error)
	SearchConversations(ctx context.Context, query string, limit int) ([]ConversationSearchResult, error)
}

type SourcegraphConfig struct {
	Endpoint string
	Token    string
}

type MCPResource struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

type MCPToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type MCPRegistry interface {
	ListResources(ctx context.Context, server string) ([]MCPResource, error)
	ReadResource(ctx context.Context, server, uri string) (string, error)
	ListTools(ctx context.Context) (map[string][]MCPToolDefinition, error)
	CallTool(ctx context.Context, server, tool string, args json.RawMessage) (string, error)
}

type AgentRunner interface {
	RunPrompt(ctx context.Context, prompt string) (string, error)
}

// ForkConfig holds configuration for a forked skill execution.
type ForkConfig struct {
	Prompt       string            // the interpolated skill body
	SkillName    string            // name of the skill being forked
	Agent        string            // agent type hint (e.g., "Explore")
	AllowedTools []string          // tool restrictions for the subagent
	Metadata     map[string]string // arbitrary metadata (parent run ID, etc.)
}

// ForkResult holds the output from a forked skill execution.
type ForkResult struct {
	Output  string // the subagent's final output
	Summary string // optional summarized output
	Error   string // error message if the subagent failed
}

// ForkedAgentRunner extends AgentRunner with support for forked skill execution.
// Implementations that only support basic RunPrompt need not implement this.
type ForkedAgentRunner interface {
	AgentRunner
	RunForkedSkill(ctx context.Context, config ForkConfig) (ForkResult, error)
}

// SkillInfo holds read-only skill metadata for the tool layer.
type SkillInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ArgumentHint string   `json:"argument_hint,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Source       string   `json:"source"`
	Context      string   `json:"context,omitempty"`
	Agent        string   `json:"agent,omitempty"`
	Verified     bool     `json:"verified,omitempty"`
	VerifiedAt   string   `json:"verified_at,omitempty"`
	VerifiedBy   string   `json:"verified_by,omitempty"`
	FilePath     string   `json:"file_path,omitempty"` // needed for verify action
}

// SkillLister provides skill lookup and listing for the skill tool.
type SkillLister interface {
	GetSkill(name string) (SkillInfo, bool)
	ListSkills() []SkillInfo
	ResolveSkill(ctx context.Context, name, args, workspace string) (string, error)
}

// SkillVerifier extends SkillLister with verification support.
type SkillVerifier interface {
	SkillLister
	GetSkillFilePath(name string) (string, bool)
	UpdateSkillVerification(ctx context.Context, name string, verified bool, verifiedAt time.Time, verifiedBy string) error
}

type WebFetcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]map[string]any, error)
	Fetch(ctx context.Context, url string) (string, error)
}

type AskUserQuestionRequest struct {
	RunID     string
	CallID    string
	Questions []AskUserQuestion
	Timeout   time.Duration
}

type AskUserQuestionPending struct {
	RunID      string            `json:"run_id"`
	CallID     string            `json:"call_id"`
	Tool       string            `json:"tool"`
	Questions  []AskUserQuestion `json:"questions"`
	DeadlineAt time.Time         `json:"deadline_at"`
}

type AskUserQuestionBroker interface {
	Ask(ctx context.Context, req AskUserQuestionRequest) (answers map[string]string, answeredAt time.Time, err error)
	Pending(runID string) (AskUserQuestionPending, bool)
	Submit(runID string, answers map[string]string) error
}

type contextKey string

const ContextKeyRunID contextKey = "run_id"
const ContextKeyToolCallID contextKey = "tool_call_id"
const ContextKeyForkedSkill contextKey = "forked_skill"
const ContextKeyRunMetadata contextKey = "run_metadata"
const ContextKeyTranscriptReader contextKey = "transcript_reader"
const ContextKeyOutputStreamer contextKey = "output_streamer"

type RunMetadata struct {
	RunID          string
	TenantID       string
	ConversationID string
	AgentID        string
}

type TranscriptMessage struct {
	Index      int64  `json:"index"`
	Role       string `json:"role"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Content    string `json:"content,omitempty"`
}

type TranscriptSnapshot struct {
	RunID          string              `json:"run_id"`
	TenantID       string              `json:"tenant_id"`
	ConversationID string              `json:"conversation_id"`
	AgentID        string              `json:"agent_id"`
	Messages       []TranscriptMessage `json:"messages"`
	GeneratedAt    time.Time           `json:"generated_at"`
}

type TranscriptReader interface {
	Snapshot(limit int, includeTools bool) TranscriptSnapshot
}

func RunIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if meta, ok := ctx.Value(ContextKeyRunMetadata).(RunMetadata); ok {
		return meta.RunID
	}
	v, _ := ctx.Value(ContextKeyRunID).(string)
	return v
}

func ToolCallIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ContextKeyToolCallID).(string)
	return v
}

func RunMetadataFromContext(ctx context.Context) (RunMetadata, bool) {
	if ctx == nil {
		return RunMetadata{}, false
	}
	v, ok := ctx.Value(ContextKeyRunMetadata).(RunMetadata)
	return v, ok
}

func TranscriptReaderFromContext(ctx context.Context) (TranscriptReader, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(ContextKeyTranscriptReader).(TranscriptReader)
	return v, ok
}

// OutputStreamerFromContext retrieves the output streamer function from the context.
func OutputStreamerFromContext(ctx context.Context) (func(chunk string), bool) {
	if ctx == nil {
		return nil, false
	}
	fn, ok := ctx.Value(ContextKeyOutputStreamer).(func(chunk string))
	return fn, ok
}

// CronClient provides access to the cron scheduler daemon.
type CronClient interface {
	CreateJob(ctx context.Context, req CronCreateJobRequest) (CronJob, error)
	ListJobs(ctx context.Context) ([]CronJob, error)
	GetJob(ctx context.Context, id string) (CronJob, error)
	UpdateJob(ctx context.Context, id string, req CronUpdateJobRequest) (CronJob, error)
	DeleteJob(ctx context.Context, id string) error
	ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]CronExecution, error)
	Health(ctx context.Context) error
}

// CronJob represents a scheduled cron job.
type CronJob struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"`
	ExecType   string    `json:"execution_type"`
	ExecConfig string    `json:"execution_config"`
	Status     string    `json:"status"`
	TimeoutSec int       `json:"timeout_seconds"`
	Tags       string    `json:"tags"`
	NextRunAt  time.Time `json:"next_run_at"`
	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CronExecution represents a single execution of a cron job.
type CronExecution struct {
	ID            string    `json:"id"`
	JobID         string    `json:"job_id"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
	Status        string    `json:"status"`
	RunID         string    `json:"run_id,omitempty"`
	OutputSummary string    `json:"output_summary,omitempty"`
	Error         string    `json:"error,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
}

// CronCreateJobRequest is the request for creating a cron job.
type CronCreateJobRequest struct {
	Name       string `json:"name"`
	Schedule   string `json:"schedule"`
	ExecType   string `json:"execution_type"`
	ExecConfig string `json:"execution_config"`
	TimeoutSec int    `json:"timeout_seconds,omitempty"`
	Tags       string `json:"tags,omitempty"`
}

// CronUpdateJobRequest is the request for updating a cron job.
type CronUpdateJobRequest struct {
	Schedule   *string `json:"schedule,omitempty"`
	ExecConfig *string `json:"execution_config,omitempty"`
	Status     *string `json:"status,omitempty"`
	TimeoutSec *int    `json:"timeout_seconds,omitempty"`
	Tags       *string `json:"tags,omitempty"`
}
```

### Key Observations for types.go

- **Context keys**: `ContextKeyRunID`, `ContextKeyToolCallID`, `ContextKeyForkedSkill`, `ContextKeyRunMetadata`, `ContextKeyTranscriptReader`, `ContextKeyOutputStreamer` — all of type `contextKey` (unexported string type).
- **`TranscriptReaderFromContext`**: Retrieves a `TranscriptReader` from ctx. This is the mechanism tools use to read the current run's message transcript.
- **`TranscriptReader` interface**: Single method `Snapshot(limit int, includeTools bool) TranscriptSnapshot`. Returns up to `limit` messages. The `includeTools` flag controls whether tool-role messages are included.
- **`TranscriptSnapshot`**: Contains `RunID`, `TenantID`, `ConversationID`, `AgentID`, `Messages`, `GeneratedAt`. This is the data a `context_status` tool would use.
- **`RunMetadataFromContext`**: Returns `RunMetadata` with `RunID`, `TenantID`, `ConversationID`, `AgentID`.
- **`BuildOptions`**: The struct used to configure tool catalog building. A new tool would need to add any new dependencies here (e.g., a `CompactionRequester` or similar).

---

## 2. internal/harness/tools/catalog.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/catalog.go`

```go
package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"go-agent-harness/internal/harness/tools/recipe"
)

func BuildCatalog(opts BuildOptions) ([]Tool, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.ApprovalMode == "" {
		opts.ApprovalMode = ApprovalModeFullAuto
	}
	if opts.AskUserTimeout <= 0 {
		opts.AskUserTimeout = 5 * time.Minute
	}

	jobManager := NewJobManager(opts.WorkspaceRoot, opts.Now)
	todos := newTodoStore()

	tools := []Tool{
		askUserQuestionTool(opts.AskUserBroker, opts.AskUserTimeout),
		observationalMemoryTool(opts.WorkspaceRoot, opts.MemoryManager, opts.AgentRunner),
		readTool(opts.WorkspaceRoot),
		writeTool(opts.WorkspaceRoot),
		editTool(opts.WorkspaceRoot),
		bashTool(jobManager),
		jobOutputTool(jobManager),
		jobKillTool(jobManager),
		lsTool(opts.WorkspaceRoot),
		globTool(opts.WorkspaceRoot),
		grepTool(opts.WorkspaceRoot),
		applyPatchTool(opts.WorkspaceRoot),
		gitStatusTool(opts.WorkspaceRoot),
		gitDiffTool(opts.WorkspaceRoot),
		fetchTool(opts.HTTPClient),
		downloadTool(opts.WorkspaceRoot, opts.HTTPClient),
	}

	if opts.EnableTodos {
		tools = append(tools, todosTool(todos))
	}
	if opts.EnableLSP {
		tools = append(tools, lspDiagnosticsTool(opts.WorkspaceRoot), lspReferencesTool(opts.WorkspaceRoot), lspRestartTool(opts.WorkspaceRoot))
	}
	if opts.Sourcegraph.Endpoint != "" {
		tools = append(tools, sourcegraphTool(opts.HTTPClient, opts.Sourcegraph))
	}
	if opts.EnableMCP && opts.MCPRegistry != nil {
		tools = append(tools, listMCPResourcesTool(opts.MCPRegistry), readMCPResourceTool(opts.MCPRegistry))
		dynamic, err := dynamicMCPTools(context.Background(), opts.MCPRegistry)
		if err != nil {
			return nil, err
		}
		tools = append(tools, dynamic...)
	}
	if opts.ModelCatalog != nil {
		tools = append(tools, listModelsTool(opts.ModelCatalog))
	}
	if opts.EnableSkills && opts.SkillLister != nil {
		tools = append(tools, skillTool(opts.SkillLister, opts.AgentRunner))
	}
	if opts.EnableSkills && opts.SkillVerifier != nil {
		tools = append(tools, verifySkillTool(opts.SkillVerifier))
	}
	if opts.EnableAgent && opts.AgentRunner != nil {
		tools = append(tools, agentTool(opts.AgentRunner))
		if opts.EnableWebOps && opts.WebFetcher != nil {
			tools = append(tools, agenticFetchTool(opts.WebFetcher, opts.AgentRunner), webSearchTool(opts.WebFetcher), webFetchTool(opts.WebFetcher))
		}
	}
	if opts.EnableCron && opts.CronClient != nil {
		tools = append(tools,
			cronCreateTool(opts.CronClient),
			cronListTool(opts.CronClient),
			cronGetTool(opts.CronClient),
			cronDeleteTool(opts.CronClient),
			cronPauseTool(opts.CronClient),
			cronResumeTool(opts.CronClient),
		)
	}

	if opts.EnableCallbacks && opts.CallbackManager != nil {
		tools = append(tools,
			setDelayedCallbackTool(opts.CallbackManager),
			cancelDelayedCallbackTool(opts.CallbackManager),
			listDelayedCallbacksTool(opts.CallbackManager),
		)
	}

	if opts.EnableRecipes {
		recipes, err := recipe.LoadRecipes(opts.RecipesDir)
		if err != nil {
			return nil, err
		}
		if len(recipes) > 0 {
			handlers := buildHandlerMap(tools)
			tools = append(tools, runRecipeTool(handlers, recipes))
		}
	}

	for i := range tools {
		tools[i].Handler = applyPolicy(tools[i].Definition, opts.ApprovalMode, opts.Policy, tools[i].Handler)
	}

	sort.SliceStable(tools, func(i, j int) bool {
		return tools[i].Definition.Name < tools[j].Definition.Name
	})
	return tools, nil
}

// buildHandlerMap constructs a handler map from a slice of tools.
func buildHandlerMap(tools []Tool) recipe.HandlerMap {
	m := make(recipe.HandlerMap, len(tools))
	for _, t := range tools {
		m[t.Definition.Name] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return t.Handler(ctx, args)
		}
	}
	return m
}
```

### Key Observations for catalog.go

- **Registration pattern**: Tools are instantiated by calling a constructor function (e.g., `readTool(workspaceRoot)`) and appended to the `tools` slice.
- **Feature gating**: Optional tools are guarded by `Enable*` booleans and non-nil dependency checks.
- **Policy wrapping**: After all tools are added, every handler is wrapped with `applyPolicy`.
- **Sort**: Tools are sorted alphabetically by name before return.
- **To add a new tool**: Create a constructor function (e.g., `contextStatusTool(...)` or `compactHistoryTool(...)`), append it to the slice, and add any new dependencies to `BuildOptions`.

---

## 3. internal/harness/runner.go (key sections)

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/runner.go`

### 3a. Runner struct and constructor (lines 1-119)

```go
package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/rollout"
	"go-agent-harness/internal/systemprompt"
)

type runState struct {
	run                Run
	staticSystemPrompt string
	promptResolved     *systemprompt.ResolvedPrompt
	usageTotals        usageTotalsAccumulator
	costTotals         RunCostTotals
	messages           []Message
	events             []Event
	subscribers        map[chan Event]struct{}
	nextEventSeq       uint64
	steeringCh         chan string // buffered channel for user steering messages
	maxCostUSD         float64
	recorder           *rollout.Recorder
}

type usageTotalsAccumulator struct {
	promptTokensTotal     int
	completionTokensTotal int
	totalTokens           int
	lastTurnTokens        int
	cachedPromptTokens    int
	hasCachedPromptTokens bool
	reasoningTokens       int
	hasReasoningTokens    bool
	inputAudioTokens      int
	hasInputAudioTokens   bool
	outputAudioTokens     int
	hasOutputAudioTokens  bool
}

var (
	ErrRunNotFound        = errors.New("run not found")
	ErrNoPendingInput     = errors.New("no pending input")
	ErrInvalidRunInput    = errors.New("invalid run input")
	ErrRunNotCompleted    = errors.New("run is not completed")
	ErrRunNotActive       = errors.New("run is not active")
	ErrSteeringBufferFull = errors.New("steering buffer full")
)

const steeringBufferSize = 10

type Runner struct {
	provider         Provider
	tools            *Registry
	config           RunnerConfig
	providerRegistry *catalog.ProviderRegistry
	activations      *ActivationTracker
	skillConstraints *SkillConstraintTracker

	mu            sync.RWMutex
	runs          map[string]*runState
	conversations map[string][]Message
	idSeq         uint64
}

func NewRunner(provider Provider, tools *Registry, config RunnerConfig) *Runner {
	if config.DefaultModel == "" {
		config.DefaultModel = "gpt-4.1-mini"
	}
	if config.DefaultAgentIntent == "" {
		config.DefaultAgentIntent = "general"
	}
	if config.AskUserTimeout <= 0 {
		config.AskUserTimeout = 5 * time.Minute
	}
	if config.HookFailureMode == "" {
		config.HookFailureMode = HookFailureModeFailClosed
	}
	if tools == nil {
		tools = NewRegistry()
	}
	activations := config.Activations
	if activations == nil {
		activations = NewActivationTracker()
	}
	skillConstraints := config.SkillConstraints
	if skillConstraints == nil {
		skillConstraints = NewSkillConstraintTracker()
	}
	return &Runner{
		provider:         provider,
		tools:            tools,
		config:           config,
		providerRegistry: config.ProviderRegistry,
		activations:      activations,
		skillConstraints: skillConstraints,
		runs:             make(map[string]*runState),
		conversations:    make(map[string][]Message),
	}
}
```

### 3b. execute() method — conversation loading and step loop (lines 577-660)

```go
func (r *Runner) execute(runID string, req RunRequest) {
	r.setStatus(runID, RunStatusRunning, "", "")
	r.emit(runID, EventRunStarted, map[string]any{"prompt": req.Prompt})

	model := req.Model
	if model == "" {
		model = r.config.DefaultModel
	}

	activeProvider, providerName, err := r.resolveProvider(runID, model, req.AllowFallback)
	if err != nil {
		r.failRun(runID, err)
		return
	}

	r.mu.Lock()
	if state, ok := r.runs[runID]; ok {
		state.run.ProviderName = providerName
	}
	r.mu.Unlock()

	r.emit(runID, EventProviderResolved, map[string]any{
		"model":    model,
		"provider": providerName,
	})

	systemPrompt, resolvedPrompt, runStartedAt := r.promptContext(runID)
	if resolvedPrompt != nil {
		r.emit(runID, EventPromptResolved, map[string]any{
			"intent":            resolvedPrompt.ResolvedIntent,
			"model_profile":     resolvedPrompt.ResolvedModelProfile,
			"model_fallback":    resolvedPrompt.ModelFallback,
			"applied_behaviors": append([]string(nil), resolvedPrompt.Behaviors...),
			"applied_talents":   append([]string(nil), resolvedPrompt.Talents...),
			"applied_skills":    append([]string(nil), resolvedPrompt.Skills...),
			"has_warnings":      len(resolvedPrompt.Warnings) > 0,
		})
		for _, warning := range resolvedPrompt.Warnings {
			r.emit(runID, EventPromptWarning, map[string]any{
				"code":    warning.Code,
				"message": warning.Message,
			})
		}
	}

	priorMessages := r.loadConversationHistory(runID)
	messages := make([]Message, 0, len(priorMessages)+16)
	messages = append(messages, priorMessages...)
	messages = append(messages, Message{Role: "user", Content: req.Prompt})

	if len(priorMessages) > 0 {
		r.emit(runID, EventConversationContinued, map[string]any{
			"conversation_id":     r.conversationID(runID),
			"prior_message_count": len(priorMessages),
		})
	}
	r.setMessages(runID, messages)

	effectiveMaxSteps := r.config.MaxSteps
	if req.MaxSteps > 0 {
		effectiveMaxSteps = req.MaxSteps
	}

	for step := 1; effectiveMaxSteps == 0 || step <= effectiveMaxSteps; step++ {
		r.emit(runID, EventRunStepStarted, map[string]any{"step": step})
		r.drainSteering(runID, &messages)

		r.emit(runID, EventLLMTurnRequested, map[string]any{"step": step})

		turnMessages := make([]Message, 0, len(messages)+4)
		if r.config.MemoryManager != nil && r.config.MemoryManager.Mode() != om.ModeOff {
			snippet, _, err := r.config.MemoryManager.Snippet(context.Background(), r.scopeKey(runID))
			if err != nil {
				r.emit(runID, EventMemoryObserveFailed, map[string]any{"step": step, "error": err.Error()})
			} else if strings.TrimSpace(snippet) != "" {
				turnMessages = append(turnMessages, Message{Role: "system", Content: snippet})
			}
		}
		if systemPrompt != "" {
			turnMessages = append(turnMessages, Message{Role: "system", Content: systemPrompt})
		}
		if resolvedPrompt != nil && r.config.PromptEngine != nil {
			usageTotals, costTotals := r.accountingTotals(runID)
			runtimeContext := strings.TrimSpace(r.config.PromptEngine.RuntimeContext(systemprompt.RuntimeContextInput{
				RunStartedAt:          runStartedAt,
				Now:                   time.Now().UTC(),
				Step:                  step,
				PromptTokensTotal:     usageTotals.PromptTokensTotal,
				CompletionTokensTotal: usageTotals.CompletionTokensTotal,
				TotalTokens:           usageTotals.TotalTokens,
				LastTurnTokens:        usageTotals.LastTurnTokens,
				CostUSDTotal:          costTotals.CostUSDTotal,
				LastTurnCostUSD:       costTotals.LastTurnCostUSD,
				CostStatus:            string(costTotals.CostStatus),
			}))
			if runtimeContext != "" {
				turnMessages = append(turnMessages, Message{Role: "system", Content: runtimeContext})
			}
		}
		turnMessages = append(turnMessages, messages...)
		// ... completion request and tool execution follows
```

### 3c. Context key injection into tool execution context (lines 837-851)

```go
			meta := r.runMetadata(runID)
			toolCtx := context.WithValue(context.Background(), htools.ContextKeyRunID, runID)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyToolCallID, call.ID)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyRunMetadata, meta)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyTranscriptReader, runTranscriptReader{runner: r, runID: runID})
			callID := call.ID
			outputStreamer := func(chunk string) {
				r.emit(runID, EventToolOutputDelta, map[string]any{
					"call_id": callID,
					"tool":    call.Name,
					"content": chunk,
				})
			}
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyOutputStreamer, outputStreamer)
			toolStart := time.Now()
			toolOutput, toolErr := r.tools.Execute(toolCtx, call.Name, callArgs)
```

### 3d. Meta-message injection from tool results (lines 855-934)

```go
			// Check for meta-messages in tool output (enriched result envelope)
			var metaMessages []htools.MetaMessage
			if toolErr == nil {
				if tr, ok := htools.UnwrapToolResult(toolOutput); ok {
					toolOutput = tr.Output
					metaMessages = tr.MetaMessages
				}
			}

			// ... hook application, error handling ...

			messages = append(messages, Message{
				Role:       "tool",
				Name:       call.Name,
				ToolCallID: call.ID,
				Content:    toolOutput,
			})

			// Inject meta-messages as system messages after the tool result
			for _, metaMsg := range metaMessages {
				messages = append(messages, Message{
					Role:    "system",
					Content: metaMsg.Content,
					IsMeta:  true,
				})
				r.emit(runID, EventMetaMessageInjected, map[string]any{
					"call_id": call.ID,
					"tool":    call.Name,
					"length":  len(metaMsg.Content),
				})
			}

			r.setMessages(runID, messages)
```

### 3e. SummarizeMessages method (lines 1820-1845)

```go
// SummarizeMessages makes a single LLM call to summarize the given messages.
// Returns a summary string suitable for use as a compact summary.
func (r *Runner) SummarizeMessages(ctx context.Context, messages []Message) (string, error) {
	if r.provider == nil {
		return "", fmt.Errorf("provider not configured")
	}
	model := r.config.DefaultModel
	if model == "" {
		model = "gpt-4.1-mini"
	}
	req := CompletionRequest{
		Model: model,
		Messages: append(append([]Message(nil), messages...), Message{
			Role:    "user",
			Content: "Please provide a concise summary of this conversation so far, suitable for use as context in a continuation. Include key facts, decisions, and outputs. Be concise.",
		}),
	}
	result, err := r.provider.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Content) == "" {
		return "", fmt.Errorf("empty summary from provider")
	}
	return result.Content, nil
}
```

### 3f. runTranscriptReader (lines 1883-1893)

```go
type runTranscriptReader struct {
	runner *Runner
	runID  string
}

func (r runTranscriptReader) Snapshot(limit int, includeTools bool) htools.TranscriptSnapshot {
	if r.runner == nil {
		return htools.TranscriptSnapshot{RunID: r.runID, GeneratedAt: time.Now().UTC()}
	}
	return r.runner.transcriptSnapshot(r.runID, limit, includeTools)
}
```

---

## 4. internal/harness/events.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/events.go`

```go
package harness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EventType represents a typed SSE event name.
type EventType string

// Run lifecycle events.
const (
	EventRunStarted         EventType = "run.started"
	EventRunCompleted       EventType = "run.completed"
	EventRunFailed          EventType = "run.failed"
	EventRunWaitingForUser  EventType = "run.waiting_for_user"
	EventRunResumed         EventType = "run.resumed"
	EventRunCostLimitReached EventType = "run.cost_limit_reached"
	EventRunStepStarted      EventType = "run.step.started"
	EventRunStepCompleted    EventType = "run.step.completed"
)

// LLM turn events.
const (
	EventLLMTurnRequested      EventType = "llm.turn.requested"
	EventLLMTurnCompleted      EventType = "llm.turn.completed"
	EventAssistantMessageDelta  EventType = "assistant.message.delta"
	EventAssistantThinkingDelta EventType = "assistant.thinking.delta"
)

// Tool execution events.
const (
	EventToolCallStarted   EventType = "tool.call.started"
	EventToolCallCompleted EventType = "tool.call.completed"
	EventToolCallDelta     EventType = "tool.call.delta"
	EventToolActivated     EventType = "tool.activated"
	EventToolOutputDelta   EventType = "tool.output.delta"
)

// Assistant completion events.
const (
	EventAssistantMessage EventType = "assistant.message"
)

// Conversation events.
const (
	EventConversationContinued EventType = "conversation.continued"
)

// Prompt events.
const (
	EventPromptResolved EventType = "prompt.resolved"
	EventPromptWarning  EventType = "prompt.warning"
)

// Provider events.
const (
	EventProviderResolved EventType = "provider.resolved"
)

// Memory events.
const (
	EventMemoryObserveStarted      EventType = "memory.observe.started"
	EventMemoryObserveCompleted    EventType = "memory.observe.completed"
	EventMemoryObserveFailed       EventType = "memory.observe.failed"
	EventMemoryReflectionCompleted EventType = "memory.reflection.completed"
)

// Accounting events.
const (
	EventUsageDelta EventType = "usage.delta"
)

// Hook events (message-level: pre/post LLM turn).
const (
	EventHookStarted   EventType = "hook.started"
	EventHookFailed    EventType = "hook.failed"
	EventHookCompleted EventType = "hook.completed"
)

// Tool hook events (tool-level: pre/post individual tool execution).
const (
	EventToolHookStarted   EventType = "tool_hook.started"
	EventToolHookFailed    EventType = "tool_hook.failed"
	EventToolHookCompleted EventType = "tool_hook.completed"
)

// Callback events.
const (
	EventCallbackScheduled EventType = "callback.scheduled"
	EventCallbackFired     EventType = "callback.fired"
	EventCallbackCanceled  EventType = "callback.canceled"
)

// Skill constraint events.
const (
	EventSkillConstraintActivated   EventType = "skill.constraint.activated"
	EventSkillConstraintDeactivated EventType = "skill.constraint.deactivated"
	EventToolCallBlocked            EventType = "tool.call.blocked"
)

// Meta-message events.
const (
	EventMetaMessageInjected EventType = "meta.message.injected"
)

// Steering events.
const (
	EventSteeringReceived EventType = "steering.received"
)

// Skill fork events.
const (
	EventSkillForkStarted   EventType = "skill.fork.started"
	EventSkillForkCompleted EventType = "skill.fork.completed"
	EventSkillForkFailed    EventType = "skill.fork.failed"
)

// AllEventTypes returns all known event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventRunStarted, EventRunCompleted, EventRunFailed,
		EventRunWaitingForUser, EventRunResumed, EventRunCostLimitReached,
		EventRunStepStarted, EventRunStepCompleted,
		EventLLMTurnRequested, EventLLMTurnCompleted,
		EventAssistantMessageDelta, EventAssistantThinkingDelta,
		EventToolCallStarted, EventToolCallCompleted, EventToolCallDelta,
		EventToolActivated, EventToolOutputDelta,
		EventAssistantMessage,
		EventConversationContinued,
		EventPromptResolved, EventPromptWarning,
		EventProviderResolved,
		EventMemoryObserveStarted, EventMemoryObserveCompleted,
		EventMemoryObserveFailed, EventMemoryReflectionCompleted,
		EventUsageDelta,
		EventHookStarted, EventHookFailed, EventHookCompleted,
		EventCallbackScheduled, EventCallbackFired, EventCallbackCanceled,
		EventSkillConstraintActivated, EventSkillConstraintDeactivated,
		EventToolCallBlocked,
		EventMetaMessageInjected,
		EventSkillForkStarted, EventSkillForkCompleted, EventSkillForkFailed,
		EventToolHookStarted, EventToolHookFailed, EventToolHookCompleted,
		EventSteeringReceived,
	}
}

// IsTerminalEvent reports whether the given event type signals the end of a run.
func IsTerminalEvent(et EventType) bool {
	return et == EventRunCompleted || et == EventRunFailed
}

// RunCompletedPayload is the typed payload for EventRunCompleted.
type RunCompletedPayload struct {
	Output      string         `json:"output"`
	UsageTotals map[string]any `json:"usage_totals,omitempty"`
	CostTotals  map[string]any `json:"cost_totals,omitempty"`
}

func (p RunCompletedPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

func ParseRunCompletedPayload(payload map[string]any) (RunCompletedPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return RunCompletedPayload{}, err
	}
	var p RunCompletedPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// RunFailedPayload is the typed payload for EventRunFailed.
type RunFailedPayload struct {
	Error       string         `json:"error"`
	UsageTotals map[string]any `json:"usage_totals,omitempty"`
	CostTotals  map[string]any `json:"cost_totals,omitempty"`
}

func (p RunFailedPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

func ParseRunFailedPayload(payload map[string]any) (RunFailedPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return RunFailedPayload{}, err
	}
	var p RunFailedPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// UsageDeltaPayload is the typed payload for EventUsageDelta.
type UsageDeltaPayload struct {
	Step              int            `json:"step"`
	UsageStatus       string         `json:"usage_status"`
	CostStatus        string         `json:"cost_status"`
	TurnUsage         map[string]any `json:"turn_usage,omitempty"`
	TurnCostUSD       float64        `json:"turn_cost_usd"`
	CumulativeUsage   map[string]any `json:"cumulative_usage,omitempty"`
	CumulativeCostUSD float64        `json:"cumulative_cost_usd"`
	PricingVersion    string         `json:"pricing_version,omitempty"`
}

func (p UsageDeltaPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

func ParseUsageDeltaPayload(payload map[string]any) (UsageDeltaPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return UsageDeltaPayload{}, err
	}
	var p UsageDeltaPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// ParseEventID parses a per-run event ID of the form "runID:seq".
func ParseEventID(id string) (runID string, seq uint64, err error) {
	idx := strings.LastIndex(id, ":")
	if idx < 0 || idx == len(id)-1 {
		return "", 0, fmt.Errorf("invalid event ID %q: missing colon separator", id)
	}
	runID = id[:idx]
	if runID == "" {
		return "", 0, fmt.Errorf("invalid event ID %q: empty run ID", id)
	}
	seq, err = strconv.ParseUint(id[idx+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid event ID %q: %w", id, err)
	}
	return runID, seq, nil
}
```

### Key Observations for events.go

- New tools might want to emit new event types (e.g., `EventCompactionStarted`, `EventCompactionCompleted`).
- Events must be added to `AllEventTypes()` to be discoverable.

---

## 5. internal/systemprompt/types.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/systemprompt/types.go`

```go
package systemprompt

import (
	"context"
	"time"
)

type Extensions struct {
	Behaviors []string
	Talents   []string
	Skills    []string
	Custom    string
}

type ResolveRequest struct {
	Model              string
	AgentIntent        string
	DefaultAgentIntent string
	PromptProfile      string
	TaskContext        string
	Extensions         Extensions
}

type RuntimeContextInput struct {
	RunStartedAt          time.Time
	Now                   time.Time
	Step                  int
	PromptTokensTotal     int
	CompletionTokensTotal int
	TotalTokens           int
	LastTurnTokens        int
	CostUSDTotal          float64
	LastTurnCostUSD       float64
	CostStatus            string
}

type Warning struct {
	Code    string
	Message string
}

// SkillResolver resolves skill names into interpolated prompt content.
type SkillResolver interface {
	ResolveSkill(ctx context.Context, name, args, workspace string) (string, error)
}

type ResolvedPrompt struct {
	StaticPrompt         string
	ResolvedIntent       string
	ResolvedModelProfile string
	ModelFallback        bool
	Behaviors            []string
	Talents              []string
	Skills               []string
	Warnings             []Warning
}

type Engine interface {
	Resolve(req ResolveRequest) (ResolvedPrompt, error)
	RuntimeContext(in RuntimeContextInput) string
}
```

### Key Observations for types.go (systemprompt)

- **`RuntimeContextInput`**: Currently includes step count, token totals, cost data. A `context_status` tool could either use this directly or build its own data from `TranscriptReader` + `usageTotalsAccumulator`.
- The runtime context is injected as a system message before each LLM turn (see runner.go section 3b).

---

## 6. internal/systemprompt/runtime_context.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/systemprompt/runtime_context.go`

```go
package systemprompt

import (
	"fmt"
	"strings"
	"time"
)

func (e *FileEngine) RuntimeContext(in RuntimeContextInput) string {
	return BuildRuntimeContext(in)
}

func BuildRuntimeContext(in RuntimeContextInput) string {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	started := in.RunStartedAt.UTC()
	if started.IsZero() {
		started = now
	}
	elapsed := int(now.Sub(started).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	step := in.Step
	if step <= 0 {
		step = 1
	}
	costStatus := strings.TrimSpace(in.CostStatus)
	if costStatus == "" {
		costStatus = "pending"
	}

	var b strings.Builder
	b.WriteString("<runtime_context>\n")
	b.WriteString(fmt.Sprintf("run_started_at_utc: %s\n", started.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("current_time_utc: %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("elapsed_seconds: %d\n", elapsed))
	b.WriteString(fmt.Sprintf("step: %d\n", step))
	b.WriteString(fmt.Sprintf("prompt_tokens_total: %d\n", in.PromptTokensTotal))
	b.WriteString(fmt.Sprintf("completion_tokens_total: %d\n", in.CompletionTokensTotal))
	b.WriteString(fmt.Sprintf("total_tokens: %d\n", in.TotalTokens))
	b.WriteString(fmt.Sprintf("last_turn_tokens: %d\n", in.LastTurnTokens))
	b.WriteString(fmt.Sprintf("cost_usd_total: %.6f\n", in.CostUSDTotal))
	b.WriteString(fmt.Sprintf("last_turn_cost_usd: %.6f\n", in.LastTurnCostUSD))
	b.WriteString(fmt.Sprintf("cost_status: %s\n", costStatus))
	b.WriteString("</runtime_context>")
	return b.String()
}
```

### Key Observations for runtime_context.go

- Outputs an XML-style block with step, token, and cost info.
- A `context_status` tool would return similar data but in JSON format for the LLM to consume as a tool result.
- The runtime context is already injected as a system message each turn, so a `context_status` tool would give the LLM on-demand access to this data (plus message counts, estimated context window usage, etc.) without relying on the system-message injection.

---

## 7. internal/observationalmemory/token_estimator.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/observationalmemory/token_estimator.go`

```go
package observationalmemory

import "unicode/utf8"

type TokenEstimator interface {
	EstimateTextTokens(text string) int
	EstimateMessagesTokens(messages []TranscriptMessage) int
}

type RuneTokenEstimator struct{}

func (RuneTokenEstimator) EstimateTextTokens(text string) int {
	runes := utf8.RuneCountInString(text)
	if runes <= 0 {
		return 0
	}
	return (runes + 3) / 4
}

func (e RuneTokenEstimator) EstimateMessagesTokens(messages []TranscriptMessage) int {
	total := 0
	for _, msg := range messages {
		total += e.EstimateTextTokens(msg.Content)
	}
	return total
}
```

### Key Observations for token_estimator.go

- **`RuneTokenEstimator`**: Simple heuristic — approximately 1 token per 4 Unicode runes (characters). Formula: `(runes + 3) / 4`.
- **`TokenEstimator` interface**: Can be implemented with a more accurate tokenizer if needed.
- **`EstimateMessagesTokens`**: Takes `[]TranscriptMessage` (from observationalmemory package). A `context_status` tool would need to either use this directly or implement a similar estimator for `[]Message` (from harness package).
- The `TranscriptMessage` type in observationalmemory is different from the one in `tools/types.go` — same field names but different package. Both have `Index`, `Role`, `Name`, `ToolCallID`, `Content`.

---

## 8. internal/harness/tools/descriptions/embed.go

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/descriptions/embed.go`

```go
package descriptions

import (
	"embed"
	"strings"
)

//go:embed *.md
var FS embed.FS

// Load reads a tool description from the embedded filesystem.
// The filename should match the tool name (e.g., "cron_create.md").
func Load(name string) string {
	data, err := FS.ReadFile(name + ".md")
	if err != nil {
		panic("missing tool description: " + name + ".md")
	}
	return strings.TrimSpace(string(data))
}
```

### Key Observations for embed.go

- **Pattern**: Tool descriptions are markdown files in `internal/harness/tools/descriptions/`.
- **Naming convention**: File is `{tool_name}.md` (e.g., `read.md`, `bash.md`).
- **Usage**: `descriptions.Load("tool_name")` — panics if file is missing.
- **For new tools**: Create `compact_history.md` and `context_status.md` in the descriptions directory.

---

## 9. internal/harness/tools/read.go (example tool)

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/read.go`

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func readTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "read",
		Description:  descriptions.Load("read"),
		Action:       ActionRead,
		ParallelSafe: true,
		Tags:         []string{"read", "file", "view", "inspect", "contents"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string", "description": "relative file path inside workspace"},
				"file_path": map[string]any{"type": "string", "description": "alias of path"},
				"max_bytes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1048576},
				"offset":    map[string]any{"type": "integer", "minimum": 0, "description": "line offset"},
				"limit":     map[string]any{"type": "integer", "minimum": 1, "description": "max lines"},
			},
			"required": []string{"path"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Path     string `json:"path"`
			FilePath string `json:"file_path"`
			MaxBytes int    `json:"max_bytes"`
			Offset   int    `json:"offset"`
			Limit    int    `json:"limit"`
		}{MaxBytes: 16 * 1024}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse read args: %w", err)
		}
		if args.Path == "" {
			args.Path = args.FilePath
		}
		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}
		if args.MaxBytes <= 0 {
			args.MaxBytes = 16 * 1024
		}
		if args.MaxBytes > 1024*1024 {
			args.MaxBytes = 1024 * 1024
		}
		if args.Offset < 0 {
			args.Offset = 0
		}
		if args.Limit < 0 {
			args.Limit = 0
		}

		absPath, err := ResolveWorkspacePath(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}

		file, err := os.Open(absPath)
		if err != nil {
			return "", fmt.Errorf("open file: %w", err)
		}
		defer file.Close()

		content, err := io.ReadAll(io.LimitReader(file, int64(args.MaxBytes+1)))
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		truncated := len(content) > args.MaxBytes
		if truncated {
			content = content[:args.MaxBytes]
		}

		text := string(content)
		lineObjects := make([]map[string]any, 0)
		if args.Offset > 0 || args.Limit > 0 {
			lines := strings.Split(text, "\n")
			start := args.Offset
			if start > len(lines) {
				start = len(lines)
			}
			end := len(lines)
			if args.Limit > 0 && start+args.Limit < end {
				end = start + args.Limit
			}
			for i := start; i < end; i++ {
				lineObjects = append(lineObjects, map[string]any{"line_number": i + 1, "text": lines[i]})
			}
			text = strings.Join(lines[start:end], "\n")
		}

		version, err := ReadFileVersion(absPath)
		if err != nil {
			return "", err
		}

		result := map[string]any{
			"path":      NormalizeRelPath(workspaceRoot, absPath),
			"content":   text,
			"truncated": truncated,
			"version":   version,
		}
		if len(lineObjects) > 0 {
			result["lines"] = lineObjects
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
```

### Key Observations for read.go (tool pattern)

- **Constructor pattern**: `func readTool(workspaceRoot string) Tool` — takes dependencies, returns `Tool`.
- **Definition**: Uses `descriptions.Load("read")` for the description.
- **Handler closure**: `func(_ context.Context, raw json.RawMessage) (string, error)`.
- **Args parsing**: Unmarshal JSON into anonymous struct, validate, then execute.
- **Result format**: `MarshalToolResult(result)` serializes a `map[string]any` to JSON.
- **Tier**: Not set explicitly, defaults to zero value of `ToolTier` (which is `""` — treated as core).
- The `context.Context` parameter is available but unused in this tool (uses `_`). Other tools use it to read `RunIDFromContext`, `TranscriptReaderFromContext`, etc.

---

## 10. internal/harness/types.go (Message type with IsCompactSummary)

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/types.go`

```go
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
	MessageID        string     `json:"message_id,omitempty"`
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
	IsMeta           bool       `json:"is_meta,omitempty"`
	IsCompactSummary bool       `json:"is_compact_summary,omitempty"`
}

type CompletionRequest struct {
	Model           string                `json:"model"`
	Messages        []Message             `json:"messages"`
	Tools           []ToolDefinition      `json:"tools,omitempty"`
	Stream          func(CompletionDelta) `json:"-"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
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
	MaxSteps         int               `json:"max_steps,omitempty"`
	MaxCostUSD       float64           `json:"max_cost_usd,omitempty"`
	ReasoningEffort  string            `json:"reasoning_effort,omitempty"`
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
	Activations         *ActivationTracker        `json:"-"`
	SkillConstraints    *SkillConstraintTracker    `json:"-"`
	RolloutDir          string
}

type Logger interface {
	Error(msg string, keysAndValues ...any)
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

// ... hook types omitted for brevity (PreMessageHook, PostMessageHook, etc.)
```

### Key Observations for types.go (harness)

- **`Message.IsCompactSummary`**: Boolean flag (`json:"is_compact_summary,omitempty"`). This marks a message as being a compaction summary. Used by the conversation store's `CompactConversation` method and presumably by the LLM to understand that a message is a summary of prior history rather than actual conversation.
- **`Message.IsMeta`**: Boolean flag for meta-messages injected by tool results.
- **`RunnerConfig.ConversationStore`**: The runner holds a `ConversationStore` which has the `CompactConversation` method.

---

## 11. internal/harness/conversation_store.go (ConversationStore interface)

**Full path**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/conversation_store.go`

```go
package harness

import (
	"context"
	"time"
)

// Conversation holds metadata for a conversation.
type Conversation struct {
	ID               string    `json:"id"`
	Title            string    `json:"title,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	MsgCount         int       `json:"message_count"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	Pinned           bool      `json:"pinned,omitempty"`
	Workspace        string    `json:"workspace,omitempty"`
	TenantID         string    `json:"tenant_id,omitempty"`
}

// ConversationTokenCost holds token usage and cost data for a conversation run.
type ConversationTokenCost struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

// ConversationFilter optionally scopes ListConversations results.
type ConversationFilter struct {
	Workspace string
	TenantID  string
}

// MessageSearchResult is a single result from a full-text search over messages.
type MessageSearchResult struct {
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role"`
	Snippet        string `json:"snippet"`
}

// ConversationStore persists conversation messages across server restarts.
type ConversationStore interface {
	Migrate(ctx context.Context) error
	Close() error
	SaveConversation(ctx context.Context, convID string, msgs []Message) error
	SaveConversationWithCost(ctx context.Context, convID string, msgs []Message, cost ConversationTokenCost) error
	LoadMessages(ctx context.Context, convID string) ([]Message, error)
	ListConversations(ctx context.Context, filter ConversationFilter, limit, offset int) ([]Conversation, error)
	DeleteConversation(ctx context.Context, convID string) error
	UpdateConversationMeta(ctx context.Context, convID, workspace, tenantID string) error
	SearchMessages(ctx context.Context, query string, limit int) ([]MessageSearchResult, error)
	DeleteOldConversations(ctx context.Context, olderThan time.Time) (int, error)
	PinConversation(ctx context.Context, convID string, pin bool) error
	// CompactConversation summarizes early conversation history. Messages with
	// step index >= keepFromStep are retained; older messages are discarded and
	// replaced by a single summary message inserted at step 0. Retained messages
	// are renumbered starting at step 1.
	//
	// keepFromStep=0 keeps all existing messages and prepends the summary.
	// keepFromStep > max_step keeps no existing messages (only the summary remains).
	// Returns an error if the conversation does not exist or keepFromStep < 0.
	CompactConversation(ctx context.Context, convID string, keepFromStep int, summary Message) error
}
```

### Key Observations for conversation_store.go

- **`CompactConversation`**: Already exists on the interface. Takes `convID`, `keepFromStep`, and a `summary Message`. This is the persistence-layer method that a `compact_history` tool would call.
- **`LoadMessages`**: Returns `[]Message` for a conversation.
- **`SaveConversation` / `SaveConversationWithCost`**: Persists the full message array.
- The `compact_history` tool would need access to both the `ConversationStore` (for `CompactConversation`) and the `Runner.SummarizeMessages` (for generating the summary).

---

## 12. Key Observations

### For implementing `context_status` tool:

1. **Data sources**: Use `TranscriptReaderFromContext` to get message count and content. Use `RunMetadataFromContext` to get run/conversation IDs. Use the runner's `usageTotalsAccumulator` and `RunCostTotals` (accessible via accounting methods) for token/cost data.
2. **Token estimation**: Use `RuneTokenEstimator` from `observationalmemory` package or create a similar one in the tools package.
3. **No new dependencies needed**: The tool can use the existing `TranscriptReader` context key. The `RuntimeContextInput` fields are already available through the transcript reader and context keys.
4. **Tool tier**: Should be `TierCore` (always available) since the LLM needs to be able to check context status at any time.

### For implementing `compact_history` tool:

1. **Dependencies needed**:
   - Access to `ConversationStore.CompactConversation` — needs a new interface or the store itself passed through `BuildOptions`.
   - Access to `Runner.SummarizeMessages` — needs a summarizer interface passed through `BuildOptions`.
   - Access to the current run's conversation ID — via `RunMetadataFromContext`.
2. **Flow**: (a) Get conversation ID from context, (b) Get current messages via `TranscriptReader`, (c) Call `SummarizeMessages` on messages to compact, (d) Call `CompactConversation` on the store, (e) Return confirmation.
3. **`IsCompactSummary` flag**: The summary `Message` passed to `CompactConversation` should have `IsCompactSummary: true`.
4. **Tool tier**: Could be `TierDeferred` (only activated when needed) since compaction is an infrequent operation.

### Pattern for adding new tools:

1. Create `internal/harness/tools/descriptions/{tool_name}.md` with the tool description.
2. Create `internal/harness/tools/{tool_name}.go` with the constructor function.
3. Add any new dependencies to `BuildOptions` in `types.go`.
4. Register the tool in `catalog.go` by appending to the tools slice.
5. Write tests in `internal/harness/tools/{tool_name}_test.go`.

### Interfaces to consider creating:

- **`Summarizer`**: `SummarizeMessages(ctx context.Context, messages []Message) (string, error)` — abstracts the Runner's summarization capability for the tool layer.
- **`Compactor`**: `CompactConversation(ctx context.Context, convID string, keepFromStep int, summary Message) error` — wraps the ConversationStore's compaction method.
- Or a combined **`CompactionService`** interface that the tool can use.
