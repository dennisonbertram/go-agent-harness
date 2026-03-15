package tui

import (
	"sort"
	"strings"
	"sync"
)

// Command represents a parsed slash command.
type Command struct {
	Name string   // lowercase, no leading slash
	Args []string // split arguments (may be empty)
	Raw  string   // original text including slash
}

// ParseCommand parses a string like "/clear" or "/help foo bar" into a Command.
// Returns (cmd, true) on success, (zero, false) if input doesn't start with '/'.
// Names are lowercased. Arguments are whitespace-split.
// Empty command name (after trimming) returns (zero, false).
func ParseCommand(input string) (Command, bool) {
	// Must start with '/'
	if !strings.HasPrefix(input, "/") {
		return Command{}, false
	}

	// Strip the leading slash and split on whitespace
	rest := input[1:]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		// "/" or "/  " with only whitespace
		return Command{}, false
	}

	name := strings.ToLower(fields[0])
	if name == "" {
		return Command{}, false
	}

	args := []string{}
	if len(fields) > 1 {
		args = fields[1:]
	}

	return Command{
		Name: name,
		Args: args,
		Raw:  input,
	}, true
}

// CommandEntry describes one registered command.
type CommandEntry struct {
	Name        string
	Aliases     []string
	Description string
	Handler     func(cmd Command) CommandResult
}

// CommandRegistry is the dispatch table for built-in commands.
// It is safe for concurrent use after construction.
type CommandRegistry struct {
	mu      sync.RWMutex
	entries []CommandEntry
	// index maps name and aliases to entry index for O(1) lookup
	index map[string]int
}

// newEmptyCommandRegistry creates an empty registry with no pre-registered commands.
// Useful for testing.
func newEmptyCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		index: make(map[string]int),
	}
}

// NewCommandRegistry creates a registry pre-populated with the built-in command stubs.
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{
		index: make(map[string]int),
	}

	builtins := []CommandEntry{
		{
			Name:        "clear",
			Description: "Clear conversation history",
			Handler: func(cmd Command) CommandResult {
				return CommandResult{Status: CmdOK, Output: "/clear not yet implemented"}
			},
		},
		{
			Name:        "help",
			Description: "Show help dialog",
			Handler: func(cmd Command) CommandResult {
				return CommandResult{Status: CmdOK, Output: "/help not yet implemented"}
			},
		},
		{
			Name:        "context",
			Description: "Show context usage grid",
			Handler: func(cmd Command) CommandResult {
				return CommandResult{Status: CmdOK, Output: "/context not yet implemented"}
			},
		},
		{
			Name:        "stats",
			Description: "Show usage statistics",
			Handler: func(cmd Command) CommandResult {
				return CommandResult{Status: CmdOK, Output: "/stats not yet implemented"}
			},
		},
		{
			Name:        "quit",
			Description: "Quit the TUI",
			Handler: func(cmd Command) CommandResult {
				return CommandResult{Status: CmdOK, Output: "/quit not yet implemented"}
			},
		},
	}

	for _, e := range builtins {
		r.Register(e)
	}

	return r
}

// Register adds a CommandEntry to the registry.
// If an entry with the same Name already exists it is replaced.
// Aliases are also indexed for dispatch.
func (r *CommandRegistry) Register(entry CommandEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if name already exists; if so replace it.
	if idx, ok := r.index[entry.Name]; ok {
		r.entries[idx] = entry
		// Re-index aliases (old aliases may differ, but name stays same)
		for _, alias := range entry.Aliases {
			r.index[alias] = idx
		}
		return
	}

	idx := len(r.entries)
	r.entries = append(r.entries, entry)
	r.index[entry.Name] = idx
	for _, alias := range entry.Aliases {
		r.index[alias] = idx
	}
}

// Dispatch looks up the command by Name in the registry and calls its handler.
// Returns UnknownResult if the command is not found.
func (r *CommandRegistry) Dispatch(cmd Command) CommandResult {
	r.mu.RLock()
	idx, ok := r.index[cmd.Name]
	var handler func(Command) CommandResult
	if ok {
		handler = r.entries[idx].Handler
	}
	r.mu.RUnlock()

	if !ok || handler == nil {
		return UnknownResult(cmd.Name)
	}
	return handler(cmd)
}

// All returns a copy of all registered entries sorted by Name.
func (r *CommandRegistry) All() []CommandEntry {
	r.mu.RLock()
	cp := make([]CommandEntry, len(r.entries))
	copy(cp, r.entries)
	r.mu.RUnlock()

	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Name < cp[j].Name
	})
	return cp
}

// Lookup returns the CommandEntry for the given name (or alias).
// Returns (zero, false) if not found.
func (r *CommandRegistry) Lookup(name string) (CommandEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	idx, ok := r.index[name]
	if !ok {
		return CommandEntry{}, false
	}
	return r.entries[idx], true
}
