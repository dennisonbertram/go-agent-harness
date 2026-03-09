package tools

import "context"

// RunForeground is an exported wrapper for the unexported runForeground method.
// Used by tools/core sub-package.
func (m *JobManager) RunForeground(ctx context.Context, command string, timeoutSeconds int, workingDir string) (map[string]any, error) {
	return m.runForeground(ctx, command, timeoutSeconds, workingDir)
}

// RunBackground is an exported wrapper for the unexported runBackground method.
// Used by tools/core sub-package.
func (m *JobManager) RunBackground(command string, timeoutSeconds int, workingDir string) (map[string]any, error) {
	return m.runBackground(command, timeoutSeconds, workingDir)
}

// Output is an exported wrapper for the unexported output method.
// Used by tools/core sub-package.
func (m *JobManager) Output(shellID string, wait bool) (map[string]any, error) {
	return m.output(shellID, wait)
}

// Kill is an exported wrapper for the unexported kill method.
// Used by tools/core sub-package.
func (m *JobManager) Kill(shellID string) (map[string]any, error) {
	return m.kill(shellID)
}
