package harness

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/core"
	"go-agent-harness/internal/harness/tools/deferred"
	"go-agent-harness/internal/harness/tools/recipe"
	"go-agent-harness/internal/harness/tools/script"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/skills/packs"
)

type DefaultRegistryOptions struct {
	ApprovalMode        ToolApprovalMode
	Policy              ToolPolicy
	AskUserBroker       htools.AskUserQuestionBroker
	AskUserTimeout      time.Duration
	MemoryManager       om.Manager
	AgentRunner         htools.AgentRunner
	SkillLister         htools.SkillLister
	ModelCatalog        *catalog.Catalog
	CronClient          htools.CronClient
	CallbackManager     *htools.CallbackManager
	Activations         *ActivationTracker            // activation tracker for deferred tools
	Sourcegraph         htools.SourcegraphConfig
	MCPConnector        deferred.MCPConnector         // optional: enables the connect_mcp tool
	RecipesDir          string                        // directory to load *.yaml recipe files from
	PromptExtensionDirs htools.PromptExtensionDirs    // directories for create_prompt_extension tool
	PackRegistry        *packs.PackRegistry           // optional skill pack registry
	ScriptToolsDir      string                        // optional: directory containing user script tools
	ConversationStore   ConversationStore             // optional: enables list_conversations and search_conversations
}

// conversationStoreAdapter adapts ConversationStore (harness package) to htools.ConversationReader.
type conversationStoreAdapter struct {
	store ConversationStore
}

func (a *conversationStoreAdapter) ListConversations(ctx context.Context, limit, offset int) ([]htools.ConversationSummary, error) {
	convs, err := a.store.ListConversations(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]htools.ConversationSummary, 0, len(convs))
	for _, c := range convs {
		result = append(result, htools.ConversationSummary{
			ID:        c.ID,
			Title:     c.Title,
			CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
			MsgCount:  c.MsgCount,
		})
	}
	return result, nil
}

func (a *conversationStoreAdapter) SearchConversations(ctx context.Context, query string, limit int) ([]htools.ConversationSearchResult, error) {
	msgs, err := a.store.SearchMessages(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	result := make([]htools.ConversationSearchResult, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, htools.ConversationSearchResult{
			ConversationID: m.ConversationID,
			Role:           m.Role,
			Snippet:        m.Snippet,
		})
	}
	return result, nil
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

	var convReader htools.ConversationReader
	if opts.ConversationStore != nil {
		convReader = &conversationStoreAdapter{store: opts.ConversationStore}
	}

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
		CallbackManager:     opts.CallbackManager,
		EnableCallbacks:     opts.CallbackManager != nil,
		Sourcegraph:         opts.Sourcegraph,
		PromptExtensionDirs: opts.PromptExtensionDirs,
		ConversationStore:   convReader,
		EnableConversations: convReader != nil,
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
		core.ApplyPatchTool(buildOpts),
		core.AskUserQuestionTool(opts.AskUserBroker, askTimeout),
		core.ObservationalMemoryTool(buildOpts),
		core.FileInspectTool(buildOpts),
	}

	// Skill tool: promoted to core with dynamic description containing available skills.
	// Only added when skills are enabled and at least one skill is registered.
	if buildOpts.EnableSkills && opts.SkillLister != nil {
		if skills := opts.SkillLister.ListSkills(); len(skills) > 0 {
			coreTools = append(coreTools, core.SkillTool(opts.SkillLister, opts.AgentRunner))
		}
	}

	// Conversation history tools: enabled when a ConversationStore is provided.
	if buildOpts.EnableConversations && convReader != nil {
		coreTools = append(coreTools,
			core.ListConversationsTool(convReader),
			core.SearchConversationsTool(convReader),
		)
	}

	// -- Build deferred tools --
	var deferredTools []htools.Tool

	// create_prompt_extension is always registered; the handler itself returns an error
	// if the prompt extension directories are not configured.
	deferredTools = append(deferredTools, deferred.CreatePromptExtensionTool(buildOpts.PromptExtensionDirs))

	if buildOpts.EnableTodos {
		coreTools = append(coreTools, deferred.TodosTool())
	}
	// LSP tools removed — bash gopls/go-build are sufficient.
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
	if opts.PackRegistry != nil {
		deferredTools = append(deferredTools, deferred.ManageSkillPacksTool(opts.PackRegistry))
	}

	// -- Load and register recipes as a deferred tool --
	if opts.RecipesDir != "" {
		recipes, err := recipe.LoadRecipes(opts.RecipesDir)
		if err != nil {
			// Log but don't panic — a bad recipe file is not fatal.
			// The tool simply won't be registered.
			_ = err
		} else if len(recipes) > 0 {
			// Build a handler map from all core and deferred tools registered so far.
			handlers := make(recipe.HandlerMap)
			for _, t := range coreTools {
				t := t
				handlers[t.Definition.Name] = t.Handler
			}
			for _, t := range deferredTools {
				t := t
				handlers[t.Definition.Name] = t.Handler
			}
			recipeTool := deferred.RunRecipeTool(handlers, recipes)
			deferredTools = append(deferredTools, recipeTool)
		}
	}

	// -- Load script tools from configured directory --
	if opts.ScriptToolsDir != "" {
		scriptTools, err := script.LoadScriptTools(opts.ScriptToolsDir)
		if err != nil {
			log.Printf("warning: failed to load script tools from %s: %v (continuing without script tools)", opts.ScriptToolsDir, err)
		} else if len(scriptTools) > 0 {
			log.Printf("loaded %d script tool(s) from %s", len(scriptTools), opts.ScriptToolsDir)
			deferredTools = append(deferredTools, scriptTools...)
		}
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

	// -- Register connect_mcp tool (requires the registry itself as the registrar) --
	// This must be done after the registry is created since the tool captures a reference to it.
	if opts.MCPConnector != nil {
		connectTool := deferred.ConnectMCPTool(registry, opts.MCPConnector)
		connectTool.Handler = htools.ApplyPolicy(connectTool.Definition, approvalMode, policyAdapter, connectTool.Handler)
		connectDef := ToolDefinition{
			Name:        connectTool.Definition.Name,
			Description: connectTool.Definition.Description,
			Parameters:  connectTool.Definition.Parameters,
		}
		connectHandler := ToolHandler(func(ctx context.Context, args json.RawMessage) (string, error) {
			return connectTool.Handler(ctx, args)
		})
		if err := registry.RegisterWithOptions(connectDef, connectHandler, RegisterOptions{
			Tier: htools.TierDeferred,
			Tags: connectTool.Definition.Tags,
		}); err != nil {
			panic(err)
		}
		deferredTools = append(deferredTools, connectTool)
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
