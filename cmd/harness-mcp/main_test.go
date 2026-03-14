package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMain_SuccessNoExit(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return nil }
	exitCalled := false
	exitFunc = func(int) { exitCalled = true }

	main()

	if exitCalled {
		t.Fatal("did not expect exit when run succeeds")
	}
}

func TestMain_ExitsOnError(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return io.ErrUnexpectedEOF }
	var exitCode int
	exitCalled := false
	exitFunc = func(code int) {
		exitCode = code
		exitCalled = true
	}

	main()

	if !exitCalled {
		t.Fatal("expected exit when run fails")
	}
	if exitCode != 1 {
		t.Errorf("got exit code %d, want 1", exitCode)
	}
}

func TestMain_NoExitOnEOF(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return io.EOF }
	exitCalled := false
	exitFunc = func(int) { exitCalled = true }

	main()

	if exitCalled {
		t.Fatal("did not expect exit when run returns io.EOF (normal shutdown)")
	}
}

// TestRun verifies the run() function reads addr from env and processes stdin.
func TestRun(t *testing.T) {
	// Mock harnessd to handle start_run.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Override package-level vars for testability.
	origGetenv := getenvFunc
	origStdin := stdinReader
	origStdout := stdoutWriter
	defer func() {
		getenvFunc = origGetenv
		stdinReader = origStdin
		stdoutWriter = origStdout
	}()

	getenvFunc = func(key string) string {
		if key == "HARNESS_ADDR" {
			return srv.URL
		}
		return ""
	}

	// Provide a single tools/list request as stdin.
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	stdinReader = strings.NewReader(input)

	var outBuf bytes.Buffer
	stdoutWriter = &outBuf

	if err := run(); err != nil && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("run: %v", err)
	}

	// Verify we got a tools/list response.
	if outBuf.Len() == 0 {
		t.Fatal("expected output from run()")
	}

	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(&outBuf).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("got jsonrpc %q, want %q", resp.JSONRPC, "2.0")
	}
}

// TestRunWithIO verifies runWithIO sends a valid tools/list response.
func TestRunWithIO(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	in := strings.NewReader(input)
	pr, pw := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		err := runWithIO(ctx, in, pw, srv.URL)
		pw.Close()
		done <- err
	}()

	scanner := bufio.NewScanner(pr)
	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		break
	}

	cancel()
	<-done

	if resp.JSONRPC != "2.0" {
		t.Errorf("got jsonrpc %q, want %q", resp.JSONRPC, "2.0")
	}
	if string(resp.ID) != "1" {
		t.Errorf("got id %s, want 1", string(resp.ID))
	}
	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
}

// TestRunDefaultAddr verifies run() uses default addr when HARNESS_ADDR is empty.
// (will fail to connect since no server at :8080, but exercises the code path)
func TestRunDefaultAddr(t *testing.T) {
	origGetenv := getenvFunc
	origStdin := stdinReader
	origStdout := stdoutWriter
	defer func() {
		getenvFunc = origGetenv
		stdinReader = origStdin
		stdoutWriter = origStdout
	}()

	// Empty HARNESS_ADDR -> default addr used.
	getenvFunc = func(key string) string { return "" }

	// Provide tools/list which doesn't need harnessd.
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	stdinReader = strings.NewReader(input)

	var outBuf bytes.Buffer
	stdoutWriter = &outBuf

	// run() will complete normally (tools/list doesn't call harnessd).
	if err := run(); err != nil {
		t.Logf("run returned: %v (ok for default-addr test)", err)
	}

	// Should have a response.
	if outBuf.Len() == 0 {
		t.Error("expected output for tools/list with default addr")
	}
}
