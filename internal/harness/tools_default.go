package harness

import (
	"context"
	"encoding/json"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
)

type DefaultRegistryOptions struct {
	ApprovalMode    ToolApprovalMode
	Policy          ToolPolicy
	AskUserBroker   htools.AskUserQuestionBroker
	AskUserTimeout  time.Duration
	MemoryManager   om.Manager
	AgentRunner     htools.AgentRunner
	SkillLister     htools.SkillLister
	CallbackManager *htools.CallbackManager
}

func NewDefaultRegistry(workspaceRoot string) *Registry {
	return NewDefaultRegistryWithOptions(workspaceRoot, DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
}

func NewDefaultRegistryWithPolicy(workspaceRoot string, mode ToolApprovalMode, policy ToolPolicy) *Registry {
	return NewDefaultRegistryWithOptions(workspaceRoot, DefaultRegistryOptions{
		ApprovalMode: mode,
		Policy:       policy,
	})
}

func NewDefaultRegistryWithOptions(workspaceRoot string, opts DefaultRegistryOptions) *Registry {
	catalog, err := htools.BuildCatalog(htools.BuildOptions{
		WorkspaceRoot:  workspaceRoot,
		ApprovalMode:   htools.ApprovalMode(opts.ApprovalMode),
		Policy:         toolPolicyAdapter{policy: opts.Policy},
		AskUserBroker:  opts.AskUserBroker,
		AskUserTimeout: opts.AskUserTimeout,
		MemoryManager:  opts.MemoryManager,
		AgentRunner:    opts.AgentRunner,
		SkillLister:    opts.SkillLister,
		EnableTodos:    true,
		EnableLSP:      true,
		EnableMCP:      true,
		EnableAgent:    true,
		EnableWebOps:   true,
		EnableSkills:    opts.SkillLister != nil,
		CallbackManager: opts.CallbackManager,
		EnableCallbacks: opts.CallbackManager != nil,
	})
	if err != nil {
		panic(err)
	}

	registry := NewRegistry()
	for _, t := range catalog {
		def := ToolDefinition{
			Name:        t.Definition.Name,
			Description: t.Definition.Description,
			Parameters:  t.Definition.Parameters,
		}
		handler := ToolHandler(func(ctx context.Context, args json.RawMessage) (string, error) {
			return t.Handler(ctx, args)
		})
		if err := registry.Register(def, handler); err != nil {
			panic(err)
		}
	}
	return registry
}

type toolPolicyAdapter struct {
	policy ToolPolicy
}

func (a toolPolicyAdapter) Allow(ctx context.Context, in htools.PolicyInput) (htools.PolicyDecision, error) {
	if a.policy == nil {
		return htools.PolicyDecision{}, nil
	}
	decision, err := a.policy.Allow(ctx, ToolPolicyInput{
		ToolName:  in.ToolName,
		Action:    string(in.Action),
		Path:      in.Path,
		Arguments: in.Arguments,
		Mutating:  in.Mutating,
	})
	if err != nil {
		return htools.PolicyDecision{}, err
	}
	return htools.PolicyDecision{Allow: decision.Allow, Reason: decision.Reason}, nil
}

// Compatibility helpers kept in harness package for existing tests.
func validateWorkspaceRelativePattern(pattern string) error {
	return htools.ValidateWorkspaceRelativePattern(pattern)
}

func buildLineMatcher(query string, useRegex bool, caseSensitive bool) (func(string) bool, error) {
	return htools.BuildLineMatcher(query, useRegex, caseSensitive)
}

func runCommand(ctx context.Context, timeout time.Duration, command string, args ...string) (string, int, bool, error) {
	return htools.RunCommand(ctx, timeout, command, args...)
}

func isDangerousCommand(command string) bool {
	return htools.IsDangerousCommand(command)
}
