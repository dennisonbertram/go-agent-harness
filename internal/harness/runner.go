package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osuser "os/user"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"unicode/utf8"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/forensics/audittrail"
	"go-agent-harness/internal/forensics/causalgraph"
	"go-agent-harness/internal/forensics/contextwindow"
	"go-agent-harness/internal/forensics/costanomaly"
	"go-agent-harness/internal/forensics/errorchain"
	"go-agent-harness/internal/forensics/redaction"
	"go-agent-harness/internal/forensics/tooldecision"
	"go-agent-harness/internal/forensics/requestenvelope"
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
	// maxCostUSD is the per-run spending ceiling (0 = unlimited).
	maxCostUSD float64
	// allowedTools is the per-run base tool filter from RunRequest.AllowedTools.
	// When non-empty, only these tools (plus AlwaysAvailableTools) are offered
	// to the LLM. Skill constraints override this during skill execution.
	// Nil or empty means no per-run restriction.
	allowedTools []string
	// permissions is the effective two-axis permission configuration for this run.
	permissions PermissionConfig
	// recorder captures every event to a JSONL rollout file when RolloutDir is set.
	recorder *rollout.Recorder
	// recorderMu serialises Record/Close calls outside the main lock so that
	// a terminal Close() never races with a concurrent non-terminal Record().
	recorderMu sync.Mutex
	// recorderClosed is set under recorderMu after rec.Close() is called.
	// Any goroutine that captured a non-nil rec before detach must check this
	// flag inside recorderMu to prevent record-after-close.
	recorderClosed bool
	// auditWriter is the append-only hash-chained audit log writer.
	// Non-nil only when AuditTrailEnabled is set in RunnerConfig and RolloutDir is set.
	auditWriter *audittrail.AuditWriter
	// previousRunID is set when this run was created via ContinueRun.
	previousRunID string
	// currentStep tracks the current step number during execution.
	currentStep int
	// continued is set to true once ContinueRun has been called on this run,
	// preventing a second continuation without mutating the run's terminal Status.
	continued bool
	// snapshotBuilder collects a rolling window of tool calls and messages for
	// error context snapshots. Non-nil only when ErrorChainEnabled is set in
	// RunnerConfig.
	snapshotBuilder *errorchain.SnapshotBuilder
	// terminated is set to true once the terminal event (run.completed or
	// run.failed) has been emitted. Any subsequent emit() call returns
	// immediately to prevent post-terminal streaming callbacks from appending
	// events after the forensic record is closed.
	terminated bool
	// compactMu serializes auto-compact and manual CompactRun calls.
	compactMu sync.Mutex
	// resetIndex increments each time the agent calls reset_context.
	// 0 means no reset has occurred yet for this run.
	resetIndex int
	// scopedMCPRegistry is the per-run MCP registry created when
	// RunRequest.MCPServers is non-empty. It is closed when the run completes.
	// Nil when no per-run MCP servers are configured.
	scopedMCPRegistry *ScopedMCPRegistry
	// profileName is the profile name from RunRequest.ProfileName, stored so
	// that forked sub-runs inherit the parent's profile (MCP servers, etc.).
	profileName string
	// resolvedRoleModels is the fully-merged role model configuration for this
	// run (per-request overrides merged on top of runner-level config). It is
	// set once at the start of execute() and read by autoCompactMessages so
	// that the per-request Summarizer override is honoured during compaction.
	resolvedRoleModels RoleModels
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
	ErrRunNotFound     = errors.New("run not found")
	ErrNoPendingInput  = errors.New("no pending input")
	ErrInvalidRunInput = errors.New("invalid run input")
	// ErrRunNotCompleted is returned by ContinueRun when the target run has not
	// reached a completed status (e.g. it is still running or has failed).
	ErrRunNotCompleted = errors.New("run is not completed")
	// ErrRunNotActive is returned by SteerRun when the target run is not in an
	// active state (running or waiting for user).
	ErrRunNotActive = errors.New("run is not active")
	// ErrSteeringBufferFull is returned by SteerRun when the run's steering
	// channel is at capacity.
	ErrSteeringBufferFull = errors.New("steering buffer full")
	// ErrConversationAccessDenied is returned by StartRun when the caller
	// supplies a ConversationID that exists but belongs to a different
	// tenant or agent (cross-tenant/cross-agent disclosure prevention).
	ErrConversationAccessDenied = errors.New("conversation access denied")
)

// steeringBufferSize is the capacity of the per-run steering message channel.
const steeringBufferSize = 10

// maxEmptyRetries is the maximum number of consecutive empty LLM responses
// (no text content, no tool calls) before the runner stops retrying and
// treats the run as complete. Handles Gemini 2.5 Flash thinking mode where
// the model returns 0 completion_tokens with empty content.
const maxEmptyRetries = 3

// conversationOwner records the tenant and agent that own a conversation.
// This is used to enforce conversation scoping: a caller-supplied ConversationID
// must match the requesting tenant + agent before its history is loaded.
type conversationOwner struct {
	tenantID string
	agentID  string
}

type Runner struct {
	provider         Provider
	tools            *Registry
	config           RunnerConfig
	providerRegistry *catalog.ProviderRegistry
	activations      *ActivationTracker
	skillConstraints *SkillConstraintTracker
	envInfo          systemprompt.EnvironmentInfo

	mu                  sync.RWMutex
	runs                map[string]*runState
	conversations       map[string][]Message
	// conversationOwners maps conversation_id -> owner (tenantID + agentID).
	// It is populated when a run completes and its conversation is saved to the
	// in-memory conversations map. Used to validate caller-supplied conversation IDs.
	conversationOwners  map[string]conversationOwner
}

func NewRunner(provider Provider, tools *Registry, config RunnerConfig) *Runner {
	if config.DefaultModel == "" {
		config.DefaultModel = "gpt-4.1-mini"
	}
	if config.DefaultAgentIntent == "" {
		config.DefaultAgentIntent = "general"
	}
	// MaxSteps <= 0 means unlimited; no default cap is applied here.
	if config.AskUserTimeout <= 0 {
		config.AskUserTimeout = 5 * time.Minute
	}
	if config.HookFailureMode == "" {
		config.HookFailureMode = HookFailureModeFailClosed
	}
	if config.AutoCompactMode == "" {
		config.AutoCompactMode = "hybrid"
	}
	if config.AutoCompactThreshold == 0 {
		config.AutoCompactThreshold = 0.80
	}
	if config.AutoCompactKeepLast <= 0 {
		config.AutoCompactKeepLast = 8
	}
	if config.ModelContextWindow == 0 {
		config.ModelContextWindow = 128000
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
	envInfo := systemprompt.EnvironmentInfo{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		Shell:     os.Getenv("SHELL"),
	}
	if h, err := os.Hostname(); err == nil {
		envInfo.Hostname = h
	}
	if u, err := osuser.Current(); err == nil {
		envInfo.Username = u.Username
	}
	if wd, err := os.Getwd(); err == nil {
		envInfo.WorkingDir = wd
	}

	return &Runner{
		provider:            provider,
		tools:               tools,
		config:              config,
		providerRegistry:    config.ProviderRegistry,
		activations:         activations,
		skillConstraints:    skillConstraints,
		envInfo:             envInfo,
		runs:                make(map[string]*runState),
		conversations:       make(map[string][]Message),
		conversationOwners:  make(map[string]conversationOwner),
	}
}

// GetProviderRegistry returns the provider registry, if configured.
func (r *Runner) GetProviderRegistry() *catalog.ProviderRegistry {
	return r.providerRegistry
}

func (r *Runner) StartRun(req RunRequest) (Run, error) {
	if r.provider == nil {
		return Run{}, fmt.Errorf("provider is required")
	}
	if req.Prompt == "" {
		return Run{}, fmt.Errorf("prompt is required")
	}
	if req.MaxSteps < 0 {
		return Run{}, fmt.Errorf("max_steps must be >= 0 (0 means use runner default)")
	}
	if req.MaxCostUSD < 0 {
		return Run{}, fmt.Errorf("max_cost_usd must be >= 0 (0 means unlimited)")
	}
	if req.Permissions != nil {
		if err := ValidatePermissionConfig(*req.Permissions); err != nil {
			return Run{}, fmt.Errorf("invalid permissions: %w", err)
		}
	}
	if len(req.MCPServers) > 0 {
		if err := validateMCPServerConfigs(req.MCPServers); err != nil {
			return Run{}, fmt.Errorf("invalid mcp_servers: %w", err)
		}
	}

	model := req.Model
	if model == "" {
		model = r.config.DefaultModel
	}
	systemPrompt, resolvedPrompt, err := r.resolveSystemPrompt(req, model)
	if err != nil {
		return Run{}, err
	}

	now := time.Now().UTC()
	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = "default"
	}
	run := Run{
		ID:          r.nextID("run"),
		Prompt:      req.Prompt,
		Model:       model,
		Status:      RunStatusQueued,
		UsageTotals: &RunUsageTotals{},
		CostTotals:  &RunCostTotals{CostStatus: CostStatusPending},
		TenantID:    tenantID,
		AgentID:     agentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	run.ConversationID = strings.TrimSpace(req.ConversationID)
	if run.ConversationID == "" {
		run.ConversationID = run.ID
	}

	// Validate caller-supplied ConversationID against tenant/agent ownership.
	// Only applies when the caller explicitly passed a ConversationID (as
	// opposed to the auto-assigned case where run.ConversationID == run.ID).
	if strings.TrimSpace(req.ConversationID) != "" {
		if err := r.checkConversationOwnership(run.ConversationID, tenantID, agentID); err != nil {
			return Run{}, err
		}
	}

	// Create rollout recorder before acquiring the run lock so that any
	// filesystem error is surfaced at start time rather than mid-run.
	var rec *rollout.Recorder
	if r.config.RolloutDir != "" {
		var recErr error
		rec, recErr = rollout.NewRecorder(rollout.RecorderConfig{
			Dir:   r.config.RolloutDir,
			RunID: run.ID,
		})
		if recErr != nil && r.config.Logger != nil {
			r.config.Logger.Error("rollout recorder: failed to create", "run_id", run.ID, "error", recErr)
		}
	}

	// Create audit writer when AuditTrailEnabled and RolloutDir are set.
	// The audit log is written to <RolloutDir>/<YYYY-MM-DD>/audit.jsonl, a single
	// shared file (not per-run) since it captures all runs in the session.
	var aw *audittrail.AuditWriter
	if r.config.AuditTrailEnabled && r.config.RolloutDir != "" {
		auditPath := auditLogPath(r.config.RolloutDir)
		var awErr error
		aw, awErr = audittrail.NewAuditWriter(auditPath)
		if awErr != nil && r.config.Logger != nil {
			r.config.Logger.Error("audit trail: failed to create writer", "run_id", run.ID, "error", awErr)
		}
	}

	// Resolve effective permissions: use request value or fall back to default.
	effectivePerms := DefaultPermissionConfig()
	if req.Permissions != nil {
		effectivePerms = *req.Permissions
		// Fill in zero-value fields with defaults.
		if effectivePerms.Sandbox == "" {
			effectivePerms.Sandbox = SandboxScopeUnrestricted
		}
		if effectivePerms.Approval == "" {
			effectivePerms.Approval = ApprovalPolicyNone
		}
	}

	var sb *errorchain.SnapshotBuilder
	if r.config.ErrorChainEnabled {
		sb = errorchain.NewSnapshotBuilder(r.config.ErrorContextDepth)
	}
	r.mu.Lock()
	r.runs[run.ID] = &runState{
		run:                run,
		staticSystemPrompt: systemPrompt,
		promptResolved:     resolvedPrompt,
		usageTotals:        usageTotalsAccumulator{},
		costTotals:         RunCostTotals{CostStatus: CostStatusPending},
		messages:           make([]Message, 0, 16),
		events:             make([]Event, 0, 32),
		subscribers:        make(map[chan Event]struct{}),
		steeringCh:         make(chan string, steeringBufferSize),
		maxCostUSD:         req.MaxCostUSD,
		allowedTools:       req.AllowedTools,
		permissions:        effectivePerms,
		recorder:           rec,
		snapshotBuilder:    sb,
		auditWriter:        aw,
		profileName:        req.ProfileName,
	}
	r.mu.Unlock()

	go r.execute(run.ID, req)

	return run, nil
}

// checkConversationOwnership validates that a caller-supplied ConversationID
// belongs to the requesting tenant + agent before its history is loaded.
//
// The check is two-phase:
//  1. In-memory: if r.conversationOwners has an entry for convID, both
//     tenantID and agentID must match (strict check, both axes enforced).
//  2. Persistent store: if not found in memory but a ConversationStore is
//     configured, the store's tenant_id column is checked (agent_id is not
//     stored in the schema, so only the tenant axis is enforced here).
//
// Returns nil if the conversation does not exist yet (new conversation
// allowed), or if the caller matches the recorded owner.
// Returns ErrConversationAccessDenied if a mismatch is detected.
//
// tenantID normalization: the runner normalises "" → "default" on input, and
// the SQLite layer stores "" for "default" tenant rows. Both sides are
// normalised before comparison so "default" and "" compare equal.
func (r *Runner) checkConversationOwnership(convID, tenantID, agentID string) error {
	// Normalise: "" and "default" are the same tenant value.
	normTenant := func(t string) string {
		if t == "" {
			return "default"
		}
		return t
	}

	callerTenant := normTenant(tenantID)
	callerAgent := agentID

	// Phase 1: in-memory map (strongest check — tenant + agent both enforced).
	r.mu.RLock()
	owner, found := r.conversationOwners[convID]
	r.mu.RUnlock()

	if found {
		if normTenant(owner.tenantID) != callerTenant || owner.agentID != callerAgent {
			return ErrConversationAccessDenied
		}
		return nil
	}

	// Phase 2: persistent store (tenant-only check — schema has no agent_id).
	if r.config.ConversationStore == nil {
		// No store configured and not in memory — brand-new conversation, allow.
		return nil
	}
	conv, err := r.config.ConversationStore.GetConversationOwner(context.Background(), convID)
	if err != nil {
		// Treat store errors as a hard failure to prevent silent bypass.
		return fmt.Errorf("conversation ownership check: %w", err)
	}
	if conv == nil {
		// Not found in store either — brand-new conversation, allow.
		return nil
	}
	// Found in store: check tenant match only (no agent_id column in schema).
	storedTenant := normTenant(conv.TenantID)
	if storedTenant != callerTenant {
		return ErrConversationAccessDenied
	}
	return nil
}

func (r *Runner) resolveSystemPrompt(req RunRequest, model string) (string, *systemprompt.ResolvedPrompt, error) {
	if strings.TrimSpace(req.SystemPrompt) != "" {
		return req.SystemPrompt, nil, nil
	}
	if r.config.PromptEngine == nil {
		return r.config.DefaultSystemPrompt, nil, nil
	}
	extensions := mapPromptExtensions(req.PromptExtensions)
	resolved, err := r.config.PromptEngine.Resolve(systemprompt.ResolveRequest{
		Model:              model,
		AgentIntent:        req.AgentIntent,
		DefaultAgentIntent: r.config.DefaultAgentIntent,
		PromptProfile:      req.PromptProfile,
		TaskContext:        req.TaskContext,
		Extensions:         extensions,
	})
	if err != nil {
		return "", nil, err
	}
	return resolved.StaticPrompt, &resolved, nil
}

func mapPromptExtensions(input *PromptExtensions) systemprompt.Extensions {
	if input == nil {
		return systemprompt.Extensions{}
	}
	return systemprompt.Extensions{
		Behaviors: append([]string(nil), input.Behaviors...),
		Talents:   append([]string(nil), input.Talents...),
		Skills:    append([]string(nil), input.Skills...),
		Custom:    input.Custom,
	}
}

func (r *Runner) GetRun(runID string) (Run, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.runs[runID]
	if !ok {
		return Run{}, false
	}
	out := state.run
	if state.run.UsageTotals != nil {
		usage := *state.run.UsageTotals
		out.UsageTotals = &usage
	}
	if state.run.CostTotals != nil {
		cost := *state.run.CostTotals
		out.CostTotals = &cost
	}
	return out, true
}

// ContinueRun appends a follow-up user message to a completed run and starts a
// new execution under the same conversation_id. The original run state is kept
// intact. The new run shares the conversation history so the LLM sees the full
// transcript.
//
// Errors:
//   - ErrRunNotFound     — the source run does not exist.
//   - ErrRunNotCompleted — the source run has not reached RunStatusCompleted
//     (it is still running, queued, waiting for user, or has failed).
//   - validation error   — message is empty.
//
// The method is safe for concurrent use. Only one goroutine can successfully
// continue a given completed run: the first to acquire the lock transitions
// the source run's status away from RunStatusCompleted, so subsequent callers
// see ErrRunNotCompleted and fail.
func (r *Runner) ContinueRun(runID, message string) (Run, error) {
	if strings.TrimSpace(message) == "" {
		return Run{}, fmt.Errorf("message is required")
	}

	// Atomically check that the run exists and is completed, then immediately
	// stamp it with RunStatusRunning to prevent any other goroutine from also
	// starting a continuation.  All snapshot values are read under the same
	// lock so we never release it between check and mutation.
	r.mu.Lock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.Unlock()
		return Run{}, ErrRunNotFound
	}
	if state.run.Status != RunStatusCompleted {
		r.mu.Unlock()
		return Run{}, ErrRunNotCompleted
	}
	if state.continued {
		r.mu.Unlock()
		return Run{}, fmt.Errorf("run %q has already been continued", runID)
	}

	// Snapshot before we release.
	convID := state.run.ConversationID
	existingModel := state.run.Model
	existingTenantID := state.run.TenantID
	existingAgentID := state.run.AgentID
	systemPrompt := state.staticSystemPrompt
	promptResolved := state.promptResolved
	// Snapshot security controls so the continuation inherits the same budget
	// ceiling and permission constraints as the source run.  Without this,
	// ContinueRun would default maxCostUSD to 0 (unlimited) and permissions
	// to a zero-value struct, allowing both budget bypass and permission bypass.
	srcMaxCostUSD := state.maxCostUSD
	srcPermissions := state.permissions
	// Snapshot resolvedRoleModels so the continuation honours any per-request
	// RoleModels overrides that were active on the source run. Without this,
	// the continuation's execute() call re-resolves from req.RoleModels (nil)
	// and falls back to runner-level config only, silently dropping any
	// per-request Primary or Summarizer overrides.
	srcResolvedRoleModels := state.resolvedRoleModels

	// Mark the source run as continued so no second goroutine can also
	// continue it. We do NOT mutate run.Status — it stays Completed.
	state.continued = true

	now := time.Now().UTC()
	newRun := Run{
		ID:             r.nextID("run"),
		Prompt:         message,
		Model:          existingModel,
		Status:         RunStatusQueued,
		UsageTotals:    &RunUsageTotals{},
		CostTotals:     &RunCostTotals{CostStatus: CostStatusPending},
		TenantID:       existingTenantID,
		ConversationID: convID,
		AgentID:        existingAgentID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	// Create rollout recorder for the continuation run (while lock is held,
	// but recorder creation doesn't need it — release lock before creating).
	r.mu.Unlock()

	var contRec *rollout.Recorder
	if r.config.RolloutDir != "" {
		var recErr error
		contRec, recErr = rollout.NewRecorder(rollout.RecorderConfig{
			Dir:   r.config.RolloutDir,
			RunID: newRun.ID,
		})
		if recErr != nil && r.config.Logger != nil {
			r.config.Logger.Error("rollout recorder: failed to create for continuation", "run_id", newRun.ID, "error", recErr)
		}
	}

	var contSB *errorchain.SnapshotBuilder
	if r.config.ErrorChainEnabled {
		contSB = errorchain.NewSnapshotBuilder(r.config.ErrorContextDepth)
	}
	r.mu.Lock()
	r.runs[newRun.ID] = &runState{
		run:                newRun,
		staticSystemPrompt: systemPrompt,
		promptResolved:     promptResolved,
		usageTotals:        usageTotalsAccumulator{},
		costTotals:         RunCostTotals{CostStatus: CostStatusPending},
		messages:           make([]Message, 0, 16),
		events:             make([]Event, 0, 32),
		subscribers:        make(map[chan Event]struct{}),
		steeringCh:         make(chan string, steeringBufferSize),
		maxCostUSD:         srcMaxCostUSD,
		permissions:        srcPermissions,
		resolvedRoleModels: srcResolvedRoleModels,
		recorder:           contRec,
		previousRunID:      runID,
		snapshotBuilder:    contSB,
	}
	r.mu.Unlock()

	// Build the request after the lock is released.
	// Propagate resolvedRoleModels into the RunRequest so that execute()'s
	// resolveRoleModels() call re-applies the same per-request overrides
	// (Primary, Summarizer) rather than silently falling back to runner-level
	// config when the originating request had per-request RoleModels set.
	var contRoleModels *RoleModels
	if srcResolvedRoleModels.Primary != "" || srcResolvedRoleModels.Summarizer != "" {
		rm := srcResolvedRoleModels
		contRoleModels = &rm
	}
	req := RunRequest{
		Prompt:         message,
		Model:          existingModel,
		ConversationID: convID,
		TenantID:       existingTenantID,
		AgentID:        existingAgentID,
		RoleModels:     contRoleModels,
	}
	if systemPrompt != "" {
		req.SystemPrompt = systemPrompt
	}

	go r.execute(newRun.ID, req)

	return newRun, nil
}

// GetRunSummary computes a telemetry summary for a completed (or failed) run
// by scanning the run's event history. Returns ErrRunNotFound if the run does
// not exist, or an error if the run is still in progress.
func (r *Runner) GetRunSummary(runID string) (RunSummary, error) {
	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return RunSummary{}, ErrRunNotFound
	}
	run := state.run
	events := append([]Event(nil), state.events...)
	acc := state.usageTotals
	costTotals := state.costTotals
	r.mu.RUnlock()

	if run.Status != RunStatusCompleted && run.Status != RunStatusFailed {
		return RunSummary{}, fmt.Errorf("run %q is still %s", runID, run.Status)
	}

	stepsTaken := 0
	var toolCalls []ToolCallSummary
	currentStep := 0
	for _, evt := range events {
		switch evt.Type {
		case EventLLMTurnRequested:
			if stepVal, ok := evt.Payload["step"]; ok {
				if s, ok := stepVal.(float64); ok {
					currentStep = int(s)
				} else if s, ok := stepVal.(int); ok {
					currentStep = s
				}
			}
			stepsTaken++
		case EventToolCallStarted:
			name, _ := evt.Payload["tool"].(string)
			toolCalls = append(toolCalls, ToolCallSummary{
				ToolName: name,
				Step:     currentStep,
			})
		}
	}

	var cacheHitRate float64
	if acc.hasCachedPromptTokens && acc.promptTokensTotal > 0 {
		cacheHitRate = float64(acc.cachedPromptTokens) / float64(acc.promptTokensTotal)
	}

	summary := RunSummary{
		RunID:                 runID,
		Status:                run.Status,
		StepsTaken:            stepsTaken,
		TotalPromptTokens:     acc.promptTokensTotal,
		TotalCompletionTokens: acc.completionTokensTotal,
		TotalCostUSD:          costTotals.CostUSDTotal,
		CostStatus:            costTotals.CostStatus,
		ToolCalls:             toolCalls,
		CacheHitRate:          cacheHitRate,
		Error:                 run.Error,
	}
	if summary.ToolCalls == nil {
		summary.ToolCalls = []ToolCallSummary{}
	}
	return summary, nil
}

