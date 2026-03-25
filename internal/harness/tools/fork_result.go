package tools

import (
	"errors"
	"strings"
)

// ForkResultExecutionError returns a concrete error when a forked child run
// finished with terminal failure information in ForkResult.Error.
func ForkResultExecutionError(result ForkResult) error {
	if strings.TrimSpace(result.Error) == "" {
		return nil
	}
	return errors.New(result.Error)
}
