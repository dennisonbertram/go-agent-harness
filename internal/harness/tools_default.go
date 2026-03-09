package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/core"
	"go-agent-harness/internal/harness/tools/deferred"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
)

type DefaultRegistryOptions struct {
	ApprovalMode    ToolApprovalMode
	Policy          ToolPolicy
	AskUserBroker   htools.AskUserQuestionBroker
	AskUserTimeout  time.Duration
	MemoryManager   om.Manager
	AgentRunner     htools.AgentRunner
	SkillLister     htools.SkillLister
	ModelCatalog    *catalog.Catalog
	CronClient      htools.CronClient
	CallbackManager *htools.CallbackManager
	Activations     *ActivationTracker // activation tracker for deferred tools
	Sourcegraph     htools.SourcegraphConfig
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
	approvalMode := htools.ApprovalMode(opts.ApprovalMode)
	if approvalMode == "" {
		approvalMode = htools.ApprovalModeFullAuto
	}
	askTimeout := opts.AskUserTimeout
	if askTimeout <= 0 {
		askTimeout = 5 * time.Minute
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Build shared resources
	jobManager := htools.NewJobManager(workspaceRoot, time.Now)
	policyAdapter := toolPolicyAdapter{policy: opts.Policy}

	buildOpts := htools.BuildOptions{
		WorkspaceRoot:  workspaceRoot,
		ApprovalMode:   approvalMode,
		Policy:         policyAdapter,
		HTTPClient:     httpClient,
		Now:            time.Now,
		AskUserBroker:  opts.AskUserBroker,
		AskUserTimeout: askTimeout,
		MemoryManager:  opts.MemoryManager,
		AgentRunner:    opts.AgentRunner,
		SkillLister:    opts.SkillLister,
		CronClient:     opts.CronClient,
		EnableTodos:    true,
		EnableLSP:      true,
		EnableMCP:      true,
		EnableAgent:    true,
		EnableWebOps:   true,
		ModelCatalog:   opts.ModelCatalog,
		EnableSkills:   opts.SkillLister != nil,
		EnableCron:     opts.CronClient != nil,
		CallbackManager: opts.CallbackManager,
		EnableCallbacks: opts.CallbackManager != nil,
		Sourcegraph:    opts.Sourcegraph,
	}

	activations := opts.Activations
	if activations == nil {
		activations = NewActivationTracker()
	}

	// -- Build core tools --
	coreTools := []htools.Tool{
		core.ReadTool(buildOpts),
		core.WriteTool(buildOpts),
		core.EditTool(buildOpts),
		core.BashTool(jobManager),
		core.JobOutputTool(jobManager),
		core.JobKillTool(jobManager),
		core.LsTool(buildOpts),
		core.GlobTool(buildOpts),
		core.GrepTool(buildOpts),
		core.ApplyPatchTool(buildOpts),
		core.GitStatusTool(buildOpts),
		core.GitDiffTool(buildOpts),
		core.AskUserQuestionTool(opts.AskUserBroker, askTimeout),
		core.ObservationalMemoryTool(buildOpts),
	}

	// -- Build deferred tools --
	var deferredTools []htools.Tool

	// Fetch + download are always available as deferred
	deferredTools = append(deferredTools,
		deferred.FetchTool(httpClient),
		deferred.DownloadTool(buildOpts),
	)

	if buildOpts.EnableTodos {
		deferredTools = append(deferredTools, deferred.TodosTool())
	}
	if buildOpts.EnableLSP {
		deferredTools = append(deferredTools,
			deferred.LspDiagnosticsTool(buildOpts),
			deferred.LspReferencesTool(buildOpts),
			deferred.LspRestartTool(),
		)
	}
	if buildOpts.Sourcegraph.Endpoint != "" {
		deferredTools = append(deferredTools, deferred.SourcegraphTool(buildOpts))
	}
	if buildOpts.EnableMCP && buildOpts.MCPRegistry != nil {
		deferredTools = append(deferredTools,
			deferred.ListMCPResourcesTool(buildOpts.MCPRegistry),
			deferred.ReadMCPResourceTool(buildOpts.MCPRegistry),
		)
		dynamic, err := deferred.DynamicMCPTools(context.Background(), buildOpts.MCPRegistry)
		if err != nil {
			panic(err)
		}
		deferredTools = append(deferredTools, dynamic...)
	}
	if buildOpts.ModelCatalog != nil {
		deferredTools = append(deferredTools, deferred.ListModelsTool(buildOpts.ModelCatalog))
	}
	if buildOpts.EnableSkills && opts.SkillLister != nil {
		deferredTools = append(deferredTools, deferred.SkillTool(opts.SkillLister))
	}
	if buildOpts.EnableAgent && opts.AgentRunner != nil {
		deferredTools = append(deferredTools, deferred.AgentTool(opts.AgentRunner))
		if buildOpts.EnableWebOps && buildOpts.WebFetcher != nil {
			deferredTools = append(deferredTools,
				deferred.AgenticFetchTool(buildOpts.WebFetcher, opts.AgentRunner),
				deferred.WebSearchTool(buildOpts.WebFetcher),
				deferred.WebFetchTool(buildOpts.WebFetcher),
			)
		}
	}
	if buildOpts.EnableCron && opts.CronClient != nil {
		deferredTools = append(deferredTools,
			deferred.CronCreateTool(opts.CronClient),
			deferred.CronListTool(opts.CronClient),
			deferred.CronGetTool(opts.CronClient),
			deferred.CronDeleteTool(opts.CronClient),
			deferred.CronPauseTool(opts.CronClient),
			deferred.CronResumeTool(opts.CronClient),
		)
	}
	if buildOpts.EnableCallbacks && opts.CallbackManager != nil {
		deferredTools = append(deferredTools,
			deferred.SetDelayedCallbackTool(opts.CallbackManager),
			deferred.CancelDelayedCallbackTool(opts.CallbackManager),
			deferred.ListDelayedCallbacksTool(opts.CallbackManager),
		)
	}

	// -- Apply policy wrapping to all tools --
	for i := range coreTools {
		coreTools[i].Handler = htools.ApplyPolicy(coreTools[i].Definition, approvalMode, policyAdapter, coreTools[i].Handler)
	}
	for i := range deferredTools {
		deferredTools[i].Handler = htools.ApplyPolicy(deferredTools[i].Definition, approvalMode, policyAdapter, deferredTools[i].Handler)
	}

	// -- Register all tools in the registry --
	registry := NewRegistry()

	for _, t := range coreTools {
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

	for _, t := range deferredTools {
		def := ToolDefinition{
			Name:        t.Definition.Name,
			Description: t.Definition.Description,
			Parameters:  t.Definition.Parameters,
		}
		handler := ToolHandler(func(ctx context.Context, args json.RawMessage) (string, error) {
			return t.Handler(ctx, args)
		})
		if err := registry.RegisterWithOptions(def, handler, RegisterOptions{
			Tier: htools.TierDeferred,
			Tags: t.Definition.Tags,
		}); err != nil {
			panic(err)
		}
	}

	// -- Create find_tool meta-tool if there are deferred tools --
	if len(deferredTools) > 0 {
		var deferredDefs []htools.Definition
		for _, t := range deferredTools {
			deferredDefs = append(deferredDefs, t.Definition)
		}
		findTool := htools.FindToolTool(
			&htools.KeywordSearcher{MaxResults: 10},
			deferredDefs,
			activations,
		)
		findTool.Handler = htools.ApplyPolicy(findTool.Definition, approvalMode, policyAdapter, findTool.Handler)
		findDef := ToolDefinition{
			Name:        findTool.Definition.Name,
			Description: findTool.Definition.Description,
			Parameters:  findTool.Definition.Parameters,
		}
		findHandler := ToolHandler(func(ctx context.Context, args json.RawMessage) (string, error) {
			return findTool.Handler(ctx, args)
		})
		if err := registry.Register(findDef, findHandler); err != nil {
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