func (r *Runner) PendingInput(runID string) (htools.AskUserQuestionPending, error) {
	r.mu.RLock()
	_, ok := r.runs[runID]
	r.mu.RUnlock()
	if !ok {
		return htools.AskUserQuestionPending{}, ErrRunNotFound
	}
	if r.config.AskUserBroker == nil {
		return htools.AskUserQuestionPending{}, ErrNoPendingInput
	}
	pending, ok := r.config.AskUserBroker.Pending(runID)
	if !ok {
		return htools.AskUserQuestionPending{}, ErrNoPendingInput
	}
	return pending, nil
}

func (r *Runner) SubmitInput(runID string, answers map[string]string) error {
	r.mu.RLock()
	_, ok := r.runs[runID]
	r.mu.RUnlock()
	if !ok {
		return ErrRunNotFound
	}
	if r.config.AskUserBroker == nil {
		return ErrNoPendingInput
	}
	if err := r.config.AskUserBroker.Submit(runID, answers); err != nil {
		if errors.Is(err, ErrNoPendingUserQuestion) {
			return ErrNoPendingInput
		}
		if errors.Is(err, ErrInvalidUserQuestionInput) {
			return ErrInvalidRunInput
		}
		return err
	}
	return nil
}

// SteerRun injects a guidance message into a running run. The message is
// appended to the transcript as a user message before the next LLM call.
//
// Errors:
//   - ErrRunNotFound        — the run does not exist.
//   - ErrRunNotActive       — the run is not currently active (already completed or failed).
//   - ErrSteeringBufferFull — the steering channel is at capacity; try again later.
//   - validation error      — message is empty.
func (r *Runner) SteerRun(runID, message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("message is required")
	}

	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return ErrRunNotFound
	}
	status := state.run.Status
	steeringCh := state.steeringCh
	r.mu.RUnlock()

	if status != RunStatusRunning && status != RunStatusWaitingForUser {
		return ErrRunNotActive
	}

	select {
	case steeringCh <- message:
		return nil
	default:
		return ErrSteeringBufferFull
	}
}

func (r *Runner) Subscribe(runID string) ([]Event, <-chan Event, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.runs[runID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("run %q not found", runID)
	}

	// Deep-clone each historical event's payload so callers cannot mutate
	// the stored forensic history by modifying nested structures.
	history := make([]Event, len(state.events))
	for i, ev := range state.events {
		history[i] = ev
		history[i].Payload = deepClonePayload(ev.Payload)
	}
	ch := make(chan Event, 64)
	state.subscribers[ch] = struct{}{}

	cancel := func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		state, ok := r.runs[runID]
		if !ok {
			return
		}
		if _, exists := state.subscribers[ch]; exists {
			delete(state.subscribers, ch)
			close(ch)
		}
	}
	return history, ch, cancel, nil
}

// RunPrompt implements htools.AgentRunner. It starts a new run with the given
// prompt (using the runner's default model and config) and waits for it to
// complete, returning the run's final output. This satisfies the AgentRunner
// interface required by the skill tool for plain (non-forked) sub-runs.
func (r *Runner) RunPrompt(ctx context.Context, prompt string) (string, error) {
	run, err := r.StartRun(RunRequest{Prompt: prompt})
	if err != nil {
		return "", fmt.Errorf("RunPrompt: start run: %w", err)
	}

	history, stream, cancel, err := r.Subscribe(run.ID)
	if err != nil {
		return "", fmt.Errorf("RunPrompt: subscribe: %w", err)
	}
	defer cancel()

	for _, ev := range history {
		if IsTerminalEvent(ev.Type) {
			return r.forkResultFromRun(run.ID).Output, nil
		}
	}

	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				return r.forkResultFromRun(run.ID).Output, nil
			}
			if IsTerminalEvent(ev.Type) {
				return r.forkResultFromRun(run.ID).Output, nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// RunForkedSkill implements htools.ForkedAgentRunner. It starts a new sub-run
// for the given ForkConfig and waits for it to complete. The sub-run inherits
// the parent run's SystemPrompt and Permissions (looked up via the run ID
// embedded in ctx). AllowedTools from ForkConfig is forwarded as RunRequest.AllowedTools.
func (r *Runner) RunForkedSkill(ctx context.Context, config htools.ForkConfig) (htools.ForkResult, error) {
	// Build the sub-run request, forwarding AllowedTools from the fork config.
	req := RunRequest{
		Prompt:       config.Prompt,
		AllowedTools: config.AllowedTools,
	}

	// Inherit SystemPrompt, Permissions, and ProfileName from the parent run when possible.
	if meta, ok := htools.RunMetadataFromContext(ctx); ok && meta.RunID != "" {
		r.mu.RLock()
		parentState, parentOK := r.runs[meta.RunID]
		if parentOK {
			req.SystemPrompt = parentState.staticSystemPrompt
			perms := parentState.permissions
			req.Permissions = &perms
			req.ProfileName = parentState.profileName
		}
		r.mu.RUnlock()
	}

	run, err := r.StartRun(req)
	if err != nil {
		return htools.ForkResult{}, fmt.Errorf("RunForkedSkill: start sub-run: %w", err)
	}

	// Wait for the sub-run to reach a terminal state.
	history, stream, cancel, err := r.Subscribe(run.ID)
	if err != nil {
		return htools.ForkResult{}, fmt.Errorf("RunForkedSkill: subscribe: %w", err)
	}
	defer cancel()

	// Check if already terminal (run completed synchronously before Subscribe).
	for _, ev := range history {
		if IsTerminalEvent(ev.Type) {
			return r.forkResultFromRun(run.ID), nil
		}
	}

	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				return r.forkResultFromRun(run.ID), nil
			}
			if IsTerminalEvent(ev.Type) {
				return r.forkResultFromRun(run.ID), nil
			}
		case <-ctx.Done():
			return htools.ForkResult{Error: ctx.Err().Error()}, ctx.Err()
		}
	}
}

// forkResultFromRun extracts a ForkResult from a completed run state.
func (r *Runner) forkResultFromRun(runID string) htools.ForkResult {
	r.mu.RLock()
	state, ok := r.runs[runID]
	r.mu.RUnlock()
	if !ok {
		return htools.ForkResult{Error: "run not found"}
	}
	if state.run.Error != "" {
		return htools.ForkResult{Error: state.run.Error}
	}
	return htools.ForkResult{Output: state.run.Output}
}

// resolveProvider determines which Provider to use for a run.
// Returns the provider, its name, and any error.
func (r *Runner) resolveProvider(runID, model, preferredProvider string, allowFallback bool) (Provider, string, error) {
	if r.providerRegistry == nil {
		return r.provider, "default", nil
	}

	// If caller explicitly specified a provider, try it first.
	if preferredProvider != "" {
		client, err := r.providerRegistry.GetClient(preferredProvider)
		if err == nil {
			if p, ok := client.(Provider); ok {
				return p, preferredProvider, nil
			}
		}
		// Preferred provider unavailable — emit warning and fall through to auto-detection if allowed.
		if !allowFallback {
			return nil, "", fmt.Errorf("requested provider %q: unavailable or does not implement Provider interface", preferredProvider)
		}
		r.emit(runID, EventPromptWarning, map[string]any{
			"code":    "provider_fallback",
			"message": fmt.Sprintf("requested provider %q unavailable, falling back to auto-detection", preferredProvider),
		})
	}

	client, providerName, err := r.providerRegistry.GetClientForModel(model)
	if err != nil {
		// Model not found or client creation failed
		if allowFallback {
			r.emit(runID, EventPromptWarning, map[string]any{
				"code":    "provider_fallback",
				"message": fmt.Sprintf("model %q provider unavailable (%v), falling back to default provider", model, err),
			})
			return r.provider, "default", nil
		}
		return nil, "", fmt.Errorf("model %q: %w", model, err)
	}

	// Type-assert ProviderClient to Provider interface
	p, ok := client.(Provider)
	if !ok {
		if allowFallback {
			r.emit(runID, EventPromptWarning, map[string]any{
				"code":    "provider_fallback",
				"message": fmt.Sprintf("provider %q client does not implement Provider interface, falling back to default", providerName),
			})
			return r.provider, "default", nil
		}
		return nil, "", fmt.Errorf("provider %q client does not implement Provider interface", providerName)
	}

	return p, providerName, nil
}

