package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	commandArgs = []string{"-harness-url", "http://fixture", "-manifest", "fixture.json", "-provenance", "provenance.json"}
	stdout, stderr = &output, &errorsOutput
	runMain = func(ctx context.Context, base, manifest, provenance string) (apisserunner.CoverageReport, error) {
		if ctx == nil || base != "http://fixture" || manifest != "fixture.json" || provenance != "provenance.json" {
			t.Fatalf("main arguments = ctx=%v base=%q manifest=%q provenance=%q", ctx, base, manifest, provenance)
		}
		return apisserunner.CoverageReport{InventoryHash: "hash"}, nil
	}
	exitFunc = func(code int) { t.Fatalf("exit(%d) called for successful report", code) }
	main()
	if output.String() != "{\"inventory_hash\":\"hash\",\"daemon_source_sha\":\"\",\"daemon_command_sha256\":\"\",\"available\":0,\"mapped\":0,\"planned\":0,\"excluded\":null,\"not_applicable\":null,\"missing\":null}\n" || errorsOutput.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", output.String(), errorsOutput.String())
	}
}

func TestMainReportsRunFailureAndExitsOne(t *testing.T) {
	previousArgs, previousRun := commandArgs, runMain
	previousStdout, previousStderr, previousExit := stdout, stderr, exitFunc
	t.Cleanup(func() {
		commandArgs, runMain, stdout, stderr, exitFunc = previousArgs, previousRun, previousStdout, previousStderr, previousExit
	})
	commandArgs = []string{"-manifest", "fixture.json", "-provenance", "provenance.json"}
	stdout, stderr = io.Discard, &bytes.Buffer{}
	runMain = func(context.Context, string, string, string) (apisserunner.CoverageReport, error) {
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
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "manifest.json")
	provenancePath := filepath.Join(dir, "provenance.json")
	commandPath := filepath.Join(dir, "harnessd")
	commandBytes := []byte("fixture daemon")
	if err := os.WriteFile(commandPath, commandBytes, 0o700); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(commandBytes)
	compiled, err := (apisserunner.Runner{BaseURL: server.URL}).LoadLiveInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"inventory_hash":"`+compiled.Hash+`","daemon_source_sha":"fixture-source","mappings":[{"item_id":"tool:fixture","owner":"test","condition":"fixture","disposition":"planned","cohort":"fixture"}],"cases":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provenancePath, []byte(`{"provenance":{"SourceSHA":"fixture-source","Address":"`+server.URL[len("http://"):]+`","CommandPath":"`+commandPath+`","CommandSHA256":"`+hex.EncodeToString(sum[:])+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := run(context.Background(), server.URL, path, provenancePath)
	if err != nil {
		t.Fatal(err)
	}
	if report.InventoryHash == "" || report.Available != 1 || len(report.Missing) != 1 || report.Missing[0] != "tool:fixture" {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunRejectsUntrustedProvenanceBeforeLoadingInventory(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "inventory must not be loaded", http.StatusInternalServerError)
	}))
	defer server.Close()
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"inventory_hash":"fixture","daemon_source_sha":"fixture-source"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	provenancePath := filepath.Join(dir, "provenance.json")
	if err := os.WriteFile(provenancePath, []byte(`{"provenance":{"SourceSHA":"fixture-source","Address":"127.0.0.1:8080","CommandPath":"relative-harnessd","CommandSHA256":"fixture-digest"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(context.Background(), server.URL, manifestPath, provenancePath); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("run error = %v", err)
	}
	if called {
		t.Fatal("run loaded inventory after provenance validation failed")
	}
}
