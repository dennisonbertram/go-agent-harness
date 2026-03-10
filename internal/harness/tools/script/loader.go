// Package script provides discovery and loading of script-based tools.
// Scripts live in a configured directory (default ~/.go-harness/tools/) with one
// subdirectory per tool. Each subdirectory must contain a tool.json manifest and
// an executable run script (run.sh, run.py, run.js, or run).
//
// The script contract:
//   - JSON arguments are written to stdin.
//   - The result string is read from stdout.
//   - Exit code 0 = success, non-zero = error (stderr is included in the error message).
package script

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	tools "go-agent-harness/internal/harness/tools"
)

const (
	defaultTimeoutSeconds = 30
	maxTimeoutSeconds     = 300
)

// ScriptToolDef holds the parsed content of a tool.json manifest.
type ScriptToolDef struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Parameters     map[string]any `json:"parameters"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}

// candidateRunFiles lists the file names (in priority order) that are accepted
// as the companion executable for a tool directory.
var candidateRunFiles = []string{"run.sh", "run.py", "run.js", "run"}

// LoadScriptTools discovers and loads script-based tools from toolsDir.
// Returns an empty slice (no error) if toolsDir does not exist.
// Individual tools that fail validation are skipped with a log warning.
func LoadScriptTools(toolsDir string) ([]tools.Tool, error) {
	if _, err := os.Stat(toolsDir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil, fmt.Errorf("read script tools dir %s: %w", toolsDir, err)
	}

	var result []tools.Tool
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		toolDir := filepath.Join(toolsDir, entry.Name())
		tool, ok := loadOneScriptTool(toolDir)
		if ok {
			result = append(result, tool)
		}
	}
	return result, nil
}

// loadOneScriptTool attempts to load a single script tool from a directory.
// Returns the tool and true on success; zero value and false on any error (logged as warning).
func loadOneScriptTool(toolDir string) (tools.Tool, bool) {
	manifestPath := filepath.Join(toolDir, "tool.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		log.Printf("script tool warning: reading %s: %v (skipping)", manifestPath, err)
		return tools.Tool{}, false
	}

	var def ScriptToolDef
	if err := json.Unmarshal(data, &def); err != nil {
		log.Printf("script tool warning: parsing %s: %v (skipping)", manifestPath, err)
		return tools.Tool{}, false
	}

	if !isValidToolName(def.Name) {
		log.Printf("script tool warning: invalid tool name %q in %s (skipping)", def.Name, manifestPath)
		return tools.Tool{}, false
	}

	if strings.TrimSpace(def.Description) == "" {
		log.Printf("script tool warning: empty description for %q in %s (skipping)", def.Name, manifestPath)
		return tools.Tool{}, false
	}

	scriptPath, ok := findRunScript(toolDir)
	if !ok {
		log.Printf("script tool warning: no executable run script found in %s (skipping)", toolDir)
		return tools.Tool{}, false
	}

	timeoutSec := def.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = defaultTimeoutSeconds
	}
	if timeoutSec > maxTimeoutSeconds {
		timeoutSec = maxTimeoutSeconds
	}

	params := def.Parameters
	if params == nil {
		params = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	toolDef := tools.Definition{
		Name:         def.Name,
		Description:  def.Description,
		Parameters:   params,
		Action:       tools.ActionExecute,
		Mutating:     true,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"script", "external"},
	}

	handler := makeScriptHandler(scriptPath, timeoutSec, def.Name)
	return tools.Tool{Definition: toolDef, Handler: handler}, true
}

// findRunScript searches for a companion executable in the tool directory.
// Returns the path and true if found; empty string and false otherwise.
func findRunScript(toolDir string) (string, bool) {
	for _, candidate := range candidateRunFiles {
		path := filepath.Join(toolDir, candidate)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if isExecutable(info) {
			return path, true
		}
	}
	// Also check for any file named "run.*" that is executable (besides the candidates above).
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Must start with "run."
		if !strings.HasPrefix(name, "run.") {
			continue
		}
		// Skip names already tried in candidateRunFiles
		alreadyTried := false
		for _, c := range candidateRunFiles {
			if c == name {
				alreadyTried = true
				break
			}
		}
		if alreadyTried {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if isExecutable(info) {
			return filepath.Join(toolDir, name), true
		}
	}
	return "", false
}

// isExecutable reports whether the file described by info has executable bits set.
func isExecutable(info fs.FileInfo) bool {
	return info.Mode()&0o111 != 0
}

// makeScriptHandler returns a ToolHandler that executes the script at scriptPath.
//
// The handler:
//  1. Marshals args JSON to stdin.
//  2. Executes the script with a timeout derived from timeoutSec.
//  3. Returns (stdout, nil) on exit 0.
//  4. Returns ("", error with stderr) on non-zero exit.
//
// Scripts inherit only HOME and PATH from the environment; secret env vars
// (e.g. OPENAI_API_KEY) are never forwarded.
//
// The script runs in its own process group so that all child processes
// (e.g. those spawned by a shell script) are killed on timeout.
func makeScriptHandler(scriptPath string, timeoutSec int, toolName string) tools.Handler {
	timeout := time.Duration(timeoutSec) * time.Second
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		//nolint:gosec // scriptPath comes from a trusted local directory
		cmd := exec.CommandContext(ctx, scriptPath)

		// Place the script in its own process group so we can kill the
		// entire group (including any shell child processes) on timeout.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		// Restrict environment: only HOME and PATH, no secrets.
		cmd.Env = []string{
			"HOME=" + os.Getenv("HOME"),
			"PATH=" + os.Getenv("PATH"),
		}

		// Write JSON args to stdin.
		argsData := []byte(raw)
		cmd.Stdin = bytes.NewReader(argsData)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("script tool %s: start: %w", toolName, err)
		}

		// Wait for the process in a goroutine; kill the process group on timeout.
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-ctx.Done():
			// Kill the entire process group to ensure all children are terminated.
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			<-done // drain
			return "", fmt.Errorf("script tool %s: timed out after %v", toolName, timeout)
		case err := <-done:
			if err != nil {
				stderrStr := strings.TrimSpace(stderr.String())
				if stderrStr != "" {
					return "", fmt.Errorf("script tool %s: %w: %s", toolName, err, stderrStr)
				}
				return "", fmt.Errorf("script tool %s: %w", toolName, err)
			}
		}

		return strings.TrimRight(stdout.String(), "\n"), nil
	}
}

// isValidToolName reports whether name is a valid tool name.
// Valid names contain only letters, digits, and underscores — no spaces,
// path separators, dots, or other special characters.
func isValidToolName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
