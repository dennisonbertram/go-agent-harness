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
)

// steeringBufferSize is the capacity of the per-run steering message channel.
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
	// MaxSteps <= 0 means unlimited; no default cap is applied here.
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
	}
	r.mu.Unlock()

	go r.execute(run.ID, req)

	return run, nil
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

	// Snapshot before we release.
	convID := state.run.ConversationID
	existingModel := state.run.Model
	existingTenantID := state.run.TenantID
	existingAgentID := state.run.AgentID
	systemPrompt := state.staticSystemPrompt
	promptResolved := state.promptResolved

	// Transition the source run so no second goroutine can also continue it.
	state.run.Status = RunStatusRunning
	state.run.UpdatedAt = time.Now().UTC()

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
	}
	r.mu.Unlock()

	// Build the request after the lock is released.
	req := RunRequest{
		Prompt:         message,
		Model:          existingModel,
		ConversationID: convID,
		TenantID:       existingTenantID,
		AgentID:        existingAgentID,
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
	r.mu.RUnlock()

	if !ok {
		return ErrRunNotFound
	}

	status := state.run.Status
	if status != RunStatusRunning && status != RunStatusWaitingForUser {
		return ErrRunNotActive
	}

	select {
	case state.steeringCh <- message:
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

	history := append([]Event(nil), state.events...)
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

// resolveProvider determines which Provider to use for a run.
// Returns the provider, its name, and any error.
func (r *Runner) resolveProvider(runID, model string, allowFallback bool) (Provider, string, error) {
	if r.providerRegistry == nil {
		return r.provider, "default", nil
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

	// Set provider name on run state
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

	// Resolve the effective step limit for this run.
	// Priority: per-run request > runner config.
	// 0 in either position means "no limit" once chosen.
	effectiveMaxSteps := r.config.MaxSteps
	if req.MaxSteps > 0 {
		effectiveMaxSteps = req.MaxSteps
	}
	// effectiveMaxSteps == 0 means unlimited.

	for step := 1; effectiveMaxSteps == 0 || step <= effectiveMaxSteps; step++ {
		// Drain any pending steering messages and inject them as user messages.
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

		completionReq := CompletionRequest{
			Model:    model,
			Messages: turnMessages,
			Tools:    r.filteredToolsForRun(runID),
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

		result, err := activeProvider.Complete(context.Background(), completionReq)
		if err != nil {
			r.failRun(runID, fmt.Errorf("provider completion failed: %w", err))
			return
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
		r.emit(runID, EventLLMTurnCompleted, map[string]any{"step": step, "tool_calls": len(result.ToolCalls)})

		if len(result.ToolCalls) == 0 {
			if result.Content != "" {
				messages = append(messages, Message{Role: "assistant", Content: result.Content})
				r.setMessages(runID, messages)
				r.emit(runID, EventAssistantMessage, map[string]any{"content": result.Content})
			}
			r.observeMemory(runID, step, messages)
			r.completeRun(runID, result.Content)
			return
		}

		messages = append(messages, Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})
		r.setMessages(runID, messages)

		for _, call := range result.ToolCalls {
			r.emit(runID, EventToolCallStarted, map[string]any{
				"call_id":   call.ID,
				"tool":      call.Name,
				"arguments": call.Arguments,
			})

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
					"call_id": call.ID,
					"tool":    call.Name,
					"error":   toolErr.Error(),
					"output":  toolOutput,
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
					"call_id": call.ID,
					"tool":    call.Name,
					"output":  toolOutput,
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
		}
		r.observeMemory(runID, step, messages)
	}

	r.failRun(runID, fmt.Errorf("max steps (%d) reached", effectiveMaxSteps))
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
			"stage":   "pre_message",
			"hook":    hookName,
			"step":    step,
			"action":  action,
			"mutated": mutated,
			"reason":  result.Reason,
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
			"stage":   "post_message",
			"hook":    hookName,
			"step":    step,
			"action":  action,
			"mutated": mutated,
			"reason":  result.Reason,
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
				// fail_closed: return current output unchanged
				continue
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

	constraint, active := r.skillConstraints.Active(runID)
	if !active || constraint.AllowedTools == nil {
		return defs // no skill constraint or no restriction
	}

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

	// Store conversation messages for multi-turn support
	r.mu.RLock()
	state, ok := r.runs[runID]
	if ok {
		convID := state.run.ConversationID
		msgs := append([]Message(nil), state.messages...)
		r.mu.RUnlock()

		r.mu.Lock()
		r.conversations[convID] = msgs
		r.mu.Unlock()

		// Persist to SQLite store if configured
		if r.config.ConversationStore != nil {
			usageTotals, costTotals := r.accountingTotals(runID)
			tokenCost := ConversationTokenCost{
				PromptTokens:     usageTotals.PromptTokensTotal,
				CompletionTokens: usageTotals.CompletionTokensTotal,
				CostUSD:          costTotals.CostUSDTotal,
			}
			if err := r.config.ConversationStore.SaveConversationWithCost(context.Background(), convID, msgs, tokenCost); err != nil {
				if r.config.Logger != nil {
					r.config.Logger.Error("failed to persist conversation", "conv_id", convID, "error", err)
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
	usageTotals, costTotals := r.accountingTotals(runID)
	r.emit(runID, EventRunFailed, map[string]any{
		"error":        err.Error(),
		"usage_totals": usageTotals,
		"cost_totals":  costTotals,
	})
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
			"turn_usage":          turnUsage,
			"turn_cost_usd":       turnCostUSD,
			"cumulative_usage":    CompletionUsage{},
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
		"turn_usage":          turnUsage,
		"turn_cost_usd":       turnCostUSD,
		"cumulative_usage":    cumulativeUsage,
		"cumulative_cost_usd": costTotals.CostUSDTotal,
		"pricing_version":     costTotals.PricingVersion,
	}
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
	state.messages = append([]Message(nil), messages...)
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
		return append([]Message(nil), msgs...) // return a copy
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
			return loaded
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
		return append([]Message(nil), msgs...), true
	}
	r.mu.RUnlock()

	// Fall through to persistent store
	if r.config.ConversationStore != nil {
		loaded, err := r.config.ConversationStore.LoadMessages(context.Background(), conversationID)
		if err != nil {
			return nil, false
		}
		if len(loaded) > 0 {
			return loaded, true
		}
	}
	return nil, false
}

// GetConversationStore returns the configured conversation store, or nil.
func (r *Runner) GetConversationStore() ConversationStore {
	return r.config.ConversationStore
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

func (r *Runner) emit(runID string, eventType EventType, payload map[string]any) {
	r.mu.Lock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.Unlock()
		return
	}

	event := Event{
		ID:        fmt.Sprintf("%s:%d", runID, state.nextEventSeq),
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
	state.nextEventSeq++

	state.events = append(state.events, event)
	subscribers := make([]chan Event, 0, len(state.subscribers))
	for ch := range state.subscribers {
		subscribers = append(subscribers, ch)
	}
	r.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is too slow; event is still persisted in run history.
		}
	}
}

func (r *Runner) emitCompletionDelta(runID string, step int, delta CompletionDelta) {
	if delta.Content != "" {
		r.emit(runID, EventAssistantMessageDelta, map[string]any{
			"step":    step,
			"content": delta.Content,
		})
	}
	if delta.Reasoning != "" {
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
	n := atomic.AddUint64(&r.idSeq, 1)
	return fmt.Sprintf("%s_%d", prefix, n)
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"failed to marshal tool error"}`
	}
	return string(data)
}
