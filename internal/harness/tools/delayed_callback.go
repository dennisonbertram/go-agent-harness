package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// Constants
const (
	MaxCallbackDelay    = 1 * time.Hour
	MinCallbackDelay    = 5 * time.Second
	MaxCallbacksPerConv = 10
)

// RunStarter is the interface for starting a new run on a conversation.
// Implemented by the runner; injected via lazy adapter to avoid circular deps.
type RunStarter interface {
	StartRun(prompt, conversationID string) error
}

// CallbackState represents the lifecycle state of a callback.
type CallbackState string

const (
	CallbackStatePending  CallbackState = "pending"
	CallbackStateFired    CallbackState = "fired"
	CallbackStateCanceled CallbackState = "canceled"
)

// CallbackInfo holds metadata about a scheduled callback.
type CallbackInfo struct {
	ID             string        `json:"id"`
	ConversationID string        `json:"conversation_id"`
	Delay          string        `json:"delay"`
	Prompt         string        `json:"prompt"`
	State          CallbackState `json:"state"`
	FiresAt        time.Time     `json:"fires_at"`
	CreatedAt      time.Time     `json:"created_at"`
}

type pendingCallback struct {
	info  CallbackInfo
	timer *time.Timer
}

// CallbackManager manages delayed callbacks for agent conversations.
type CallbackManager struct {
	mu        sync.Mutex
	callbacks map[string]*pendingCallback // keyed by callback ID
	byConv    map[string][]string         // conversation ID -> callback IDs
	starter   RunStarter
	now       func() time.Time
	stopped   bool
}

// NewCallbackManager creates a new CallbackManager.
func NewCallbackManager(starter RunStarter) *CallbackManager {
	return &CallbackManager{
		callbacks: make(map[string]*pendingCallback),
		byConv:    make(map[string][]string),
		starter:   starter,
		now:       time.Now,
	}
}

// Set schedules a new delayed callback.
func (m *CallbackManager) Set(conversationID string, delay time.Duration, prompt string) (CallbackInfo, error) {
	if delay < MinCallbackDelay {
		return CallbackInfo{}, fmt.Errorf("delay %v is less than minimum %v", delay, MinCallbackDelay)
	}
	if delay > MaxCallbackDelay {
		return CallbackInfo{}, fmt.Errorf("delay %v exceeds maximum %v", delay, MaxCallbackDelay)
	}
	if prompt == "" {
		return CallbackInfo{}, fmt.Errorf("prompt must not be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return CallbackInfo{}, fmt.Errorf("callback manager is shut down")
	}

	// Check per-conversation limit
	if len(m.byConv[conversationID]) >= MaxCallbacksPerConv {
		return CallbackInfo{}, fmt.Errorf("conversation %s has reached the maximum of %d callbacks", conversationID, MaxCallbacksPerConv)
	}

	id := uuid.New().String()
	now := m.now()
	info := CallbackInfo{
		ID:             id,
		ConversationID: conversationID,
		Delay:          delay.String(),
		Prompt:         prompt,
		State:          CallbackStatePending,
		FiresAt:        now.Add(delay),
		CreatedAt:      now,
	}

	timer := time.AfterFunc(delay, func() {
		m.fire(id)
	})

	m.callbacks[id] = &pendingCallback{info: info, timer: timer}
	m.byConv[conversationID] = append(m.byConv[conversationID], id)

	return info, nil
}

// Cancel cancels a pending callback.
func (m *CallbackManager) Cancel(id string) (CallbackInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cb, ok := m.callbacks[id]
	if !ok {
		return CallbackInfo{}, fmt.Errorf("callback %s not found", id)
	}

	switch cb.info.State {
	case CallbackStateFired:
		return CallbackInfo{}, fmt.Errorf("callback %s already fired", id)
	case CallbackStateCanceled:
		return CallbackInfo{}, fmt.Errorf("callback %s already canceled", id)
	}

	cb.timer.Stop()
	cb.info.State = CallbackStateCanceled
	return cb.info, nil
}

// List returns all callbacks for a conversation.
func (m *CallbackManager) List(conversationID string) []CallbackInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := m.byConv[conversationID]
	result := make([]CallbackInfo, 0, len(ids))
	for _, id := range ids {
		if cb, ok := m.callbacks[id]; ok {
			result = append(result, cb.info)
		}
	}
	return result
}

