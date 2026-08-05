package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/acceptance/apisserunner"
)

func TestMainRunsHashBoundReportWithoutExit(t *testing.T) {
	previousArgs, previousRun := commandArgs, runMain
	previousStdout, previousStderr, previousExit := stdout, stderr, exitFunc
	t.Cleanup(func() {
		commandArgs, runMain, stdout, stderr, exitFunc = previousArgs, previousRun, previousStdout, previousStderr, previousExit
	})
	var output, errorsOutput bytes.Buffer
	commandArgs = []string{"-harness-url", "http://fixture", "-manifest", "fixture.json"}
	stdout, stderr = &output, &errorsOutput
	runMain = func(ctx context.Context, base, manifest string) (apisserunner.CoverageReport, error) {
		if ctx == nil || base != "http://fixture" || manifest != "fixture.json" {
			t.Fatalf("main arguments = ctx=%v base=%q manifest=%q", ctx, base, manifest)
		}
		return apisserunner.CoverageReport{InventoryHash: "hash"}, nil
	}
	exitFunc = func(code int) { t.Fatalf("exit(%d) called for successful report", code) }
	main()
	if output.String() != "{\"inventory_hash\":\"hash\",\"available\":0,\"planned\":0,\"not_applicable\":null,\"missing\":null}\n" || errorsOutput.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", output.String(), errorsOutput.String())
	}
}

func TestMainReportsRunFailureAndExitsOne(t *testing.T) {
	previousArgs, previousRun := commandArgs, runMain
	previousStdout, previousStderr, previousExit := stdout, stderr, exitFunc
	t.Cleanup(func() {
		commandArgs, runMain, stdout, stderr, exitFunc = previousArgs, previousRun, previousStdout, previousStderr, previousExit
	})
	commandArgs = []string{"-manifest", "fixture.json"}
	stdout, stderr = io.Discard, &bytes.Buffer{}
	runMain = func(context.Context, string, string) (apisserunner.CoverageReport, error) {
		return apisserunner.CoverageReport{}, errors.New("fixture failure")
	}
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	main()
	if exitCode != 1 || !bytes.Contains(stderr.(*bytes.Buffer).Bytes(), []byte("fixture failure")) {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr.(*bytes.Buffer).String())
	}
}

func TestRunReportsLiveInventoryGapFromManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tools":[{"name":"fixture","description":"fixture","tier":"core","owner":"test","condition":"fixture"}],"configured_unavailable_toolsets":[],"unavailable":[]}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "manifest.json")
	compiled, err := (apisserunner.Runner{BaseURL: server.URL}).LoadLiveInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"inventory_hash":"`+compiled.Hash+`","cases":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := run(context.Background(), server.URL, path)
	if err != nil {
		t.Fatal(err)
	}
	if report.InventoryHash == "" || report.Available != 1 || len(report.Missing) != 1 || report.Missing[0] != "tool:fixture" {
		t.Fatalf("report = %#v", report)
	}
}
