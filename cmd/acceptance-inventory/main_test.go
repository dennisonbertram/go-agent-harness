package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMainRunsSuccessfullyWithoutExit(t *testing.T) {
	previousArgs, previousRun := commandArgs, runMain
	previousStdout, previousStderr, previousExit := stdout, stderr, exitFunc
	t.Cleanup(func() {
		commandArgs, runMain = previousArgs, previousRun
		stdout, stderr, exitFunc = previousStdout, previousStderr, previousExit
	})

	var output, errorsOutput bytes.Buffer
	commandArgs = []string{"-harness-url", "http://fixture"}
	stdout, stderr = &output, &errorsOutput
	runMain = func(out io.Writer, endpoint string) error {
		if endpoint != "http://fixture" {
			t.Fatalf("endpoint = %q", endpoint)
		}
		_, _ = io.WriteString(out, "report")
		return nil
	}
	exitFunc = func(code int) { t.Fatalf("exit(%d) called for successful run", code) }

	main()
	if output.String() != "report" || errorsOutput.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", output.String(), errorsOutput.String())
	}
}

func TestMainReportsFailureAndExitsOne(t *testing.T) {
	previousArgs, previousRun := commandArgs, runMain
	previousStdout, previousStderr, previousExit := stdout, stderr, exitFunc
	t.Cleanup(func() {
		commandArgs, runMain = previousArgs, previousRun
		stdout, stderr, exitFunc = previousStdout, previousStderr, previousExit
	})

	var errorsOutput bytes.Buffer
	commandArgs = nil
	stdout, stderr = io.Discard, &errorsOutput
	runMain = func(io.Writer, string) error { return errors.New("fixture failure") }
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }

	main()
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(errorsOutput.String(), "acceptance-inventory: fixture failure") {
		t.Fatalf("stderr = %q", errorsOutput.String())
	}
}

func TestRunCompilesReportFromRunningToolsBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"tools":[{"name":"read","description":"read","tier":"core","owner":"harness.default.core","condition":"built-in runtime registry"}],
			"configured_unavailable_toolsets":[{"name":"mcp:calendar","owner":"harness.mcp","condition":"calendar configured","provenance":{"source":"runtime.mcp_registry","provider":"calendar","individual_names_known":false}}],
			"unavailable":[{"kind":"toolset","name":"mcp:calendar","owner":"harness.mcp","condition":"calendar configured","reason":"mcp_tool_discovery_failed","provenance":{"source":"runtime.mcp_registry","provider":"calendar","individual_names_known":false}}]
		}`))
	}))
	t.Cleanup(server.Close)

	var output bytes.Buffer
	if err := run(&output, server.URL); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte("tool:read")) || !bytes.Contains(output.Bytes(), []byte("tui_command:resume")) || !bytes.Contains(output.Bytes(), []byte("toolset:mcp:calendar")) || !bytes.Contains(output.Bytes(), []byte("runtime.mcp_registry")) {
		t.Fatalf("report does not reconcile HTTP tools and TUI registry: %s", output.String())
	}
}

func TestRunRejectsMissingOrNullResolverEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "absent", body: `{"tools":[]}`},
		{name: "null", body: `{"tools":[],"configured_unavailable_toolsets":null,"unavailable":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			var output bytes.Buffer
			err := run(&output, server.URL)
			if err == nil || !strings.Contains(err.Error(), "resolver evidence") {
				t.Fatalf("run error = %v, want missing resolver evidence rejection", err)
			}
		})
	}
}

func TestRunAcceptsExplicitEmptyResolverEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tools":[],"configured_unavailable_toolsets":[],"unavailable":[]}`))
	}))
	t.Cleanup(server.Close)

	var output bytes.Buffer
	if err := run(&output, server.URL); err != nil {
		t.Fatalf("run rejected explicit empty resolver evidence: %v", err)
	}
}
