package deferred

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestGitTools_SurfaceGitFailures pins the fix for a silent-failure bug.
//
// tools.RunCommand returns a NIL error for any normal process exit, including a
// non-zero one — the exit code is the signal. These tools checked only the
// returned error, so a git failure ("not a git repository", a bad ref) produced
// an empty result that reads to a model as "nothing found" rather than "this
// did not work". Running them against a directory that is not a git repository
// must now be an explicit error.
func TestGitTools_SurfaceGitFailures(t *testing.T) {
	notARepo := t.TempDir()
	opts := tools.BuildOptions{WorkspaceRoot: notARepo}
	ctx := context.Background()

	cases := []struct {
		name string
		tool tools.Tool
		args string
	}{
		{"git_log_search", GitLogSearchTool(opts), `{"query":"anything"}`},
		{"git_file_history", GitFileHistoryTool(opts), `{"path":"some/file.go"}`},
		{"git_diff_range", GitDiffRangeTool(opts), `{"from":"HEAD~1","to":"HEAD"}`},
		{"git_contributor_context", GitContributorContextTool(opts), `{"path":"some/file.go"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.tool.Handler(ctx, json.RawMessage(tc.args))
			if err == nil {
				t.Fatalf("%s returned no error outside a git repository; output was %q", tc.name, out)
			}
			// The message should point at git, not at some generic failure, so
			// an operator can tell what actually went wrong.
			if !strings.Contains(strings.ToLower(err.Error()), "git") {
				t.Errorf("error %q should mention git", err.Error())
			}
		})
	}
}
