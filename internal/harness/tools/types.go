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

	MCPRegistry  MCPRegistry
	AgentRunner  AgentRunner
	WebFetcher   WebFetcher
	Sourcegraph  SourcegraphConfig
	EnableTodos  bool
	EnableLSP    bool
	EnableMCP    bool
	EnableAgent  bool
	EnableWebOps bool
	EnableSkills    bool
	SkillLister     SkillLister
	SkillVerifier   SkillVerifier
	ModelCatalog    *catalog.Catalog
	EnableCron      bool
	CronClient      CronClient
	CallbackManager *CallbackManager
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
// It provides the file path of a skill's SKILL.md for structural validation,
// and allows marking a skill as verified in the underlying store.
type SkillVerifier interface {
	SkillLister
	// GetSkillFilePath returns the absolute path to the skill's SKILL.md file.
	GetSkillFilePath(name string) (string, bool)
	// UpdateSkillVerification updates the verified status of a skill.
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
// The streamer, if present, receives incremental output chunks as they are produced
// by a running tool. Callers that do not support streaming may omit it.
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