func (r *Runner) execute(runID string, req RunRequest) {
	// Recover from any panic inside the step loop so that a misbehaving tool
	// handler or internal bug does not crash the entire server process.
	// The panic value is converted to a descriptive error, emitted as a
	// run.failed event, and logged with a stack trace.
	defer func() {
		if p := recover(); p != nil {
			stack := debug.Stack()
			errMsg := fmt.Sprintf("internal panic: %v", p)
			if r.config.Logger != nil {
				r.config.Logger.Error("runner: recovered panic in execute",
					"run_id", runID,
					"panic", p,
					"stack", string(stack),
				)
			}
			r.failRun(runID, fmt.Errorf("%s", errMsg))
		}
	}()

	r.setStatus(runID, RunStatusRunning, "", "")

	// Build run.started payload with optional previous_run_id for continuations.
	startPayload := map[string]any{"prompt": req.Prompt}
	r.mu.RLock()
	if state, ok := r.runs[runID]; ok && state.previousRunID != "" {
		startPayload["previous_run_id"] = state.previousRunID
	}
	r.mu.RUnlock()
	r.emit(runID, EventRunStarted, startPayload)

	// Audit trail: write run.started with provenance (model, initiator prefix).
	if r.config.AuditTrailEnabled {
		auditModel := req.Model
		if auditModel == "" {
			auditModel = r.config.DefaultModel
		}
		r.writeAudit(runID, audittrail.AuditRecord{
			RunID:     runID,
			EventType: string(EventRunStarted),
			Payload: map[string]any{
				"prompt":                  req.Prompt,
				"model":                   auditModel,
				"initiator_api_key_prefix": req.InitiatorAPIKeyPrefix,
			},
		})
	}

	model := req.Model
	if model == "" {
		model = r.config.DefaultModel
	}

	// Resolve per-role model overrides. primaryModel is used in CompletionRequests
	// for the main step loop. An empty Primary falls back to the base model.
	roleModels := r.resolveRoleModels(req)
	primaryModel := model
	if roleModels.Primary != "" {
		primaryModel = roleModels.Primary
	}

	activeProvider, providerName, err := r.resolveProvider(runID, model, req.ProviderName, req.AllowFallback)
	if err != nil {
		r.failRun(runID, err)
		return
	}

	// Set provider name and resolved role models on run state.
	// resolvedRoleModels is stored so that autoCompactMessages can honour the
	// per-request Summarizer override without needing the original RunRequest.
	r.mu.Lock()
	if state, ok := r.runs[runID]; ok {
		state.run.ProviderName = providerName
		state.resolvedRoleModels = roleModels
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
	r.snapshotRecordMessage(runID, "user", req.Prompt)

	if len(priorMessages) > 0 {
		r.emit(runID, EventConversationContinued, map[string]any{
			"conversation_id":     r.conversationID(runID),
			"prior_message_count": len(priorMessages),
		})
	}
	r.setMessages(runID, messages)

	// Build per-run MCP registry when profile and/or run-level MCP servers are configured.
	// Profile servers shadow global server names (no error); run-level servers error on collision.
	if req.ProfileName != "" || len(req.MCPServers) > 0 {
		var profileMCPServers []MCPServerConfig

		if req.ProfileName != "" {
			// Resolve the profiles directory: use runner config value or fall back to default.
			profilesDir := r.config.ProfilesDir
			if profilesDir == "" {
				profilesDir = defaultProfilesDir()
			}
			profileCfg, profileErr := loadProfileMCPServers(profilesDir, req.ProfileName)
			if profileErr != nil {
				// Non-fatal: log and continue without profile servers.
				if r.config.Logger != nil {
					r.config.Logger.Error("failed to load profile MCP servers",
						"run_id", runID,
						"profile", req.ProfileName,
						"error", profileErr)
				}
			} else {
				for name, srv := range profileCfg {
					// Harness MCPServerConfig infers transport from Command/URL;
					// the config.MCPServerConfig Transport field is not forwarded here.
					profileMCPServers = append(profileMCPServers, MCPServerConfig{
						Name:    name,
						Command: srv.Command,
						Args:    srv.Args,
						URL:     srv.URL,
					})
				}
			}
		}

		scopedReg, mcpErr := buildPerRunMCPRegistry(
			r.config.GlobalMCPRegistry,
			r.config.GlobalMCPServerNames,
			profileMCPServers,
			req.MCPServers,
		)
		if mcpErr != nil {
			r.failRun(runID, fmt.Errorf("build per-run MCP registry: %w", mcpErr))
			return
		}
		r.mu.Lock()
		if state, ok := r.runs[runID]; ok {
			state.scopedMCPRegistry = scopedReg
		} else {
			// Run was cancelled before we could store the registry; clean up.
			_ = scopedReg.Close()
		}
		r.mu.Unlock()
	}

	// Resolve the effective step limit for this run.
	// Priority: per-run request > runner config.
	// 0 in either position means "no limit" once chosen.
	effectiveMaxSteps := r.config.MaxSteps
	if req.MaxSteps > 0 {
		effectiveMaxSteps = req.MaxSteps
	}
	// effectiveMaxSteps == 0 means unlimited.

	// Forensics: per-run tracking state.
	// callSeq is the sequential tool-call counter (increments across all steps).
	callSeq := 0
	// antiPatternCounts tracks (tool_name + "\x00" + args) -> call count.
	// Only allocated when DetectAntiPatterns is enabled.
	var antiPatternCounts map[string]int
	if r.config.DetectAntiPatterns {
		antiPatternCounts = make(map[string]int)
	}
	// alreadyAlerted tracks keys for which a tool.antipattern event was already
	// emitted, so we don't spam repeated events for the same pattern.
	var alreadyAlerted map[string]bool
	if r.config.DetectAntiPatterns {
		alreadyAlerted = make(map[string]bool)
	}
	// costAnomalyDetector is allocated only when CostAnomalyDetectionEnabled
	// is true. It tracks per-step costs and fires alerts when a step is
	// disproportionately expensive relative to the rolling average.
	var costAnomalyDetector *costanomaly.Detector
	if r.config.CostAnomalyDetectionEnabled {
		multiplier := r.config.CostAnomalyStepMultiplier
		if multiplier <= 0 {
			multiplier = 2.0
		}
		costAnomalyDetector = costanomaly.NewDetector(multiplier)
	}
	// causalBuilder is allocated only when CausalGraphEnabled is true.
	// It accumulates Tier 1 (context deps) and Tier 2 (data-flow) events
	// and emits a causal.graph.snapshot at run end.
	var causalBuilder *causalgraph.Builder
	if r.config.CausalGraphEnabled {
		causalBuilder = causalgraph.NewBuilder()
	}
	// consecutiveEmptyResponses counts consecutive LLM turns that returned
	// no text content and no tool calls. When this reaches maxEmptyRetries the
	// run is treated as complete rather than continuing indefinitely.
	// This handles Gemini 2.5 Flash thinking mode: it "thinks" for 26+ seconds
	// and then returns a response with 0 completion_tokens and 0 tool_calls.
	consecutiveEmptyResponses := 0

	// emitCausalGraph builds and emits the causal graph snapshot when
	// CausalGraphEnabled is true. Called before terminal events.
	emitCausalGraph := func(lastStep int) {
		if causalBuilder == nil {
			return
		}
		graph := causalBuilder.Build()
		graphJSON, err := json.Marshal(graph)
		if err != nil {
			return
		}
		var graphMap any
		json.Unmarshal(graphJSON, &graphMap)
		r.emit(runID, EventCausalGraphSnapshot, map[string]any{
			"step":  lastStep,
			"graph": graphMap,
		})
	}

	for step := 1; effectiveMaxSteps == 0 || step <= effectiveMaxSteps; step++ {
		// Update currentStep on runState so emit() includes it in all events.
		r.mu.Lock()
		if s, ok := r.runs[runID]; ok {
			s.currentStep = step
		}
		r.mu.Unlock()

		// Re-read messages from state at the start of each step so CompactRun()
		// results from the inter-step window take effect before building the next
		// LLM context (#232). messagesForStep holds compactMu only for the brief
		// slice copy — it never holds compactMu while calling setMessages, so
		// there is no lock-order inversion.
		{
			r.mu.RLock()
			st, ok := r.runs[runID]
			r.mu.RUnlock()
			if !ok {
				return
			}
			messages = r.messagesForStep(st)
		}

		stepStartTime := time.Now()
		r.emit(runID, EventRunStepStarted, map[string]any{
			"step":          step,
			"step_start_ms": stepStartTime.UnixMilli(),
		})
		// Drain any pending steering messages and inject them as user messages.
		r.drainSteering(runID, &messages)

		r.emit(runID, EventLLMTurnRequested, map[string]any{"step": step})

		// memorySnippetForSnapshot captures the memory snippet text so that
		// CaptureRequestEnvelope can include it in the llm.request.snapshot event.
		var memorySnippetForSnapshot string
		turnMessages := make([]Message, 0, len(messages)+4)
		if r.config.MemoryManager != nil && r.config.MemoryManager.Mode() != om.ModeOff {
			snippet, _, err := r.config.MemoryManager.Snippet(context.Background(), r.scopeKey(runID))
			if err != nil {
				r.emit(runID, EventMemoryObserveFailed, map[string]any{"step": step, "error": err.Error()})
			} else if strings.TrimSpace(snippet) != "" {
				turnMessages = append(turnMessages, Message{Role: "system", Content: snippet})
				memorySnippetForSnapshot = snippet
			}
		}
		if systemPrompt != "" {
			turnMessages = append(turnMessages, Message{Role: "system", Content: systemPrompt})
		}
		if resolvedPrompt != nil && r.config.PromptEngine != nil {
			usageTotals, costTotals := r.accountingTotals(runID)

			// Estimate context tokens for runtime context reporting.
			estimatedCtxTokens := 0
			for _, m := range messages {
				runes := utf8.RuneCountInString(m.Content)
				if runes > 0 {
					estimatedCtxTokens += (runes + 3) / 4
				}
			}

			envInfo := r.envInfo
			envInfo.Model = model
			if r.providerRegistry != nil {
				providerName, found := r.providerRegistry.ResolveProvider(model)
				if found {
					cat := r.providerRegistry.Catalog()
					if cat != nil {
						if result, ok := cat.ModelInfo(providerName, model); ok && result.Model.Pricing != nil {
							envInfo.InputCostPerMToken = result.Model.Pricing.InputPer1MTokensUSD
							envInfo.OutputCostPerMToken = result.Model.Pricing.OutputPer1MTokensUSD
						}
					}
				}
			}
			runtimeContext := strings.TrimSpace(r.config.PromptEngine.RuntimeContext(systemprompt.RuntimeContextInput{
				RunStartedAt:           runStartedAt,
				Now:                    time.Now().UTC(),
				Step:                   step,
				PromptTokensTotal:      usageTotals.PromptTokensTotal,
				CompletionTokensTotal:  usageTotals.CompletionTokensTotal,
				TotalTokens:            usageTotals.TotalTokens,
				LastTurnTokens:         usageTotals.LastTurnTokens,
				CostUSDTotal:           costTotals.CostUSDTotal,
				LastTurnCostUSD:        costTotals.LastTurnCostUSD,
				CostStatus:             string(costTotals.CostStatus),
				EstimatedContextTokens: estimatedCtxTokens,
				MessageCount:           len(messages),
				Environment:            envInfo,
			}))
			if runtimeContext != "" {
				turnMessages = append(turnMessages, Message{Role: "system", Content: runtimeContext})
			}
		}
		turnMessages = append(turnMessages, copyMessages(messages)...)

		// Proactive auto-compaction: if enabled, estimate token usage and
		// compact messages before sending them to the LLM.
		if r.config.AutoCompactEnabled && r.config.ModelContextWindow > 0 {
			estimated := 0
			for _, m := range turnMessages {
				runes := utf8.RuneCountInString(m.Content)
				if runes > 0 {
					estimated += (runes + 3) / 4
				}
			}
			ratio := float64(estimated) / float64(r.config.ModelContextWindow)
			if ratio > r.config.AutoCompactThreshold {
				r.emit(runID, EventAutoCompactStarted, map[string]any{
					"estimated_tokens":    estimated,
					"context_window":      r.config.ModelContextWindow,
					"threshold":           r.config.AutoCompactThreshold,
					"ratio":               ratio,
					"mode":                r.config.AutoCompactMode,
				})
				compactedMsgs, compactErr := r.autoCompactMessages(runID, messages)
				if compactErr == nil && compactedMsgs != nil {
					afterTokens := 0
					for _, m := range compactedMsgs {
						runes := utf8.RuneCountInString(m.Content)
						if runes > 0 {
							afterTokens += (runes + 3) / 4
						}
					}
					messages = compactedMsgs
					r.setMessages(runID, messages)
					// Rebuild turnMessages with the compacted messages.
					turnMessages = turnMessages[:0]
					if r.config.MemoryManager != nil && r.config.MemoryManager.Mode() != om.ModeOff {
						snippet, _, err := r.config.MemoryManager.Snippet(context.Background(), r.scopeKey(runID))
						if err == nil && strings.TrimSpace(snippet) != "" {
							turnMessages = append(turnMessages, Message{Role: "system", Content: snippet})
						}
					}
					if systemPrompt != "" {
						turnMessages = append(turnMessages, Message{Role: "system", Content: systemPrompt})
					}
					turnMessages = append(turnMessages, copyMessages(messages)...)
					r.emit(runID, EventAutoCompactCompleted, map[string]any{
						"before_tokens": estimated,
						"after_tokens":  afterTokens,
						"mode":          r.config.AutoCompactMode,
					})
				} else if compactErr != nil {
					// Log but do not fail the run.
					r.emit(runID, EventAutoCompactCompleted, map[string]any{
						"before_tokens": estimated,
						"after_tokens":  estimated,
						"mode":          r.config.AutoCompactMode,
						"error":         compactErr.Error(),
					})
				}
			}
		}

		completionReq := CompletionRequest{
			Model:           primaryModel,
			Messages:        turnMessages,
			Tools:           r.filteredToolsForRun(runID),
			ReasoningEffort: req.ReasoningEffort,
			Stream: func(delta CompletionDelta) {
				r.emitCompletionDelta(runID, step, delta)
			},
		}

		completionReq, blocked, err := r.applyPreHooks(context.Background(), runID, step, completionReq)
		if err != nil {
			r.failRun(runID, err)
			return
		}
		if blocked != nil {
			reason := blocked.reason
			if reason == "" {
				reason = "blocked"
			}
			r.failRun(runID, fmt.Errorf("blocked by pre-message hook %s: %s", blocked.hookName, reason))
			return
		}

		// Emit llm.request.snapshot before the provider call when forensic
		// capture is enabled. The snapshot captures a hash of all message
		// content (avoiding PII/bloat), the list of tool names, the memory
		// snippet (if any), and the current step number.
		if r.config.CaptureRequestEnvelope {
			// Build a concatenated string of all message content for hashing.
			var promptBuilder strings.Builder
			for _, m := range completionReq.Messages {
				promptBuilder.WriteString(m.Content)
				for _, tc := range m.ToolCalls {
					promptBuilder.WriteString(tc.Arguments)
				}
			}
			toolNames := make([]string, 0, len(completionReq.Tools))
			for _, td := range completionReq.Tools {
				toolNames = append(toolNames, td.Name)
			}
			snapshotPayload := map[string]any{
				"step":        step,
				"prompt_hash": requestenvelope.HashPrompt(promptBuilder.String()),
				"tool_names":  toolNames,
			}
			// Only include memory_snippet when the operator has explicitly opted
			// in via SnapshotMemorySnippet=true. Omitting it by default prevents
			// PII or sensitive context from being written to forensic logs (#229).
			if r.config.SnapshotMemorySnippet && memorySnippetForSnapshot != "" {
				snapshotPayload["memory_snippet"] = memorySnippetForSnapshot
			}
			r.emit(runID, EventLLMRequestSnapshot, snapshotPayload)
		}

		llmCallStart := time.Now()
		result, err := activeProvider.Complete(context.Background(), completionReq)
		if err != nil {
			r.failRun(runID, fmt.Errorf("provider completion failed: %w", err))
			return
		}
		llmTotalDurationMs := result.TotalDurationMs
		if llmTotalDurationMs == 0 {
			// Fall back to wall-clock measurement when provider doesn't report it.
			llmTotalDurationMs = time.Since(llmCallStart).Milliseconds()
		}

		// Emit llm.response.meta after the provider call when forensic capture
		// is enabled. Captures wall-clock latency and the model version string.
		if r.config.CaptureRequestEnvelope {
			r.emit(runID, EventLLMResponseMeta, map[string]any{
				"step":          step,
				"latency_ms":    llmTotalDurationMs,
				"model_version": result.ModelVersion,
			})
		}

		result, blocked, err = r.applyPostHooks(context.Background(), runID, step, completionReq, result)
		if err != nil {
			r.failRun(runID, err)
			return
		}
		if blocked != nil {
			reason := blocked.reason
			if reason == "" {
				reason = "blocked"
			}
			r.failRun(runID, fmt.Errorf("blocked by post-message hook %s: %s", blocked.hookName, reason))
			return
		}

		accountingPayload := r.recordAccounting(runID, result, step)
		r.emit(runID, EventUsageDelta, accountingPayload)
		r.emit(runID, EventLLMTurnCompleted, map[string]any{
			"step":              step,
			"tool_calls":        len(result.ToolCalls),
			"total_duration_ms": llmTotalDurationMs,
			"ttft_ms":           result.TTFTMs,
		})

		// Causal graph: record this LLM turn with its context IDs.
		if causalBuilder != nil {
			turnID := fmt.Sprintf("turn-%d", step)
			var contextIDs []string
			for _, m := range turnMessages {
				if m.ToolCallID != "" {
					contextIDs = append(contextIDs, m.ToolCallID)
				} else if m.CorrelationID != "" {
					contextIDs = append(contextIDs, m.CorrelationID)
				} else if m.MessageID != "" {
					contextIDs = append(contextIDs, m.MessageID)
				}
			}
			causalBuilder.RecordTurn(step, turnID, contextIDs)
		}

		// Cost anomaly detection: check whether this step's cost is
		// disproportionately high relative to the rolling run average.
		if costAnomalyDetector != nil {
			var stepCost float64
			if v, ok := accountingPayload["turn_cost_usd"].(float64); ok {
				stepCost = v
			}
			if alert := costAnomalyDetector.Record(step, stepCost); alert != nil {
				r.emit(runID, EventCostAnomaly, map[string]any{
					"step":                 alert.Step,
					"anomaly_type":         string(alert.AnomalyType),
					"step_cost_usd":        alert.StepCostUSD,
					"avg_cost_usd":         alert.AvgCostUSD,
					"threshold_multiplier": alert.ThresholdMultiplier,
				})
			}
		}

		// Context window snapshot: emit per-step context window state when
		// ContextWindowSnapshotEnabled is set in RunnerConfig.
		if r.config.ContextWindowSnapshotEnabled {
			r.emitContextWindowSnapshot(runID, step, model, systemPrompt, turnMessages, result)
		}

		// Capture and emit reasoning/thinking text when opt-in is enabled.
		// Apply redaction to the reasoning text before storage and emission.
		capturedReasoning := ""
		if r.config.CaptureReasoning && result.ReasoningText != "" {
			capturedReasoning = result.ReasoningText
			if r.config.RedactionPipeline != nil {
				redacted, keep := r.config.RedactionPipeline.Apply(
					string(EventReasoningComplete),
					map[string]any{"text": capturedReasoning},
				)
				if keep {
					if t, ok := redacted["text"].(string); ok {
						capturedReasoning = t
					}
				} else {
					capturedReasoning = ""
				}
			}
			if capturedReasoning != "" {
				r.emit(runID, EventReasoningComplete, map[string]any{
					"text":   capturedReasoning,
					"tokens": result.ReasoningTokens,
					"step":   step,
				})
			}
		}

		// Check per-run cost ceiling after each LLM turn.
		// Only enforced when cost data is available (priced model).
		if r.exceedsCostCeiling(runID) {
			_, costTotals := r.accountingTotals(runID)
			r.mu.RLock()
			maxCost := r.runs[runID].maxCostUSD
			r.mu.RUnlock()
			r.emit(runID, EventRunCostLimitReached, map[string]any{
				"step":                step,
				"max_cost_usd":        maxCost,
				"cumulative_cost_usd": costTotals.CostUSDTotal,
			})
			r.observeMemory(runID, step, messages)
			r.emit(runID, EventRunStepCompleted, map[string]any{
				"step":        step,
				"tool_calls":  0,
				"duration_ms": time.Since(stepStartTime).Milliseconds(),
			})
			emitCausalGraph(step)
			r.completeRun(runID, result.Content)
			return
		}

		// Re-read messages from state so that any concurrent CompactRun()
		// results take effect before we append the LLM response (#232).
		// messagesForStep holds compactMu only for the brief slice copy and
		// releases it before returning, so there is no lock-order inversion.
		r.mu.RLock()
		stepState, stepOk := r.runs[runID]
		r.mu.RUnlock()
		if !stepOk {
			return
		}
		messages = r.messagesForStep(stepState)

		if len(result.ToolCalls) == 0 {
			// Detect empty response: no text and no tool calls.
			// This happens with Gemini 2.5 Flash thinking mode — the model
			// "thinks" for 26+ seconds then returns 0 completion_tokens with
			// an empty Content string. Inject a retry prompt instead of
			// treating this as run completion.
			if strings.TrimSpace(result.Content) == "" {
				consecutiveEmptyResponses++
				if consecutiveEmptyResponses < maxEmptyRetries {
					r.emit(runID, EventEmptyResponseRetry, map[string]any{
						"step":        step,
						"retry":       consecutiveEmptyResponses,
						"max_retries": maxEmptyRetries,
					})
					// Inject an assistant + user message pair so the model
					// has context about the empty response and will try again.
					messages = append(messages,
						Message{Role: "assistant", Content: ""},
						Message{
							Role:    "user",
							Content: "Your previous response was empty — no text and no tool calls. Please use the available tools to make progress on the task. What do you need to do next?",
						},
					)
					r.setMessages(runID, messages)
					r.emit(runID, EventRunStepCompleted, map[string]any{
						"step":        step,
						"tool_calls":  0,
						"duration_ms": time.Since(stepStartTime).Milliseconds(),
					})
					emitCausalGraph(step)
					continue
				}
				// Max retries exhausted — fall through to normal completion.
			} else {
				// Non-empty content resets the consecutive counter.
				consecutiveEmptyResponses = 0
			}

			if result.Content != "" {
				messages = append(messages, Message{
					Role:      "assistant",
					Content:   result.Content,
					Reasoning: capturedReasoning,
				})
			}
			r.setMessages(runID, messages)
			if result.Content != "" {
				r.snapshotRecordMessage(runID, "assistant", result.Content)
				r.emit(runID, EventAssistantMessage, map[string]any{"content": result.Content})
			}
			r.observeMemory(runID, step, messages)
			r.emit(runID, EventRunStepCompleted, map[string]any{
				"step":        step,
				"tool_calls":  0,
				"duration_ms": time.Since(stepStartTime).Milliseconds(),
			})
			emitCausalGraph(step)
			r.completeRun(runID, result.Content)
			return
		}

		// Non-empty tool calls: reset the consecutive empty response counter.
		consecutiveEmptyResponses = 0

		messages = append(messages, Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: append([]ToolCall(nil), result.ToolCalls...),
			Reasoning: capturedReasoning,
		})
		r.setMessages(runID, messages)
		r.snapshotRecordMessage(runID, "assistant", result.Content)

		// Forensics Part 1: emit tool.decision event when tracing is enabled.
		if r.config.TraceToolDecisions && len(result.ToolCalls) > 0 {
			callSeq++
			availableTools := make([]string, 0, len(completionReq.Tools))
			for _, td := range completionReq.Tools {
				availableTools = append(availableTools, td.Name)
			}
			selectedTools := make([]string, 0, len(result.ToolCalls))
			for _, tc := range result.ToolCalls {
				selectedTools = append(selectedTools, tc.Name)
			}
			snap := tooldecision.ToolDecisionSnapshot{
				Step:           step,
				CallSequence:   callSeq,
				AvailableTools: availableTools,
				SelectedTools:  selectedTools,
			}
			r.emit(runID, EventToolDecision, map[string]any{
				"step":            snap.Step,
				"call_sequence":   snap.CallSequence,
				"call_sequence_id": snap.CallSequenceID(),
				"available_tools": snap.AvailableTools,
				"selected_tools":  snap.SelectedTools,
			})
		}

		for _, call := range result.ToolCalls {
			r.emit(runID, EventToolCallStarted, map[string]any{
				"call_id":   call.ID,
				"tool":      call.Name,
				"arguments": call.Arguments,
			})

			// Causal graph: record this tool call.
			if causalBuilder != nil {
				causalBuilder.RecordToolCall(step, call.ID, call.Name, call.Arguments)
			}

			// Audit trail: emit audit.action for state-modifying tool calls.
			if r.config.AuditTrailEnabled && audittrail.IsStateModifying(call.Name) {
				auditPayload := map[string]any{
					"tool":      call.Name,
					"call_id":   call.ID,
					"arguments": call.Arguments,
					"step":      step,
				}
				r.emit(runID, EventAuditAction, auditPayload)
				r.writeAudit(runID, audittrail.AuditRecord{
					RunID:     runID,
					EventType: string(EventAuditAction),
					Payload:   auditPayload,
				})
			}

			// Forensics Part 2: anti-pattern detection.
			if r.config.DetectAntiPatterns {
				apKey := call.Name + "\x00" + call.Arguments
				antiPatternCounts[apKey]++
				count := antiPatternCounts[apKey]
				if count >= 3 && !alreadyAlerted[apKey] {
					alreadyAlerted[apKey] = true
					alert := tooldecision.AntiPatternAlert{
						Type:      tooldecision.AntiPatternRetryLoop,
						ToolName:  call.Name,
						CallCount: count,
						Step:      step,
					}
					r.emit(runID, EventToolAntiPattern, map[string]any{
						"type":       string(alert.Type),
						"tool":       alert.ToolName,
						"call_count": alert.CallCount,
						"step":       alert.Step,
					})
				}
			}

			// Check skill constraint before executing tool
			if !r.skillConstraints.IsToolAllowed(runID, call.Name) {
				constraint, _ := r.skillConstraints.Active(runID)
				constraintSkillName := ""
				var constraintAllowed []string
				if constraint != nil {
					constraintSkillName = constraint.SkillName
					constraintAllowed = constraint.AllowedTools
				}
				toolOutput := mustJSON(map[string]any{
					"error": fmt.Sprintf(
						"tool %q is not allowed while skill %q is active",
						call.Name, constraintSkillName,
					),
					"allowed_tools": constraintAllowed,
				})
				r.emit(runID, EventToolCallBlocked, map[string]any{
					"call_id": call.ID,
					"tool":    call.Name,
					"skill":   constraintSkillName,
					"reason":  "not_in_allowed_tools",
				})
				messages = append(messages, Message{
					Role:       "tool",
					Name:       call.Name,
					ToolCallID: call.ID,
					Content:    toolOutput,
				})
				r.setMessages(runID, messages)
				continue
			}

			waitingForUser := false
			if call.Name == htools.AskUserQuestionToolName {
				questions, err := htools.ParseAskUserQuestionArgs(json.RawMessage(call.Arguments))
				if err == nil {
					waitingForUser = true
					deadlineAt := time.Now().UTC().Add(r.config.AskUserTimeout)
					r.setStatus(runID, RunStatusWaitingForUser, "", "")
					r.emit(runID, EventRunWaitingForUser, map[string]any{
						"call_id":     call.ID,
						"tool":        call.Name,
						"questions":   questions,
						"deadline_at": deadlineAt,
					})
				}
			}

			// Apply pre-tool-use hooks; may deny execution or modify args.
			callArgs := json.RawMessage(call.Arguments)
			if denied, denialOutput := r.applyPreToolUseHooks(context.Background(), runID, call, &callArgs); denied {
				messages = append(messages, Message{
					Role:       "tool",
					Name:       call.Name,
					ToolCallID: call.ID,
					Content:    denialOutput,
				})
				r.setMessages(runID, messages)
				continue
			}

			meta := r.runMetadata(runID)
			toolCtx := context.WithValue(context.Background(), htools.ContextKeyRunID, runID)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyToolCallID, call.ID)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyRunMetadata, meta)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyTranscriptReader, runTranscriptReader{runner: r, runID: runID})
			callID := call.ID
			callName := call.Name
			var streamIndex atomic.Int64
			toolStep := step // capture step at creation time
			outputStreamer := func(chunk string) {
				idx := int(streamIndex.Add(1) - 1)
				r.emit(runID, EventToolOutputDelta, map[string]any{
					"step":         toolStep,
					"call_id":      callID,
					"tool":         callName,
					"stream_index": idx,
					"content":      chunk,
				})
			}
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyOutputStreamer, outputStreamer)
			// Inject message replacer so compact_history can swap the in-flight messages.
			// Capture the pre-compact message list for token count enrichment.
			preCompactMessages := messages
			messageReplacer := func(replacedMaps []map[string]any) {
				compactStart := time.Now()
				replaced := make([]Message, 0, len(replacedMaps))
				for _, m := range replacedMaps {
					msg := Message{}
					if v, ok := m["role"].(string); ok {
						msg.Role = v
					}
					if v, ok := m["content"].(string); ok {
						msg.Content = v
					}
					if v, ok := m["name"].(string); ok {
						msg.Name = v
						if v == "compact_summary" {
							msg.IsCompactSummary = true
						}
					}
					if v, ok := m["tool_call_id"].(string); ok {
						msg.ToolCallID = v
					}
					replaced = append(replaced, msg)
				}
				messages = replaced
				r.setMessages(runID, messages)

				// Build compaction event payload; enrich with token counts when
				// ContextWindowSnapshotEnabled is set (estimates labeled as such).
				compactPayload := map[string]any{
					"message_count": len(replaced),
					"duration_ms":   time.Since(compactStart).Milliseconds(),
				}
				if r.config.ContextWindowSnapshotEnabled {
					var beforeTokens, afterTokens int
					for _, m := range preCompactMessages {
						beforeTokens += contextwindow.EstimateTokens(m.Content)
					}
					for _, m := range replaced {
						afterTokens += contextwindow.EstimateTokens(m.Content)
					}
					compactPayload["before_tokens"] = beforeTokens
					compactPayload["after_tokens"] = afterTokens
					compactPayload["tokens_estimated"] = true
				}
				r.emit(runID, EventCompactHistoryCompleted, compactPayload)
			}
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyMessageReplacer, messageReplacer)
			toolStart := time.Now()
			toolOutput, toolErr := r.tools.Execute(toolCtx, call.Name, callArgs)
			toolDuration := time.Since(toolStart)

			// Check for meta-messages in tool output (enriched result envelope)
			var metaMessages []htools.MetaMessage
			if toolErr == nil {
				if tr, ok := htools.UnwrapToolResult(toolOutput); ok {
					toolOutput = tr.Output
					metaMessages = tr.MetaMessages
				}
			}

			// Apply post-tool-use hooks; may modify the result.
			// For error results, the raw output passed to hooks is empty; the
			// final error JSON is built afterward unless a hook provides a ModifiedResult.
			hookOutput := r.applyPostToolUseHooks(context.Background(), runID, call, callArgs, toolOutput, toolDuration, toolErr)

			if toolErr != nil {
				// Use the hook-modified output if provided; otherwise default to the
				// standard error JSON envelope.
				if hookOutput != "" {
					toolOutput = hookOutput
				} else {
					toolOutput = mustJSON(map[string]any{"error": toolErr.Error()})
				}
				r.emit(runID, EventToolCallCompleted, map[string]any{
					"call_id":     call.ID,
					"tool":        call.Name,
					"error":       toolErr.Error(),
					"output":      toolOutput,
					"duration_ms": toolDuration.Milliseconds(),
				})
				if waitingForUser && htools.IsAskUserQuestionTimeout(toolErr) {
					r.failRun(runID, toolErr)
					return
				}
				if waitingForUser {
					r.setStatus(runID, RunStatusRunning, "", "")
				}
			} else {
				// Use the hook output (may be original or modified by a post hook)
				toolOutput = hookOutput
				r.emit(runID, EventToolCallCompleted, map[string]any{
					"call_id":     call.ID,
					"tool":        call.Name,
					"output":      toolOutput,
					"duration_ms": toolDuration.Milliseconds(),
				})
				if waitingForUser {
					r.setStatus(runID, RunStatusRunning, "", "")
					r.emit(runID, EventRunResumed, map[string]any{
						"call_id":     call.ID,
						"tool":        call.Name,
						"answered_at": time.Now().UTC(),
					})
				}
			}

			// Check if the skill tool was invoked and activate constraints
			if call.Name == "skill" && toolErr == nil {
				r.maybeActivateSkillConstraint(runID, toolOutput)
			}

			// Handle context reset: if the reset_context tool succeeded and returned
			// the sentinel payload, perform the reset before appending the tool result.
			if toolErr == nil {
				if persist, isReset := htools.IsResetContextResult(call.Name, toolOutput); isReset {
					// Increment resetIndex under the run lock.
					r.mu.Lock()
					var resetIdx int
					if state, ok := r.runs[runID]; ok {
						resetIdx = state.resetIndex
						state.resetIndex++
					}
					r.mu.Unlock()

					// Write persist payload to observational memory.
					if r.config.MemoryManager != nil && r.config.MemoryManager.Mode() != om.ModeOff {
						persistContent := string(persist)
						if persistContent == "" || persistContent == "null" {
							persistContent = "{}"
						}
						memMsg := om.TranscriptMessage{
							Index:   int64(step),
							Role:    "system",
							Name:    "context_reset",
							Content: "[context_reset] persist: " + persistContent,
						}
						_, _ = r.config.MemoryManager.Observe(context.Background(), om.ObserveRequest{
							Scope:      r.scopeKey(runID),
							RunID:      runID,
							ToolCallID: call.ID,
							Messages:   []om.TranscriptMessage{memMsg},
						})
					}

					// Record in the optional ContextResetStore.
					if r.config.ContextResetStore != nil {
						_ = r.config.ContextResetStore.RecordContextReset(
							context.Background(), runID, resetIdx, step, persist,
						)
					}

					// Emit the context.reset SSE event.
					r.emit(runID, EventContextReset, map[string]any{
						"reset_index": resetIdx,
						"at_step":     step,
						"persist":     persist,
					})

					// Build the opening user message for the new segment.
					var persistPretty string
					if formatted, err := json.MarshalIndent(persist, "", "  "); err == nil {
						persistPretty = string(formatted)
					} else {
						persistPretty = string(persist)
					}
					openingContent := fmt.Sprintf(
						"[Context Reset — Segment %d of this run]\n\nYou previously reset your context. Here is what you carried forward:\n\n%s\n\nContinue from here.",
						resetIdx+1,
						persistPretty,
					)
					resetMessages := []Message{
						{Role: "user", Content: openingContent},
					}
					messages = resetMessages
					r.setMessages(runID, messages)

					// Skip appending the tool result to messages — the transcript is now reset.
					continue
				}
			}

			// Record the tool call in the snapshot builder (no-op when ErrorChainEnabled is false).
			{
				errMsg := ""
				if toolErr != nil {
					errMsg = toolErr.Error()
				}
				r.snapshotRecordToolCall(runID, call.Name, call.ID, call.Arguments, errMsg)
			}

			// Causal graph: record tool result for data-flow analysis.
			if causalBuilder != nil {
				causalBuilder.RecordToolResult(step, call.ID, toolOutput)
			}

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
			}

			r.setMessages(runID, messages)

			for _, metaMsg := range metaMessages {
				r.emit(runID, EventMetaMessageInjected, map[string]any{
					"call_id": call.ID,
					"tool":    call.Name,
					"length":  len(metaMsg.Content),
				})
			}
		}
		r.observeMemory(runID, step, messages)
		r.emit(runID, EventRunStepCompleted, map[string]any{
			"step":        step,
			"tool_calls":  len(result.ToolCalls),
			"duration_ms": time.Since(stepStartTime).Milliseconds(),
		})
	}

	emitCausalGraph(effectiveMaxSteps)
	r.failRunMaxSteps(runID, effectiveMaxSteps)
}

