package systemprompt

import "time"

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
	RunStartedAt time.Time
	Now          time.Time
	Step         int
}

type Warning struct {
	Code    string
	Message string
}

type ResolvedPrompt struct {
	StaticPrompt         string
	ResolvedIntent       string
	ResolvedModelProfile string
	ModelFallback        bool
	Behaviors            []string
	Talents              []string
	Warnings             []Warning
}

type Engine interface {
	Resolve(req ResolveRequest) (ResolvedPrompt, error)
	RuntimeContext(in RuntimeContextInput) string
}
