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
)

type runState struct {
	run         Run
	events      []Event
	subscribers map[chan Event]struct{}
}

var (
	ErrRunNotFound     = errors.New("run not found")
	ErrNoPendingInput  = errors.New("no pending input")
	ErrInvalidRunInput = errors.New("invalid run input")
)

type Runner struct {
	provider Provider
	tools    *Registry
	config   RunnerConfig

	mu    sync.RWMutex
	runs  map[string]*runState
	idSeq uint64
}

func NewRunner(provider Provider, tools *Registry, config RunnerConfig) *Runner {
	if config.DefaultModel == "" {
		config.DefaultModel = "gpt-4.1-mini"
	}
	if config.MaxSteps <= 0 {
		config.MaxSteps = 8
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
	return &Runner{
		provider: provider,
		tools:    tools,
		config:   config,
		runs:     make(map[string]*runState),
	}
}

func (r *Runner) StartRun(req RunRequest) (Run, error) {
	if r.provider == nil {
		return Run{}, fmt.Errorf("provider is required")
	}
	if req.Prompt == "" {
		return Run{}, fmt.Errorf("prompt is required")
	}

	model := req.Model
	if model == "" {
		model = r.config.DefaultModel
	}

	now := time.Now().UTC()
	run := Run{
		ID:        r.nextID("run"),
		Prompt:    req.Prompt,
		Model:     model,
		Status:    RunStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.mu.Lock()
	r.runs[run.ID] = &runState{
		run:         run,
		events:      make([]Event, 0, 32),
		subscribers: make(map[chan Event]struct{}),
	}
	r.mu.Unlock()

	go r.execute(run.ID, req)

	return run, nil
}

func (r *Runner) GetRun(runID string) (Run, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.runs[runID]
	if !ok {
		return Run{}, false
	}
	return state.run, true
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

func (r *Runner) execute(runID string, req RunRequest) {
	r.setStatus(runID, RunStatusRunning, "", "")
	r.emit(runID, "run.started", map[string]any{"prompt": req.Prompt})

	model := req.Model
	if model == "" {
		model = r.config.DefaultModel
	}
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = r.config.DefaultSystemPrompt
	}

	messages := make([]Message, 0, 16)
	if systemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, Message{Role: "user", Content: req.Prompt})

	for step := 1; step <= r.config.MaxSteps; step++ {
		r.emit(runID, "llm.turn.requested", map[string]any{"step": step})

		completionReq := CompletionRequest{
			Model:    model,
			Messages: messages,
			Tools:    r.tools.Definitions(),
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

		result, err := r.provider.Complete(context.Background(), completionReq)
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

		r.emit(runID, "llm.turn.completed", map[string]any{"step": step, "tool_calls": len(result.ToolCalls)})

		if len(result.ToolCalls) == 0 {
			if result.Content != "" {
				r.emit(runID, "assistant.message", map[string]any{"content": result.Content})
			}
			r.completeRun(runID, result.Content)
			return
		}

		messages = append(messages, Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})

		for _, call := range result.ToolCalls {
			r.emit(runID, "tool.call.started", map[string]any{
				"call_id":   call.ID,
				"tool":      call.Name,
				"arguments": call.Arguments,
			})

			waitingForUser := false
			if call.Name == htools.AskUserQuestionToolName {
				questions, err := htools.ParseAskUserQuestionArgs(json.RawMessage(call.Arguments))
				if err == nil {
					waitingForUser = true
					deadlineAt := time.Now().UTC().Add(r.config.AskUserTimeout)
					r.setStatus(runID, RunStatusWaitingForUser, "", "")
					r.emit(runID, "run.waiting_for_user", map[string]any{
						"call_id":     call.ID,
						"tool":        call.Name,
						"questions":   questions,
						"deadline_at": deadlineAt,
					})
				}
			}

			toolCtx := context.WithValue(context.Background(), htools.ContextKeyRunID, runID)
			toolCtx = context.WithValue(toolCtx, htools.ContextKeyToolCallID, call.ID)
			toolOutput, toolErr := r.tools.Execute(toolCtx, call.Name, json.RawMessage(call.Arguments))
			if toolErr != nil {
				toolOutput = mustJSON(map[string]any{"error": toolErr.Error()})
				r.emit(runID, "tool.call.completed", map[string]any{
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
				r.emit(runID, "tool.call.completed", map[string]any{
					"call_id": call.ID,
					"tool":    call.Name,
					"output":  toolOutput,
				})
				if waitingForUser {
					r.setStatus(runID, RunStatusRunning, "", "")
					r.emit(runID, "run.resumed", map[string]any{
						"call_id":     call.ID,
						"tool":        call.Name,
						"answered_at": time.Now().UTC(),
					})
				}
			}

			messages = append(messages, Message{
				Role:       "tool",
				Name:       call.Name,
				ToolCallID: call.ID,
				Content:    toolOutput,
			})
		}
	}

	r.failRun(runID, fmt.Errorf("max steps (%d) reached", r.config.MaxSteps))
}

type hookBlock struct {
	hookName string
	reason   string
}

func (r *Runner) applyPreHooks(ctx context.Context, runID string, step int, req CompletionRequest) (CompletionRequest, *hookBlock, error) {
	current := req
	for _, hook := range r.config.PreMessageHooks {
		hookName := normalizeHookName(hook.Name())
		r.emit(runID, "hook.started", map[string]any{
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
			r.emit(runID, "hook.failed", map[string]any{
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

		r.emit(runID, "hook.completed", map[string]any{
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
		r.emit(runID, "hook.started", map[string]any{
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
			r.emit(runID, "hook.failed", map[string]any{
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

		r.emit(runID, "hook.completed", map[string]any{
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

func (r *Runner) completeRun(runID, output string) {
	r.setStatus(runID, RunStatusCompleted, output, "")
	r.emit(runID, "run.completed", map[string]any{"output": output})
}

func (r *Runner) failRun(runID string, err error) {
	if err == nil {
		err = errors.New("run failed")
	}
	r.setStatus(runID, RunStatusFailed, "", err.Error())
	r.emit(runID, "run.failed", map[string]any{"error": err.Error()})
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

func (r *Runner) emit(runID, eventType string, payload map[string]any) {
	event := Event{
		ID:        r.nextID("evt"),
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}

	r.mu.Lock()
	state, ok := r.runs[runID]
	if !ok {
		r.mu.Unlock()
		return
	}
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