// Shutdown stops all pending callbacks and prevents new ones.
func (m *CallbackManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopped = true
	for _, cb := range m.callbacks {
		if cb.info.State == CallbackStatePending {
			cb.timer.Stop()
			cb.info.State = CallbackStateCanceled
		}
	}
}

// fire is called by the timer when a callback is ready.
func (m *CallbackManager) fire(id string) {
	m.mu.Lock()
	cb, ok := m.callbacks[id]
	if !ok || cb.info.State != CallbackStatePending {
		m.mu.Unlock()
		return
	}
	cb.info.State = CallbackStateFired
	convID := cb.info.ConversationID
	prompt := cb.info.Prompt
	m.mu.Unlock()

	// Call StartRun outside the lock to avoid deadlocks
	if err := m.starter.StartRun(prompt, convID); err != nil {
		// Log error but callback is still marked as fired
		log.Printf("callback %s: StartRun error: %v", id, err)
	}
}

// --- Tool Constructors ---

func setDelayedCallbackTool(mgr *CallbackManager) Tool {
	def := Definition{
		Name:        "set_delayed_callback",
		Description: descriptions.Load("set_delayed_callback"),
		Action:      ActionExecute,
		Mutating:    true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"delay": map[string]any{
					"type":        "string",
					"description": "How long to wait before firing the callback. Go duration format: '30s', '5m', '1h30m'. Minimum 5s, maximum 1h.",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "The prompt to use when starting the new run. Should describe what to check or do.",
				},
			},
			"required": []string{"delay", "prompt"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Delay  string `json:"delay"`
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse set_delayed_callback args: %w", err)
		}

		delay, err := time.ParseDuration(args.Delay)
		if err != nil {
			return "", fmt.Errorf("invalid delay format %q: %w", args.Delay, err)
		}

		md, ok := RunMetadataFromContext(ctx)
		if !ok {
			return "", fmt.Errorf("set_delayed_callback: no run metadata in context")
		}

		info, err := mgr.Set(md.ConversationID, delay, args.Prompt)
		if err != nil {
			return "", fmt.Errorf("set_delayed_callback failed: %w", err)
		}

		return marshalToolResult(info)
	}

	return Tool{Definition: def, Handler: handler}
}

func cancelDelayedCallbackTool(mgr *CallbackManager) Tool {
	def := Definition{
		Name:        "cancel_delayed_callback",
		Description: descriptions.Load("cancel_delayed_callback"),
		Action:      ActionExecute,
		Mutating:    true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"callback_id": map[string]any{
					"type":        "string",
					"description": "The ID of the callback to cancel.",
				},
			},
			"required": []string{"callback_id"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			CallbackID string `json:"callback_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse cancel_delayed_callback args: %w", err)
		}

		info, err := mgr.Cancel(args.CallbackID)
		if err != nil {
			return "", fmt.Errorf("cancel_delayed_callback failed: %w", err)
		}

		return marshalToolResult(info)
	}

	return Tool{Definition: def, Handler: handler}
}

func listDelayedCallbacksTool(mgr *CallbackManager) Tool {
	def := Definition{
		Name:         "list_delayed_callbacks",
		Description:  descriptions.Load("list_delayed_callbacks"),
		Action:       ActionList,
		ParallelSafe: true,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		md, ok := RunMetadataFromContext(ctx)
		if !ok {
			return "", fmt.Errorf("list_delayed_callbacks: no run metadata in context")
		}

		callbacks := mgr.List(md.ConversationID)
		return marshalToolResult(callbacks)
	}

	return Tool{Definition: def, Handler: handler}
}
