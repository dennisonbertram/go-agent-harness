package subagents

import (
	"context"
	"fmt"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

// InlineManager wraps a Manager and implements the tools.SubagentManager interface.
// It creates a subagent with inline isolation and waits for completion by polling.
type InlineManager struct {
	m Manager
}

// NewInlineManager wraps a Manager to implement tools.SubagentManager.
func NewInlineManager(m Manager) *InlineManager {
	return &InlineManager{m: m}
}

// CreateAndWait creates an inline subagent and blocks until it completes.
// It polls the manager's Get method until the subagent reaches a terminal status.
func (im *InlineManager) CreateAndWait(ctx context.Context, req tools.SubagentRequest) (tools.SubagentResult, error) {
	// Map isolation mode from profile string to typed constant.
	isolation := IsolationInline
	if req.IsolationMode == string(IsolationWorktree) {
		isolation = IsolationWorktree
	}

	// Map cleanup policy from profile string to typed constant.
	// Default to DestroyOnCompletion for resource hygiene; profiles may override.
	cleanupPolicy := CleanupDestroyOnCompletion
	switch req.CleanupPolicy {
	case "keep":
		cleanupPolicy = CleanupPreserve
	case "delete":
		cleanupPolicy = CleanupDestroyOnCompletion
	case "delete_on_success":
		cleanupPolicy = CleanupDestroyOnSuccess
	}

	saReq := Request{
		Prompt:          req.Prompt,
		Model:           req.Model,
		SystemPrompt:    req.SystemPrompt,
		MaxSteps:        req.MaxSteps,
		MaxCostUSD:      req.MaxCostUSD,
		ReasoningEffort: req.ReasoningEffort,
		AllowedTools:    append([]string(nil), req.AllowedTools...),
		ProfileName:     req.ProfileName,
		Isolation:       isolation,
		CleanupPolicy:   cleanupPolicy,
		BaseRef:         req.BaseRef,
	}

	sa, err := im.m.Create(ctx, saReq)
	if err != nil {
		return tools.SubagentResult{}, fmt.Errorf("create subagent: %w", err)
	}

	// Poll until terminal.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return tools.SubagentResult{}, ctx.Err()
		case <-ticker.C:
			sa, err = im.m.Get(ctx, sa.ID)
			if err != nil {
				return tools.SubagentResult{}, fmt.Errorf("poll subagent: %w", err)
			}
			switch string(sa.Status) {
			case "completed", "failed", "cancelled":
				return tools.SubagentResult{
					ID:     sa.ID,
					RunID:  sa.RunID,
					Status: string(sa.Status),
					Output: sa.Output,
					Error:  sa.Error,
				}, nil
			}
		}
	}
}