type hookBlock struct {
	hookName string
	reason   string
}

func (r *Runner) applyPreHooks(ctx context.Context, runID string, step int, req CompletionRequest) (CompletionRequest, *hookBlock, error) {
	current := req
	for _, hook := range r.config.PreMessageHooks {
		hookName := normalizeHookName(hook.Name())
		r.emit(runID, EventHookStarted, map[string]any{
			"stage": "pre_message",
			"hook":  hookName,
			"step":  step,
		})

		hookStart := time.Now()
		result, err := hook.BeforeMessage(ctx, PreMessageHookInput{
			RunID:   runID,
			Step:    step,
			Request: current,
		})
		if err != nil {
			ignored := r.config.HookFailureMode == HookFailureModeFailOpen
			r.emit(runID, EventHookFailed, map[string]any{
				"stage":   "pre_message",
				"hook":    hookName,
				"step":    step,
				"error":   err.Error(),
				"mode":    r.config.HookFailureMode,
				"ignored": ignored,
			})
			if ignored {
				continue
			}
			return current, nil, fmt.Errorf("pre-message hook %s failed: %w", hookName, err)
		}

		action := result.Action
		if action == "" {
			action = HookActionContinue
		}
		mutated := false
		if result.MutatedRequest != nil {
			current = *result.MutatedRequest
			mutated = true
		}

		r.emit(runID, EventHookCompleted, map[string]any{
			"stage":       "pre_message",
			"hook":        hookName,
			"step":        step,
			"action":      action,
			"mutated":     mutated,
			"reason":      result.Reason,
			"duration_ms": time.Since(hookStart).Milliseconds(),
		})

		if action == HookActionBlock {
			return current, &hookBlock{hookName: hookName, reason: result.Reason}, nil
		}
	}
	return current, nil, nil
}

