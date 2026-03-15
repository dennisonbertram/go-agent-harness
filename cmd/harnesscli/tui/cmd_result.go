package tui

// CmdStatus represents the outcome of a command dispatch.
type CmdStatus int

const (
	// CmdOK means the command ran successfully.
	CmdOK CmdStatus = iota
	// CmdError means the command encountered an error.
	CmdError
	// CmdUnknown means the command name was not found in the registry.
	CmdUnknown
)

// CommandResult is returned by command handlers.
type CommandResult struct {
	Status CmdStatus
	Output string // text to display in viewport
	Hint   string // optional user hint (e.g. "Did you mean /clear?")
}

// ErrorResult creates a CmdError result with the given message as Output.
func ErrorResult(msg string) CommandResult {
	return CommandResult{
		Status: CmdError,
		Output: msg,
	}
}

// UnknownResult creates a CmdUnknown result with a hint referencing the attempted command.
func UnknownResult(attempted string) CommandResult {
	return CommandResult{
		Status: CmdUnknown,
		Hint:   "Unknown command: /" + attempted + ". Type /help to see available commands.",
	}
}
