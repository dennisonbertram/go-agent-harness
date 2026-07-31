package cron

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateExecutionConfig rejects executable jobs that cannot safely run.
// The execution type is authoritative: shell jobs require a non-empty command
// and harness jobs require a non-empty prompt.
func ValidateExecutionConfig(execType, execConfig string) error {
	switch execType {
	case ExecTypeShell, "":
		var cfg struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(execConfig), &cfg); err != nil {
			return fmt.Errorf("shell execution_config must be valid JSON: %w", err)
		}
		if strings.TrimSpace(cfg.Command) == "" {
			return fmt.Errorf("shell execution_config requires a non-empty command")
		}
	case ExecTypeHarness:
		var cfg struct {
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(execConfig), &cfg); err != nil {
			return fmt.Errorf("harness execution_config must be valid JSON: %w", err)
		}
		if strings.TrimSpace(cfg.Prompt) == "" {
			return fmt.Errorf("harness execution_config requires a non-empty prompt")
		}
	default:
		return fmt.Errorf("execution_type must be shell or harness")
	}
	return nil
}