func (r *Runner) applyPostHooks(ctx context.Context, runID string, step int, req CompletionRequest, res CompletionResult) (CompletionResult, *hookBlock, error) {
	current := res
	for _, hook := range r.config.PostMessageHooks {
		hookName := normalizeHookName(hook.Name())
		r.emit(runID, EventHookStarted, map[string]any{
			"stage": "post_message",
			"hook":  hookName,
			"step":  step,
		})

		hookStart := time.Now()
		result, err := hook.AfterMessage(ctx, PostMessageHookInput{
			RunID:     runID,
			Step:      step,
			Request:   req,
			Response:  current,
			ToolCalls: current.ToolCalls,
		})
		if err != nil {
			ignored := r.config.HookFailureMode == HookFailureModeFailOpen
			r.emit(runID, EventHookFailed, map[string]any{
				"stage":   "post_message",
				"hook":    hookName,
				"step":    step,
				"error":   err.Error(),
				"mode":    r.config.HookFailureMode,
				"ignored": ignored,
			})
			if ignored {
				continue
			}
			return current, nil, fmt.Errorf("post-message hook %s failed: %w", hookName, err)
		}

		action := result.Action
		if action == "" {
			action = HookActionContinue
		}
		mutated := false
		if result.MutatedResponse != nil {
			current = *result.MutatedResponse
			mutated = true
		}

		r.emit(runID, EventHookCompleted, map[string]any{
			"stage":       "post_message",
			"hook":        hookName,
			"step":        step,
			"action":      action,
			"mutated":     mutated,
			"reason":      result.Reason,
			"duration_ms": time.Since(hookStart).Milliseconds(),
		})

		if action == HookActionBlock {
			return current, &hookBlock{hookName: hookName, reason: result.Reason}, nil
		}
	}
	return current, nil, nil
}

func normalizeHookName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "unnamed_hook"
	}
	return trimmed
}

// applyPreToolUseHooks runs all registered PreToolUseHooks for a given tool call.
//
// It returns (denied=true, errorOutput) if any hook denies execution.
// If denied=false, callArgs may have been modified in place by a hook.
//
// Panic recovery: a panicking hook is caught and treated as a hook error.
// - fail_open:   panic is recovered, hook is skipped, execution continues.
// - fail_closed: panic is recovered, tool is denied with an error output.
func (r *Runner) applyPreToolUseHooks(ctx context.Context, runID string, call ToolCall, callArgs *json.RawMessage) (denied bool, denialOutput string) {
	if len(r.config.PreToolUseHooks) == 0 {
		return false, ""
	}

	for _, hook := range r.config.PreToolUseHooks {
		hookName := normalizeHookName(hook.Name())
		r.emit(runID, EventToolHookStarted, map[string]any{
			"stage":   "pre_tool_use",
			"hook":    hookName,
			"tool":    call.Name,
			"call_id": call.ID,
		})

		// Forensics Part 3: capture args before hook runs for mutation tracing.
		argsBefore := ""
		if r.config.TraceHookMutations {
			argsBefore = string(*callArgs)
		}

		result, err := safeCallPreToolUseHook(hook, ctx, PreToolUseEvent{
			ToolName: call.Name,
			CallID:   call.ID,
			Args:     append(json.RawMessage(nil), *callArgs...),
			RunID:    runID,
		})

		if err != nil {
			ignored := r.config.HookFailureMode == HookFailureModeFailOpen
			r.emit(runID, EventToolHookFailed, map[string]any{
				"stage":   "pre_tool_use",
				"hook":    hookName,
				"tool":    call.Name,
				"call_id": call.ID,
				"error":   err.Error(),
				"ignored": ignored,
			})
			if r.config.TraceHookMutations {
				// Hook error in fail_closed mode counts as an implicit block.
				if !ignored {
					mutation := tooldecision.HookMutation{
						ToolCallID: call.ID,
						HookName:   hookName,
						Action:     tooldecision.HookActionBlock,
						ArgsBefore: argsBefore,
					}
					r.emit(runID, EventToolHookMutation, map[string]any{
						"tool_call_id": mutation.ToolCallID,
						"hook":         mutation.HookName,
						"action":       string(mutation.Action),
						"args_before":  mutation.ArgsBefore,
						"args_after":   mutation.ArgsAfter,
					})
				}
			}
			if ignored {
				continue
			}
			// fail_closed: deny the tool call with an error result
			return true, mustJSON(map[string]any{
				"error": fmt.Sprintf("pre_tool_use hook %s failed: %v", hookName, err),
			})
		}

		// nil result is treated as allow with no modification
		if result == nil {
			r.emit(runID, EventToolHookCompleted, map[string]any{
				"stage":    "pre_tool_use",
				"hook":     hookName,
				"tool":     call.Name,
				"call_id":  call.ID,
				"decision": "allow",
				"mutated":  false,
			})
			continue
		}

		mutated := false
		if len(result.ModifiedArgs) > 0 {
			*callArgs = append(json.RawMessage(nil), result.ModifiedArgs...)
			mutated = true
		}

		decision := "allow"
		if result.Decision == ToolHookDeny {
			decision = "deny"
		}

		r.emit(runID, EventToolHookCompleted, map[string]any{
			"stage":    "pre_tool_use",
			"hook":     hookName,
			"tool":     call.Name,
			"call_id":  call.ID,
			"decision": decision,
			"reason":   result.Reason,
			"mutated":  mutated,
		})

		// Forensics Part 3: emit hook mutation event when tracing is enabled.
		if r.config.TraceHookMutations {
			argsAfter := string(*callArgs)
			blocked := result.Decision == ToolHookDeny
			action := tooldecision.ClassifyHookAction(blocked, argsBefore, argsAfter)
			// Only emit when something interesting happened (not a plain Allow).
			if action != tooldecision.HookActionAllow {
				mutation := tooldecision.HookMutation{
					ToolCallID: call.ID,
					HookName:   hookName,
					Action:     action,
					ArgsBefore: argsBefore,
					ArgsAfter:  argsAfter,
				}
				r.emit(runID, EventToolHookMutation, map[string]any{
					"tool_call_id": mutation.ToolCallID,
					"hook":         mutation.HookName,
					"action":       string(mutation.Action),
					"args_before":  mutation.ArgsBefore,
					"args_after":   mutation.ArgsAfter,
				})
			}
		}

		if result.Decision == ToolHookDeny {
			reason := result.Reason
			if reason == "" {
				reason = "denied by hook"
			}
			return true, mustJSON(map[string]any{
				"error": fmt.Sprintf("tool %q denied by hook %s: %s", call.Name, hookName, reason),
			})
		}
	}
	return false, ""
}

// applyPostToolUseHooks runs all registered PostToolUseHooks after a tool executes.
//
// It returns the (possibly modified) tool output. If toolErr is non-nil,
// the output passed to hooks will be the empty string and toolErr will be set
// in the event; the original error output is still returned to the LLM
// (hooks can override it via ModifiedResult).
//
// Panic recovery mirrors pre-tool-use hook behaviour.
func (r *Runner) applyPostToolUseHooks(ctx context.Context, runID string, call ToolCall, callArgs json.RawMessage, output string, duration time.Duration, toolErr error) string {
	if len(r.config.PostToolUseHooks) == 0 {
		return output
	}

	// For error results, pass the empty string as result (the JSON error
	// output is constructed after hooks run).
	rawResult := output
	if toolErr != nil {
		rawResult = ""
	}

	current := output
	for _, hook := range r.config.PostToolUseHooks {
		hookName := normalizeHookName(hook.Name())
		r.emit(runID, EventToolHookStarted, map[string]any{
			"stage":   "post_tool_use",
			"hook":    hookName,
			"tool":    call.Name,
			"call_id": call.ID,
		})

		result, err := safeCallPostToolUseHook(hook, ctx, PostToolUseEvent{
			ToolName: call.Name,
			CallID:   call.ID,
			Args:     append(json.RawMessage(nil), callArgs...),
			Result:   rawResult,
			Duration: duration,
			Error:    toolErr,
			RunID:    runID,
		})

		if err != nil {
			ignored := r.config.HookFailureMode == HookFailureModeFailOpen
			r.emit(runID, EventToolHookFailed, map[string]any{
				"stage":   "post_tool_use",
				"hook":    hookName,
				"tool":    call.Name,
				"call_id": call.ID,
				"error":   err.Error(),
				"ignored": ignored,
			})
			if !ignored {
				// fail_closed: stop the chain and return current output unchanged
				return current
			}
			continue
		}

		mutated := false
		if result != nil && result.ModifiedResult != "" {
			current = result.ModifiedResult
			rawResult = result.ModifiedResult
			mutated = true
		}

		r.emit(runID, EventToolHookCompleted, map[string]any{
			"stage":   "post_tool_use",
			"hook":    hookName,
			"tool":    call.Name,
			"call_id": call.ID,
			"mutated": mutated,
		})
	}
	return current
}

// safeCallPreToolUseHook calls hook.PreToolUse and recovers from panics,
// returning the panic as an error.
func safeCallPreToolUseHook(hook PreToolUseHook, ctx context.Context, ev PreToolUseEvent) (result *PreToolUseResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("hook panic: %v", r)
		}
	}()
	return hook.PreToolUse(ctx, ev)
}

// safeCallPostToolUseHook calls hook.PostToolUse and recovers from panics,
// returning the panic as an error.
func safeCallPostToolUseHook(hook PostToolUseHook, ctx context.Context, ev PostToolUseEvent) (result *PostToolUseResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("hook panic: %v", r)
		}
	}()
	return hook.PostToolUse(ctx, ev)
}

// filteredToolsForRun returns tool definitions for a run, applying skill
// constraints on top of the deferred-tool activation filter. If no skill
// constraint is active, or if the constraint has nil AllowedTools, all
// definitions from DefinitionsForRun are returned.
func (r *Runner) filteredToolsForRun(runID string) []ToolDefinition {
	defs := r.tools.DefinitionsForRun(runID, r.activations)

	// Skill constraints (activated by the skill tool) take precedence over the
	// per-run base filter. If a skill constraint is active with a non-nil
	// AllowedTools list, apply it exclusively.
	constraint, active := r.skillConstraints.Active(runID)
	if active && constraint.AllowedTools != nil {
		allowed := make(map[string]bool, len(constraint.AllowedTools)+len(AlwaysAvailableTools))
		for _, name := range constraint.AllowedTools {
			allowed[name] = true
		}
		for name := range AlwaysAvailableTools {
			allowed[name] = true
		}
		filtered := make([]ToolDefinition, 0, len(allowed))
		for _, def := range defs {
			if allowed[def.Name] {
				filtered = append(filtered, def)
			}
		}
		return filtered
	}

	// No active skill constraint (or skill constraint with nil AllowedTools =
	// unrestricted). Apply the per-run base allowed-tools list from RunRequest.
	r.mu.RLock()
	state, stateOK := r.runs[runID]
	var baseAllowed []string
	if stateOK {
		baseAllowed = state.allowedTools
	}
	r.mu.RUnlock()

	if len(baseAllowed) == 0 {
		return defs // no per-run restriction either
	}

	allowed := make(map[string]bool, len(baseAllowed)+len(AlwaysAvailableTools))
	for _, name := range baseAllowed {
		allowed[name] = true
	}
	for name := range AlwaysAvailableTools {
		allowed[name] = true
	}
	filtered := make([]ToolDefinition, 0, len(allowed))
	for _, def := range defs {
		if allowed[def.Name] {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// maybeActivateSkillConstraint inspects a skill tool result and activates
// a constraint if the result contains allowed_tools.
func (r *Runner) maybeActivateSkillConstraint(runID, resultJSON string) {
	var result struct {
		Skill        string   `json:"skill"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return // not a valid skill result
	}
	if result.Skill == "" {
		return // was a "list" action, not "apply"
	}

	// Check if there was a previous constraint
	if prev, active := r.skillConstraints.Active(runID); active {
		r.emit(runID, EventSkillConstraintDeactivated, map[string]any{
			"skill":  prev.SkillName,
			"reason": "replaced_by_new_skill",
		})
	}

	constraint := SkillConstraint{
		SkillName:    result.Skill,
		AllowedTools: result.AllowedTools,
	}
	r.skillConstraints.Activate(runID, constraint)
	r.emit(runID, EventSkillConstraintActivated, map[string]any{
		"skill":         result.Skill,
		"allowed_tools": result.AllowedTools,
		"unrestricted":  result.AllowedTools == nil,
	})
}

// drainSteering reads all pending steering messages from the run's steeringCh
// and appends them as user messages to the transcript. A steering.received event
// is emitted for each injected message. This is called at the top of each step
// before the next LLM call.
func (r *Runner) drainSteering(runID string, messages *[]Message) {
	r.mu.RLock()
	state, ok := r.runs[runID]
	r.mu.RUnlock()
	if !ok {
		return
	}
	for {
		select {
		case msg := <-state.steeringCh:
			*messages = append(*messages, Message{Role: "user", Content: msg})
			r.snapshotRecordMessage(runID, "user", msg)
			r.setMessages(runID, *messages)
			r.emit(runID, EventSteeringReceived, map[string]any{"message": msg})
		default:
			return
		}
	}
}

func (r *Runner) completeRun(runID, output string) {
	r.setStatus(runID, RunStatusCompleted, output, "")

	// Clean up deferred tool activations for this run
	r.activations.Cleanup(runID)

	// Clean up skill constraints for this run
	r.skillConstraints.Cleanup(runID)

	// Clean up per-run MCP servers
	r.closeScopedMCP(runID)

	// Store conversation messages for multi-turn support
	r.mu.RLock()
	state, ok := r.runs[runID]
	if ok {
		convID := state.run.ConversationID
		tenantID := state.run.TenantID
		agentID := state.run.AgentID
		msgs := copyMessages(state.messages)
		r.mu.RUnlock()

		r.mu.Lock()
		r.conversations[convID] = msgs
		// Record ownership so that future StartRun callers with the same
		// ConversationID can be validated against the originating tenant+agent
		// (cross-tenant/cross-agent disclosure prevention, issue #221).
		r.conversationOwners[convID] = conversationOwner{
			tenantID: tenantID,
			agentID:  agentID,
		}
		r.mu.Unlock()

		// Persist to SQLite store if configured
		if r.config.ConversationStore != nil {
			storeMsgs := copyMessages(msgs) // defensive clone for untrusted store boundary
			usageTotals, costTotals := r.accountingTotals(runID)
			tokenCost := ConversationTokenCost{
				PromptTokens:     usageTotals.PromptTokensTotal,
				CompletionTokens: usageTotals.CompletionTokensTotal,
				CostUSD:          costTotals.CostUSDTotal,
			}
			if err := r.config.ConversationStore.SaveConversationWithCost(context.Background(), convID, storeMsgs, tokenCost); err != nil {
				if r.config.Logger != nil {
					r.config.Logger.Error("failed to persist conversation", "conv_id", convID, "error", err)
				}
			} else {
				// Wire tenant scoping: set workspace and tenant_id on the conversation row.
				if tenantID == "default" {
					tenantID = ""
				}
				if tenantID != "" {
					if err := r.config.ConversationStore.UpdateConversationMeta(context.Background(), convID, "", tenantID); err != nil {
						if r.config.Logger != nil {
							r.config.Logger.Error("failed to update conversation meta", "conv_id", convID, "error", err)
						}
					}
				}
			}
		}
	} else {
		r.mu.RUnlock()
	}

	usageTotals, costTotals := r.accountingTotals(runID)
	r.emit(runID, EventRunCompleted, map[string]any{
		"output":       output,
		"usage_totals": usageTotals,
		"cost_totals":  costTotals,
	})

	// Audit trail: write run.completed and close the writer.
	if r.config.AuditTrailEnabled {
		r.writeAudit(runID, audittrail.AuditRecord{
			RunID:     runID,
			EventType: string(EventRunCompleted),
			Payload:   map[string]any{"status": "completed"},
		})
		r.closeAuditWriter(runID)
	}
}

// closeScopedMCP closes the per-run scoped MCP registry if one was configured
// for this run. It is a no-op when no scoped registry exists.
func (r *Runner) closeScopedMCP(runID string) {
	r.mu.RLock()
	state, ok := r.runs[runID]
	var scoped *ScopedMCPRegistry
	if ok {
		scoped = state.scopedMCPRegistry
	}
	r.mu.RUnlock()
	if scoped != nil {
		_ = scoped.Close()
	}
}

// snapshotRecordToolCall records a tool call in the run's snapshot builder when
// ErrorChainEnabled is set. It is a no-op when ErrorChainEnabled is false.
func (r *Runner) snapshotRecordToolCall(runID, name, callID, args, errMsg string) {
	if !r.config.ErrorChainEnabled {
		return
	}
	r.mu.RLock()
	state, ok := r.runs[runID]
	var sb *errorchain.SnapshotBuilder
	if ok {
		sb = state.snapshotBuilder
	}
	r.mu.RUnlock()
	if sb != nil {
		sb.RecordToolCall(name, callID, args, errMsg)
	}
}

// snapshotRecordMessage records a message in the run's snapshot builder when
// ErrorChainEnabled is set. It is a no-op when ErrorChainEnabled is false.
func (r *Runner) snapshotRecordMessage(runID, role, content string) {
	if !r.config.ErrorChainEnabled {
		return
	}
	r.mu.RLock()
	state, ok := r.runs[runID]
	var sb *errorchain.SnapshotBuilder
	if ok {
		sb = state.snapshotBuilder
	}
	r.mu.RUnlock()
	if sb != nil {
		sb.RecordMessage(role, content)
	}
}

// emitContextWindowSnapshot emits a context.window.snapshot event after an LLM
// turn. It builds a token breakdown using the rune/4 estimation heuristic for
// all fields except the provider-reported prompt token count (when available).
//
// The maxContextTokens is resolved in priority order:
//  1. Provider catalog (via providerRegistry.MaxContextTokens) if the registry is set.
//  2. RunnerConfig.ModelContextWindow fallback.
//
// A context.window.warning event is also emitted when ContextWindowWarningThreshold
// is non-zero and the usage ratio exceeds the threshold.
func (r *Runner) emitContextWindowSnapshot(
	runID string,
	step int,
	model string,
	systemPromptText string,
	turnMessages []Message,
	result CompletionResult,
) {
	// Determine max context tokens: prefer catalog, fall back to config.
	maxCtxTokens := r.config.ModelContextWindow
	if r.providerRegistry != nil {
		if catalogMax, ok := r.providerRegistry.MaxContextTokens(model); ok && catalogMax > 0 {
			maxCtxTokens = catalogMax
		}
	}

	// Extract provider-reported prompt token count from the result.
	providerPromptTokens := 0
	providerReported := false
	if result.Usage != nil && result.Usage.PromptTokens > 0 {
		providerPromptTokens = result.Usage.PromptTokens
		providerReported = result.UsageStatus == UsageStatusProviderReported || result.UsageStatus == ""
	}

	// Build message list for estimation (exclude system messages from turnMessages
	// since we handle systemPromptText separately).
	msgs := make([]contextwindow.MessageForEstimate, 0, len(turnMessages))
	for _, m := range turnMessages {
		if m.Role == "system" {
			// System messages (including memory snippets injected as system)
			// are counted in system prompt tokens.
			systemPromptText += " " + m.Content
			continue
		}
		msgs = append(msgs, contextwindow.MessageForEstimate{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	snap := contextwindow.BuildSnapshot(
		step,
		systemPromptText,
		msgs,
		providerPromptTokens,
		providerReported,
		maxCtxTokens,
	)

	r.emit(runID, EventContextWindowSnapshot, contextwindow.SnapshotToPayload(snap))

	// Emit warning when threshold is configured and usage exceeds it.
	if r.config.ContextWindowWarningThreshold > 0 && snap.UsageRatio >= r.config.ContextWindowWarningThreshold {
		tokensUsed := snap.EstimatedTotalTokens
		if snap.ProviderReported {
			tokensUsed = snap.ProviderReportedTokens
		}
		r.emit(runID, EventContextWindowWarning, map[string]any{
			"step":               step,
			"usage_ratio":        snap.UsageRatio,
			"threshold":          r.config.ContextWindowWarningThreshold,
			"provider_reported":  snap.ProviderReported,
			"tokens_used":        tokensUsed,
			"max_context_tokens": maxCtxTokens,
		})
	}
}

func (r *Runner) failRun(runID string, err error) {
	if err == nil {
		err = errors.New("run failed")
	}
	r.setStatus(runID, RunStatusFailed, "", err.Error())

	// Clean up deferred tool activations for this run
	r.activations.Cleanup(runID)

	// Clean up skill constraints for this run
	r.skillConstraints.Cleanup(runID)

	// Clean up per-run MCP servers
	r.closeScopedMCP(runID)

	// Emit error.context before run.failed when ErrorChainEnabled.
	if r.config.ErrorChainEnabled {
		r.mu.RLock()
		state, ok := r.runs[runID]
		var sb *errorchain.SnapshotBuilder
		if ok {
			sb = state.snapshotBuilder
		}
		r.mu.RUnlock()
		if sb != nil {
			ce := errorchain.NewChainedError(errorchain.ClassProvider, err.Error(), nil)
			payload := errorchain.BuildErrorContextPayload(ce, sb)
			r.emit(runID, EventErrorContext, payload)
		}
	}

	usageTotals, costTotals := r.accountingTotals(runID)
	r.emit(runID, EventRunFailed, map[string]any{
		"error":        err.Error(),
		"usage_totals": usageTotals,
		"cost_totals":  costTotals,
	})

	// Audit trail: write run.failed and close the writer.
	if r.config.AuditTrailEnabled {
		r.writeAudit(runID, audittrail.AuditRecord{
			RunID:     runID,
			EventType: string(EventRunFailed),
			Payload:   map[string]any{"error": err.Error(), "status": "failed"},
		})
		r.closeAuditWriter(runID)
	}
}

// failRunMaxSteps is a specialisation of failRun used when the step loop
// exhausts its budget.  The run.failed event carries a structured
// reason="max_steps_reached" and max_steps field so clients can distinguish
// this terminal state from other failures without parsing the error string.
func (r *Runner) failRunMaxSteps(runID string, maxSteps int) {
	err := fmt.Errorf("max steps (%d) reached", maxSteps)
	r.setStatus(runID, RunStatusFailed, "", err.Error())

	// Clean up deferred tool activations for this run
	r.activations.Cleanup(runID)

	// Clean up skill constraints for this run
	r.skillConstraints.Cleanup(runID)

	// Clean up per-run MCP servers
	r.closeScopedMCP(runID)
	usageTotals, costTotals := r.accountingTotals(runID)
	r.emit(runID, EventRunFailed, map[string]any{
		"error":        err.Error(),
		"reason":       "max_steps_reached",
		"max_steps":    maxSteps,
		"usage_totals": usageTotals,
		"cost_totals":  costTotals,
	})

	// Audit trail: write run.failed and close the writer.
	if r.config.AuditTrailEnabled {
		r.writeAudit(runID, audittrail.AuditRecord{
			RunID:     runID,
			EventType: string(EventRunFailed),
			Payload: map[string]any{
				"error":   err.Error(),
				"reason":  "max_steps_reached",
				"status":  "failed",
			},
		})
		r.closeAuditWriter(runID)
	}
}

func (r *Runner) recordAccounting(runID string, result CompletionResult, step int) map[string]any {
	turnUsage, usageStatus := normalizeTurnUsage(result)
	turnCostUSD, costStatus, pricingVersion := normalizeTurnCost(result, usageStatus)

	r.mu.Lock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.Unlock()
		return map[string]any{
			"step":                step,
			"usage_status":        usageStatus,
			"cost_status":         costStatus,
			"turn_usage":          completionUsageToMap(turnUsage),
			"turn_cost_usd":       turnCostUSD,
			"cumulative_usage":    completionUsageToMap(CompletionUsage{}),
			"cumulative_cost_usd": 0.0,
			"pricing_version":     pricingVersion,
		}
	}
	state.usageTotals.add(turnUsage)
	state.costTotals.CostUSDTotal += turnCostUSD
	state.costTotals.LastTurnCostUSD = turnCostUSD
	state.costTotals.CostStatus = costStatus
	if trimmedVersion := strings.TrimSpace(pricingVersion); trimmedVersion != "" {
		state.costTotals.PricingVersion = trimmedVersion
	}
	usageTotals := state.usageTotals.runTotals()
	costTotals := state.costTotals
	cumulativeUsage := state.usageTotals.completionUsage()
	state.run.UsageTotals = &usageTotals
	state.run.CostTotals = &costTotals
	r.mu.Unlock()

	return map[string]any{
		"step":                step,
		"usage_status":        usageStatus,
		"cost_status":         costStatus,
		"turn_usage":          completionUsageToMap(turnUsage),
		"turn_cost_usd":       turnCostUSD,
		"cumulative_usage":    completionUsageToMap(cumulativeUsage),
		"cumulative_cost_usd": costTotals.CostUSDTotal,
		"pricing_version":     costTotals.PricingVersion,
	}
}

// completionUsageToMap converts a CompletionUsage struct into a map[string]any
// using its JSON representation. This breaks all pointer aliases: the resulting
// map contains only scalar values (float64 for JSON numbers) that are safe for
// insertion into event payloads distributed to multiple subscribers.
// CompletionUsage contains only numeric types so marshal cannot fail in practice.
func completionUsageToMap(u CompletionUsage) map[string]any {
	b, err := json.Marshal(u)
	if err != nil {
		return map[string]any{
			"prompt_tokens":     u.PromptTokens,
			"completion_tokens": u.CompletionTokens,
			"total_tokens":      u.TotalTokens,
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{
			"prompt_tokens":     u.PromptTokens,
			"completion_tokens": u.CompletionTokens,
			"total_tokens":      u.TotalTokens,
		}
	}
	return m
}

func normalizeTurnUsage(result CompletionResult) (CompletionUsage, UsageStatus) {
	if result.Usage == nil {
		return CompletionUsage{}, UsageStatusProviderUnreported
	}
	usage := *result.Usage
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	status := result.UsageStatus
	if status == "" {
		status = UsageStatusProviderReported
	}
	return usage, status
}

func normalizeTurnCost(result CompletionResult, usageStatus UsageStatus) (float64, CostStatus, string) {
	status := result.CostStatus
	if status == "" {
		if usageStatus == UsageStatusProviderUnreported {
			status = CostStatusProviderUnreported
		} else if result.Cost != nil || result.CostUSD != nil {
			status = CostStatusAvailable
		} else {
			status = CostStatusUnpricedModel
		}
	}
	if status != CostStatusAvailable {
		return 0, status, pricingVersionFromResult(result)
	}

	total := 0.0
	if result.Cost != nil {
		total = result.Cost.TotalUSD
	}
	if result.CostUSD != nil {
		total = *result.CostUSD
	}
	return total, status, pricingVersionFromResult(result)
}

func pricingVersionFromResult(result CompletionResult) string {
	if result.Cost == nil {
		return ""
	}
	return strings.TrimSpace(result.Cost.PricingVersion)
}

func (r *Runner) accountingTotals(runID string) (RunUsageTotals, RunCostTotals) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.runs[runID]
	if !ok {
		return RunUsageTotals{}, RunCostTotals{CostStatus: CostStatusPending}
	}
	usage := state.usageTotals.runTotals()
	cost := state.costTotals
	if cost.CostStatus == "" {
		cost.CostStatus = CostStatusPending
	}
	return usage, cost
}

// exceedsCostCeiling reports whether the cumulative cost for the given run has
// reached or exceeded the per-run cost ceiling (max_cost_usd). Returns false
// when no ceiling is set (maxCostUSD == 0) or when cost data is unavailable
// (unpriced model or provider-unreported cost).
func (r *Runner) exceedsCostCeiling(runID string) bool {
	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return false
	}
	maxCost := state.maxCostUSD
	total := state.costTotals.CostUSDTotal
	status := state.costTotals.CostStatus
	r.mu.RUnlock()

	if maxCost <= 0 {
		return false // no ceiling configured
	}
	if status != CostStatusAvailable {
		return false // cost data unavailable; don't halt on unknown costs
	}
	return total >= maxCost
}

func (a *usageTotalsAccumulator) add(turn CompletionUsage) {
	a.promptTokensTotal += turn.PromptTokens
	a.completionTokensTotal += turn.CompletionTokens
	a.totalTokens += turn.TotalTokens
	a.lastTurnTokens = turn.TotalTokens
	if turn.CachedPromptTokens != nil {
		a.cachedPromptTokens += *turn.CachedPromptTokens
		a.hasCachedPromptTokens = true
	}
	if turn.ReasoningTokens != nil {
		a.reasoningTokens += *turn.ReasoningTokens
		a.hasReasoningTokens = true
	}
	if turn.InputAudioTokens != nil {
		a.inputAudioTokens += *turn.InputAudioTokens
		a.hasInputAudioTokens = true
	}
	if turn.OutputAudioTokens != nil {
		a.outputAudioTokens += *turn.OutputAudioTokens
		a.hasOutputAudioTokens = true
	}
}

func (a usageTotalsAccumulator) runTotals() RunUsageTotals {
	return RunUsageTotals{
		PromptTokensTotal:     a.promptTokensTotal,
		CompletionTokensTotal: a.completionTokensTotal,
		TotalTokens:           a.totalTokens,
		LastTurnTokens:        a.lastTurnTokens,
	}
}

func (a usageTotalsAccumulator) completionUsage() CompletionUsage {
	out := CompletionUsage{
		PromptTokens:     a.promptTokensTotal,
		CompletionTokens: a.completionTokensTotal,
		TotalTokens:      a.totalTokens,
	}
	if a.hasCachedPromptTokens {
		n := a.cachedPromptTokens
		out.CachedPromptTokens = &n
	}
	if a.hasReasoningTokens {
		n := a.reasoningTokens
		out.ReasoningTokens = &n
	}
	if a.hasInputAudioTokens {
		n := a.inputAudioTokens
		out.InputAudioTokens = &n
	}
	if a.hasOutputAudioTokens {
		n := a.outputAudioTokens
		out.OutputAudioTokens = &n
	}
	return out
}

func (r *Runner) setStatus(runID string, status RunStatus, output, runErr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.runs[runID]
	if !ok {
		return
	}
	state.run.Status = status
	state.run.Output = output
	state.run.Error = runErr
	state.run.UpdatedAt = time.Now().UTC()
}

func (r *Runner) setMessages(runID string, messages []Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.runs[runID]
	if !ok {
		return
	}
	state.messages = copyMessages(messages)
}

// GetRunMessages returns a snapshot of the messages for the given run.
// Returns nil when the run does not exist. The returned slice is a copy
// so callers cannot mutate the stored state.
func (r *Runner) GetRunMessages(runID string) []Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.runs[runID]
	if !ok {
		return nil
	}
	return copyMessages(state.messages)
}

func (r *Runner) promptContext(runID string) (string, *systemprompt.ResolvedPrompt, time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.runs[runID]
	if !ok {
		return "", nil, time.Now().UTC()
	}
	return state.staticSystemPrompt, state.promptResolved, state.run.CreatedAt
}

func (r *Runner) scopeKey(runID string) om.ScopeKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.runs[runID]
	if !ok {
		return om.ScopeKey{TenantID: "default", ConversationID: runID, AgentID: "default"}
	}
	return om.ScopeKey{
		TenantID:       state.run.TenantID,
		ConversationID: state.run.ConversationID,
		AgentID:        state.run.AgentID,
	}
}

func (r *Runner) runMetadata(runID string) htools.RunMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.runs[runID]
	if !ok {
		return htools.RunMetadata{RunID: runID, TenantID: "default", ConversationID: runID, AgentID: "default"}
	}
	return htools.RunMetadata{
		RunID:          state.run.ID,
		TenantID:       state.run.TenantID,
		ConversationID: state.run.ConversationID,
		AgentID:        state.run.AgentID,
	}
}

func (r *Runner) transcriptSnapshot(runID string, limit int, includeTools bool) htools.TranscriptSnapshot {
	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return htools.TranscriptSnapshot{
			RunID:          runID,
			TenantID:       "default",
			ConversationID: runID,
			AgentID:        "default",
			Messages:       []htools.TranscriptMessage{},
			GeneratedAt:    time.Now().UTC(),
		}
	}
	run := state.run
	messages := append([]Message(nil), state.messages...)
	r.mu.RUnlock()

	items := make([]htools.TranscriptMessage, 0, len(messages))
	for i, msg := range messages {
		if msg.IsMeta {
			continue // meta-messages are not visible in transcripts
		}
		if !includeTools && msg.Role == "tool" {
			continue
		}
		items = append(items, htools.TranscriptMessage{
			Index:      int64(i),
			Role:       msg.Role,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
			Content:    msg.Content,
		})
	}
	if limit > 0 && len(items) > limit {
		items = append([]htools.TranscriptMessage(nil), items[len(items)-limit:]...)
	}
	return htools.TranscriptSnapshot{
		RunID:          run.ID,
		TenantID:       run.TenantID,
		ConversationID: run.ConversationID,
		AgentID:        run.AgentID,
		Messages:       items,
		GeneratedAt:    time.Now().UTC(),
	}
}

func (r *Runner) loadConversationHistory(runID string) []Message {
	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return nil
	}
	convID := state.run.ConversationID
	msgs, found := r.conversations[convID]
	if found {
		r.mu.RUnlock()
		return copyMessages(msgs)
	}
	r.mu.RUnlock()

	// Fall through to persistent store
	if r.config.ConversationStore != nil {
		loaded, err := r.config.ConversationStore.LoadMessages(context.Background(), convID)
		if err != nil {
			if r.config.Logger != nil {
				r.config.Logger.Error("failed to load conversation from store", "conv_id", convID, "error", err)
			}
			return nil
		}
		if len(loaded) > 0 {
			return copyMessages(loaded)
		}
	}
	return nil
}

func (r *Runner) conversationID(runID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.runs[runID]
	if !ok {
		return ""
	}
	return state.run.ConversationID
}

func (r *Runner) ConversationMessages(conversationID string) ([]Message, bool) {
	r.mu.RLock()
	msgs, ok := r.conversations[conversationID]
	if ok {
		r.mu.RUnlock()
		return copyMessages(msgs), true
	}
	r.mu.RUnlock()

	// Fall through to persistent store
	if r.config.ConversationStore != nil {
		loaded, err := r.config.ConversationStore.LoadMessages(context.Background(), conversationID)
		if err != nil {
			return nil, false
		}
		if len(loaded) > 0 {
			return copyMessages(loaded), true
		}
	}
	return nil, false
}

// GetConversationStore returns the configured conversation store, or nil.
func (r *Runner) GetConversationStore() ConversationStore {
	return r.config.ConversationStore
}

// RunContextStatus holds the context window status for a run.
type RunContextStatus struct {
	MessageCount    int    `json:"message_count"`
	EstimatedTokens int    `json:"estimated_tokens"`
	ContextPressure string `json:"context_pressure"`
}

// GetRunContextStatus returns the current context status for the given run.
// Returns ErrRunNotFound if the run does not exist.
func (r *Runner) GetRunContextStatus(runID string) (RunContextStatus, error) {
	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return RunContextStatus{}, ErrRunNotFound
	}
	messages := append([]Message(nil), state.messages...)
	r.mu.RUnlock()

	totalTokens := 0
	for _, msg := range messages {
		runes := utf8.RuneCountInString(msg.Content)
		if runes > 0 {
			totalTokens += (runes + 3) / 4
		}
	}

	pressure := contextPressureLevel(totalTokens)
	return RunContextStatus{
		MessageCount:    len(messages),
		EstimatedTokens: totalTokens,
		ContextPressure: pressure,
	}, nil
}

func contextPressureLevel(estimatedTokens int) string {
	switch {
	case estimatedTokens > 60000:
		return "high"
	case estimatedTokens > 30000:
		return "medium"
	default:
		return "low"
	}
}

// CompactRunRequest holds the parameters for a CompactRun call.
type CompactRunRequest struct {
	// Mode must be one of "strip", "summarize", or "hybrid". Defaults to "strip".
	Mode     string
	KeepLast int
}

// CompactRunResult holds the result of a CompactRun call.
type CompactRunResult struct {
	MessagesRemoved int `json:"messages_removed"`
}

// CompactRun triggers in-memory context compaction on an active run.
// Returns ErrRunNotFound if the run does not exist, ErrRunNotActive if the
// run is not currently active (running or waiting for user input).
func (r *Runner) CompactRun(ctx context.Context, runID string, req CompactRunRequest) (CompactRunResult, error) {
	mode := req.Mode
	if mode == "" {
		mode = "strip"
	}
	if mode != "strip" && mode != "summarize" && mode != "hybrid" {
		return CompactRunResult{}, fmt.Errorf("mode must be one of: strip, summarize, hybrid")
	}

	keepLast := req.KeepLast
	if keepLast < 2 {
		keepLast = 4
	}

	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return CompactRunResult{}, ErrRunNotFound
	}
	status := state.run.Status
	r.mu.RUnlock()

	if status != RunStatusRunning && status != RunStatusWaitingForUser {
		return CompactRunResult{}, ErrRunNotActive
	}

	// Serialize with auto-compact to prevent concurrent mutations.
	state.compactMu.Lock()
	defer state.compactMu.Unlock()

	// Re-read messages under compactMu.
	r.mu.RLock()
	messages := append([]Message(nil), state.messages...)
	r.mu.RUnlock()

	// Convert messages to TranscriptMessages for the compaction logic.
	snap := messagesAsTranscriptSnapshot(messages)
	if len(snap) == 0 {
		return CompactRunResult{}, nil
	}

	// Snapshot the per-request Summarizer model from runState so that manual
	// CompactRun calls honour the RoleModels.Summarizer override, not just
	// the runner-level default (fix for HIGH issue in #25).
	r.mu.RLock()
	summarizerModel := state.resolvedRoleModels.Summarizer
	r.mu.RUnlock()

	beforeCount := len(snap)
	summarizer := r.newMessageSummarizerWithModel(summarizerModel)
	compacted, err := compactMessagesHTTP(ctx, snap, mode, keepLast, summarizer)
	if err != nil {
		return CompactRunResult{}, fmt.Errorf("compaction failed: %w", err)
	}

	// Convert compacted TranscriptMessages back to harness Messages.
	newMessages := transcriptMessagesToHarness(compacted)
	r.setMessages(runID, newMessages)

	removed := beforeCount - len(compacted)
	if removed < 0 {
		removed = 0
	}
	return CompactRunResult{MessagesRemoved: removed}, nil
}

// messagesForStep returns a fresh copy of state.messages under compactMu,
// ensuring that CompactRun() results are visible at the start of each step.
func (r *Runner) messagesForStep(state *runState) []Message {
	state.compactMu.Lock()
	msgs := copyMessages(state.messages)
	state.compactMu.Unlock()
	return msgs
}

// autoCompactMessages performs compaction on the run's messages under compactMu.
// It tries hybrid (or configured) mode first and falls back to strip on error.
// The per-request Summarizer role model override stored in runState is honoured
// so that a per-request RoleModels.Summarizer is not silently ignored.
func (r *Runner) autoCompactMessages(runID string, messages []Message) ([]Message, error) {
	r.mu.RLock()
	state, ok := r.runs[runID]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrRunNotFound
	}

	state.compactMu.Lock()
	defer state.compactMu.Unlock()

	snap := messagesAsTranscriptSnapshot(messages)
	if len(snap) == 0 {
		return messages, nil
	}

	mode := r.config.AutoCompactMode
	keepLast := r.config.AutoCompactKeepLast

	// Use the per-request summarizer model if one was resolved for this run.
	// newMessageSummarizerWithModel falls back to runner-level config when the
	// override is empty, preserving existing behaviour.
	summarizer := r.newMessageSummarizerWithModel(state.resolvedRoleModels.Summarizer)

	compacted, err := compactMessagesHTTP(context.Background(), snap, mode, keepLast, summarizer)
	if err != nil && mode != "strip" {
		// Fallback to strip mode if hybrid/summarize fails.
		compacted, err = compactMessagesHTTP(context.Background(), snap, "strip", keepLast, nil)
	}
	if err != nil {
		return nil, err
	}

	return transcriptMessagesToHarness(compacted), nil
}

// messagesAsTranscriptSnapshot converts harness Messages to the tool-layer
// TranscriptMessage format used by the compaction logic.
func messagesAsTranscriptSnapshot(msgs []Message) []htools.TranscriptMessage {
	result := make([]htools.TranscriptMessage, 0, len(msgs))
	for i, m := range msgs {
		if m.IsMeta {
			continue
		}
		tm := htools.TranscriptMessage{
			Index:      int64(i),
			Role:       m.Role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		}
		result = append(result, tm)
	}
	return result
}

// transcriptMessagesToHarness converts tool-layer TranscriptMessages back to
// harness Messages suitable for setMessages.
func transcriptMessagesToHarness(msgs []htools.TranscriptMessage) []Message {
	result := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, Message{
			Role:       m.Role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		})
	}
	return result
}

// compactMessagesHTTP applies the compaction strategy to transcript messages.
// It mirrors the logic inside the compact_history tool handler but operates
// directly on slices without a context-based reader/replacer.
func compactMessagesHTTP(
	ctx context.Context,
	msgs []htools.TranscriptMessage,
	mode string,
	keepLast int,
	summarizer htools.MessageSummarizer,
) ([]htools.TranscriptMessage, error) {
	turns := parseTurnsHTTP(msgs)
	prefixEnd, compactEnd := findCompactionBoundsHTTP(turns, keepLast)

	if compactEnd <= prefixEnd {
		// Nothing to compact — return the original slice.
		return msgs, nil
	}

	switch mode {
	case "strip":
		return compactStripHTTP(turns, prefixEnd, compactEnd), nil
	case "summarize":
		if summarizer == nil {
			return nil, fmt.Errorf("summarize mode requires a message summarizer (not configured)")
		}
		result, _, err := compactSummarizeHTTP(ctx, turns, prefixEnd, compactEnd, summarizer)
		return result, err
	case "hybrid":
		result, _, err := compactHybridHTTP(ctx, turns, prefixEnd, compactEnd, summarizer)
		return result, err
	default:
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}
}

// httpTurn mirrors the tools-layer turn type for HTTP-based compaction.
type httpTurn struct {
	Messages []htools.TranscriptMessage
	Kind     string
}

func parseTurnsHTTP(msgs []htools.TranscriptMessage) []httpTurn {
	if len(msgs) == 0 {
		return nil
	}

	var turns []httpTurn
	i := 0

	for i < len(msgs) && msgs[i].Role == "system" {
		kind := "system_prefix"
		if msgs[i].Name == "compact_summary" {
			kind = "compact_summary"
		}
		turns = append(turns, httpTurn{
			Messages: []htools.TranscriptMessage{msgs[i]},
			Kind:     kind,
		})
		i++
	}

	for i < len(msgs) {
		msg := msgs[i]

		switch msg.Role {
		case "user":
			turns = append(turns, httpTurn{
				Messages: []htools.TranscriptMessage{msg},
				Kind:     "user",
			})
			i++

		case "assistant":
			t := httpTurn{
				Messages: []htools.TranscriptMessage{msg},
				Kind:     "assistant_text",
			}
			i++

			hasToolResults := false
			for i < len(msgs) && (msgs[i].Role == "tool" || msgs[i].Role == "system") {
				if msgs[i].Role == "tool" {
					hasToolResults = true
					t.Messages = append(t.Messages, msgs[i])
					i++
				} else if msgs[i].Role == "system" {
					t.Messages = append(t.Messages, msgs[i])
					i++
				} else {
					break
				}
			}
			if hasToolResults {
				t.Kind = "assistant_tool"
			}
			turns = append(turns, t)

		case "system":
			kind := "system_prefix"
			if msg.Name == "compact_summary" {
				kind = "compact_summary"
			}
			turns = append(turns, httpTurn{
				Messages: []htools.TranscriptMessage{msg},
				Kind:     kind,
			})
			i++

		case "tool":
			turns = append(turns, httpTurn{
				Messages: []htools.TranscriptMessage{msg},
				Kind:     "assistant_tool",
			})
			i++

		default:
			turns = append(turns, httpTurn{
				Messages: []htools.TranscriptMessage{msg},
				Kind:     "user",
			})
			i++
		}
	}

	return turns
}

func findCompactionBoundsHTTP(turns []httpTurn, keepLast int) (prefixEnd, compactEnd int) {
	for prefixEnd < len(turns) {
		if turns[prefixEnd].Kind != "system_prefix" && turns[prefixEnd].Kind != "compact_summary" {
			break
		}
		prefixEnd++
	}

	nonPrefixCount := len(turns) - prefixEnd
	if nonPrefixCount <= keepLast {
		return prefixEnd, prefixEnd
	}

	compactEnd = len(turns) - keepLast
	return prefixEnd, compactEnd
}

func compactStripHTTP(turns []httpTurn, prefixEnd, compactEnd int) []htools.TranscriptMessage {
	var result []htools.TranscriptMessage

	for i := 0; i < prefixEnd; i++ {
		result = append(result, turns[i].Messages...)
	}

	stripped := 0
	for i := prefixEnd; i < compactEnd; i++ {
		t := turns[i]
		switch t.Kind {
		case "assistant_tool":
			if len(t.Messages) > 0 && strings.TrimSpace(t.Messages[0].Content) != "" {
				result = append(result, htools.TranscriptMessage{
					Index:   t.Messages[0].Index,
					Role:    "assistant",
					Content: t.Messages[0].Content,
				})
			}
			for _, m := range t.Messages {
				if m.Role == "tool" {
					stripped++
				}
			}
		default:
			result = append(result, t.Messages...)
		}
	}

	if stripped > 0 {
		result = append(result, htools.TranscriptMessage{
			Role:    "system",
			Name:    "compact_summary",
			Content: fmt.Sprintf("[context compacted: %d tool interactions stripped]", stripped),
		})
	}

	for i := compactEnd; i < len(turns); i++ {
		result = append(result, turns[i].Messages...)
	}

	return result
}

func compactSummarizeHTTP(
	ctx context.Context,
	turns []httpTurn,
	prefixEnd, compactEnd int,
	summarizer htools.MessageSummarizer,
) ([]htools.TranscriptMessage, string, error) {
	var result []htools.TranscriptMessage

	for i := 0; i < prefixEnd; i++ {
		result = append(result, turns[i].Messages...)
	}

	var zoneMsgs []map[string]any
	for i := prefixEnd; i < compactEnd; i++ {
		for _, m := range turns[i].Messages {
			zoneMsgs = append(zoneMsgs, map[string]any{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}

	summary, err := summarizer.SummarizeMessages(ctx, zoneMsgs)
	if err != nil {
		return nil, "", err
	}

	result = append(result, htools.TranscriptMessage{
		Role:    "system",
		Name:    "compact_summary",
		Content: summary,
	})

	for i := compactEnd; i < len(turns); i++ {
		result = append(result, turns[i].Messages...)
	}

	return result, summary, nil
}

func compactHybridHTTP(
	ctx context.Context,
	turns []httpTurn,
	prefixEnd, compactEnd int,
	summarizer htools.MessageSummarizer,
) ([]htools.TranscriptMessage, string, error) {
	var result []htools.TranscriptMessage

	for i := 0; i < prefixEnd; i++ {
		result = append(result, turns[i].Messages...)
	}

	const largeTokenThreshold = 500
	var removedContent []string
	stripped := 0

	for i := prefixEnd; i < compactEnd; i++ {
		t := turns[i]
		switch t.Kind {
		case "assistant_tool":
			if len(t.Messages) > 0 && strings.TrimSpace(t.Messages[0].Content) != "" {
				result = append(result, htools.TranscriptMessage{
					Index:   t.Messages[0].Index,
					Role:    "assistant",
					Content: t.Messages[0].Content,
				})
			}
			for _, m := range t.Messages {
				if m.Role != "tool" {
					continue
				}
				runes := utf8.RuneCountInString(m.Content)
				tokens := 0
				if runes > 0 {
					tokens = (runes + 3) / 4
				}
				if tokens > largeTokenThreshold {
					removedContent = append(removedContent, m.Content)
					stripped++
				} else {
					result = append(result, m)
				}
			}
		default:
			result = append(result, t.Messages...)
		}
	}

	var summary string
	if len(removedContent) > 0 {
		if summarizer != nil {
			var summaryMsgs []map[string]any
			for _, content := range removedContent {
				summaryMsgs = append(summaryMsgs, map[string]any{
					"role":    "tool",
					"content": content,
				})
			}
			var err error
			summary, err = summarizer.SummarizeMessages(ctx, summaryMsgs)
			if err != nil {
				summary = ""
			}
		}

		marker := fmt.Sprintf("[context compacted: %d large tool outputs removed]", stripped)
		if summary != "" {
			marker = fmt.Sprintf("[context compacted: %d large tool outputs summarized]\n%s", stripped, summary)
		}
		result = append(result, htools.TranscriptMessage{
			Role:    "system",
			Name:    "compact_summary",
			Content: marker,
		})
	}

	for i := compactEnd; i < len(turns); i++ {
		result = append(result, turns[i].Messages...)
	}

	return result, summary, nil
}

// SummarizeMessages makes a single LLM call to summarize the given messages.
// Returns a summary string suitable for use as a compact summary.
// The model is resolved from: runner-level config defaults < config RoleModels.Summarizer.
// Use SummarizeMessagesWithModel to supply a per-request override on top of that.
func (r *Runner) SummarizeMessages(ctx context.Context, messages []Message) (string, error) {
	return r.SummarizeMessagesWithModel(ctx, messages, "")
}

// SummarizeMessagesWithModel is like SummarizeMessages but accepts an explicit
// model override. When overrideModel is non-empty it takes precedence over both
// the runner-level DefaultModel and the config-level RoleModels.Summarizer.
// This is used to honour per-request RoleModels.Summarizer overrides during
// auto-compaction, where the resolved model is stored in runState.
func (r *Runner) SummarizeMessagesWithModel(ctx context.Context, messages []Message, overrideModel string) (string, error) {
	if r.provider == nil {
		return "", fmt.Errorf("provider not configured")
	}
	model := r.config.DefaultModel
	if model == "" {
		model = "gpt-4.1-mini"
	}
	// Apply Summarizer role model override when configured.
	if r.config.RoleModels.Summarizer != "" {
		model = r.config.RoleModels.Summarizer
	}
	// Per-request override wins over everything else.
	if overrideModel != "" {
		model = overrideModel
	}
	req := CompletionRequest{
		Model: model,
		Messages: append(copyMessages(messages), Message{
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

// runnerMessageSummarizer adapts *Runner to the tools.MessageSummarizer interface.
// overrideModel, when non-empty, is passed to SummarizeMessagesWithModel so that
// per-request Summarizer role model overrides are honoured during compaction.
type runnerMessageSummarizer struct {
	runner        *Runner
	overrideModel string
}

func (s *runnerMessageSummarizer) SummarizeMessages(ctx context.Context, msgs []map[string]any) (string, error) {
	converted := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		msg := Message{}
		if v, ok := m["role"].(string); ok {
			msg.Role = v
		}
		if v, ok := m["content"].(string); ok {
			msg.Content = v
		}
		if v, ok := m["name"].(string); ok {
			msg.Name = v
		}
		if v, ok := m["tool_call_id"].(string); ok {
			msg.ToolCallID = v
		}
		converted = append(converted, msg)
	}
	return s.runner.SummarizeMessagesWithModel(ctx, converted, s.overrideModel)
}

// NewMessageSummarizer returns a tools.MessageSummarizer backed by this runner.
func (r *Runner) NewMessageSummarizer() htools.MessageSummarizer {
	return &runnerMessageSummarizer{runner: r}
}

// newMessageSummarizerWithModel returns a tools.MessageSummarizer that uses
// overrideModel for all summarization calls, taking precedence over the
// runner-level config. Pass "" to use the default resolution order.
func (r *Runner) newMessageSummarizerWithModel(overrideModel string) htools.MessageSummarizer {
	return &runnerMessageSummarizer{runner: r, overrideModel: overrideModel}
}

// GetSummarizer returns a MessageSummarizer backed by this runner, or nil if no
// provider is configured (which means summarization is not available).
func (r *Runner) GetSummarizer() htools.MessageSummarizer {
	if r.provider == nil {
		return nil
	}
	return &runnerMessageSummarizer{runner: r}
}

func (r *Runner) observeMemory(runID string, step int, messages []Message) {
	if r.config.MemoryManager == nil || r.config.MemoryManager.Mode() == om.ModeOff {
		return
	}
	scope := r.scopeKey(runID)
	converted := make([]om.TranscriptMessage, 0, len(messages))
	for i, msg := range messages {
		converted = append(converted, om.TranscriptMessage{
			Index:      int64(i),
			Role:       msg.Role,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
			Content:    msg.Content,
		})
	}
	r.emit(runID, EventMemoryObserveStarted, map[string]any{"step": step})
	out, err := r.config.MemoryManager.Observe(context.Background(), om.ObserveRequest{
		Scope:    scope,
		RunID:    runID,
		Messages: converted,
	})
	if err != nil {
		r.emit(runID, EventMemoryObserveFailed, map[string]any{"step": step, "error": err.Error()})
		return
	}
	r.emit(runID, EventMemoryObserveCompleted, map[string]any{
		"step":        step,
		"observed":    out.Observed,
		"reflected":   out.Reflected,
		"observation": out.Status.ObservationCount,
	})
	if out.Reflected {
		r.emit(runID, EventMemoryReflectionCompleted, map[string]any{"step": step})
	}
}

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

// deepClonePayload returns a fully isolated deep copy of a map[string]any.
// It uses reflection to deep-clone all slice and map types (including typed
// slices like []string) so that no mutable references are shared between
// stored forensic history, the rollout recorder, and subscriber copies.
// Primitive scalar types (string, bool, numeric) are returned as-is since
// they are immutable value types in Go.
func deepClonePayload(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCloneValue(v)
	}
	return out
}

// deepCloneValue recursively deep-clones any value containing mutable
// reference types (slices, maps). Scalars and nil are returned as-is.
func deepCloneValue(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		out := reflect.MakeMap(rv.Type())
		for _, key := range rv.MapKeys() {
			cloned := deepCloneValue(rv.MapIndex(key).Interface())
			cv := reflect.ValueOf(cloned)
			if cv.IsValid() {
				out.SetMapIndex(key, cv)
			} else {
				// cloned is nil: preserve the key with a typed zero value so that
				// {"x": nil} is not silently dropped from the cloned map.
				out.SetMapIndex(key, reflect.Zero(rv.Type().Elem()))
			}
		}
		return out.Interface()
	case reflect.Slice:
		if rv.IsNil() {
			return v
		}
		out := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		for i := 0; i < rv.Len(); i++ {
			cloned := deepCloneValue(rv.Index(i).Interface())
			if cv := reflect.ValueOf(cloned); cv.IsValid() {
				out.Index(i).Set(cv)
			}
		}
		return out.Interface()
	default:
		// Scalars (string, bool, int*, uint*, float*) are value types —
		// no aliasing is possible through an interface{}.
		return v
	}
}

// deepCloneMessage returns a Message with an independent copy of its ToolCalls.
func deepCloneMessage(m Message) Message {
	return m.Clone()
}

// copyMessages returns a deep copy of msgs where each Message has an
// independent ToolCalls slice, preventing callers from mutating runner state.
func copyMessages(msgs []Message) []Message {
	if len(msgs) == 0 {
		return nil // preserve nil for both nil and empty-non-nil (matches original append behavior)
	}
	result := make([]Message, len(msgs))
	for i := range msgs {
		result[i] = deepCloneMessage(msgs[i])
	}
	return result
}

func (r *Runner) emit(runID string, eventType EventType, payload map[string]any) {
	r.mu.Lock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.Unlock()
		return
	}

	// Drop post-terminal events to preserve forensic ordering. Provider
	// streaming callbacks and tool output goroutines can fire after
	// run.completed/run.failed; we gate them here so no orphan events are
	// appended to the forensic record after it is sealed.
	if state.terminated {
		r.mu.Unlock()
		return
	}

	// Deep-clone the caller's payload so that nested maps and slices inside
	// the payload are not aliased. A shallow copy is insufficient: if the
	// caller holds a reference to a nested slice or map and mutates it after
	// emit() returns (or concurrently), the stored forensic event would
	// otherwise observe those mutations (#228).
	enriched := deepClonePayload(payload)
	if enriched == nil {
		enriched = make(map[string]any, 3)
	}
	// Inject forensic correlation fields into every event payload.
	enriched["schema_version"] = EventSchemaVersion
	enriched["conversation_id"] = state.run.ConversationID
	if _, ok := enriched["step"]; !ok {
		enriched["step"] = state.currentStep
	}

	// Seal the run for terminal events BEFORE redaction so that even if the
	// redaction pipeline drops the event, the recorder is still closed and
	// the terminated gate is still armed. Without this, a "drop" rule on
	// run.completed would leave the run unsealed forever.
	isTerminal := IsTerminalEvent(eventType)
	rec := state.recorder
	if isTerminal {
		state.terminated = true
		if rec != nil {
			state.recorder = nil
		}
	}

	// Apply PII/secret redaction pipeline if configured.
	if r.config.RedactionPipeline != nil {
		var keep bool
		enriched, keep = redaction.RedactPayload(r.config.RedactionPipeline, string(eventType), enriched)
		if !keep {
			r.mu.Unlock()
			// Still close the recorder if this was a terminal event that got dropped.
			if isTerminal && rec != nil {
				state.recorderMu.Lock()
				if !state.recorderClosed {
					rec.Close()
					state.recorderClosed = true
				}
				state.recorderMu.Unlock()
			}
			return
		}
	}

	// Deep-clone the enriched payload for immutable forensic storage.
	// This prevents any nested map/slice from being shared with subscribers,
	// the recorder, or the original caller.
	storedPayload := deepClonePayload(enriched)

	eventSeq := state.nextEventSeq
	event := Event{
		ID:        fmt.Sprintf("%s:%d", runID, eventSeq),
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   storedPayload,
	}
	state.nextEventSeq++

	state.events = append(state.events, event)

	// Fan out to subscribers while holding the lock so cancel() cannot close
	// the channel between our check and send — prevents send-on-closed-channel.
	// Each subscriber gets its own deep copy of the payload so that one
	// subscriber cannot corrupt the stored forensic history or race with
	// other subscribers by mutating nested structures.
	for ch := range state.subscribers {
		evCopy := event
		evCopy.Payload = deepClonePayload(storedPayload)
		select {
		case ch <- evCopy:
		default:
			// Drop if subscriber is too slow; event is still persisted in run history.
		}
	}

	r.mu.Unlock()

	// Record to JSONL rollout file (outside the main lock to avoid blocking).
	// We serialise through recorderMu so that concurrent Record/Close calls
	// never write to a closed file.  The Seq field is set to the logical
	// sequence number assigned above (under r.mu) so that even if two
	// goroutines swap order while competing for recorderMu, the on-disk
	// "seq" field correctly reflects the emission order, not the write order.
	if rec != nil {
		state.recorderMu.Lock()
		// Check recorderClosed inside the lock: a goroutine that captured rec
		// before terminal detach must not record after the terminal goroutine
		// has already closed it.
		if !state.recorderClosed {
			rec.Record(rollout.RecordableEvent{
				ID:        event.ID,
				RunID:     event.RunID,
				Type:      string(event.Type),
				Timestamp: event.Timestamp,
				Payload:   event.Payload,
				Seq:       eventSeq,
			})
			// Close the recorder after terminal events so the file is flushed.
			if isTerminal {
				_ = rec.Close()
				state.recorderClosed = true
			}
		}
		state.recorderMu.Unlock()
	}
}

func (r *Runner) emitCompletionDelta(runID string, step int, delta CompletionDelta) {
	if delta.Content != "" {
		r.emit(runID, EventAssistantMessageDelta, map[string]any{
			"step":    step,
			"content": delta.Content,
		})
	}
	if delta.Reasoning != "" && r.config.CaptureReasoning {
		r.emit(runID, EventAssistantThinkingDelta, map[string]any{
			"step":    step,
			"content": delta.Reasoning,
		})
	}
	if delta.ToolCall.ID == "" && delta.ToolCall.Name == "" && delta.ToolCall.Arguments == "" {
		return
	}
	r.emit(runID, EventToolCallDelta, map[string]any{
		"step":      step,
		"index":     delta.ToolCall.Index,
		"call_id":   delta.ToolCall.ID,
		"tool":      delta.ToolCall.Name,
		"arguments": delta.ToolCall.Arguments,
	})
}

func (r *Runner) nextID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.New().String())
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"failed to marshal tool error"}`
	}
	return string(data)
}

// auditLogPath returns the path for the audit.jsonl file under the given
// rollout directory, partitioned by the current UTC date.
func auditLogPath(rolloutDir string) string {
	dateDir := rolloutDir + "/" + time.Now().UTC().Format("2006-01-02")
	return dateDir + "/audit.jsonl"
}

// writeAudit writes a record to the run's audit writer if audit trail is
// enabled and the writer is available. It never blocks the run loop.
func (r *Runner) writeAudit(runID string, rec audittrail.AuditRecord) {
	r.mu.RLock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.RUnlock()
		return
	}
	aw := state.auditWriter
	r.mu.RUnlock()

	if aw == nil {
		return
	}
	// Errors are silently dropped to never impact the run loop.
	_ = aw.Write(rec)
}

// resolveRoleModels merges the per-request RoleModels with the runner-level
// RoleModels configuration. Request-level fields take precedence; empty fields
// fall back to the runner config. The returned RoleModels always reflects the
// highest-priority non-empty override for each role.
func (r *Runner) resolveRoleModels(req RunRequest) RoleModels {
	result := r.config.RoleModels // start from config defaults
	if req.RoleModels != nil {
		if req.RoleModels.Primary != "" {
			result.Primary = req.RoleModels.Primary
		}
		if req.RoleModels.Summarizer != "" {
			result.Summarizer = req.RoleModels.Summarizer
		}
	}
	return result
}

// closeAuditWriter closes the audit writer for a run, if any.
func (r *Runner) closeAuditWriter(runID string) {
	r.mu.Lock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.Unlock()
		return
	}
	aw := state.auditWriter
	state.auditWriter = nil
	r.mu.Unlock()

	if aw != nil {
		_ = aw.Close()
	}
}
