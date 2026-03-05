package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainDelegatesToRunAndExit(t *testing.T) {
	origRun := runCommand
	origExit := exitFunc
	origArgs := osArgs
	defer func() {
		runCommand = origRun
		exitFunc = origExit
		osArgs = origArgs
	}()

	runCommand = func(args []string) int {
		if len(args) != 1 || args[0] != "-x" {
			t.Fatalf("unexpected args: %v", args)
		}
		return 7
	}
	exited := -1
	exitFunc = func(code int) {
		exited = code
	}
	osArgs = []string{"coveragegate", "-x"}

	main()

	if exited != 7 {
		t.Fatalf("expected exit 7, got %d", exited)
	}
}

func TestRunPass(t *testing.T) {
	origReport := coverageReportFn
	origStdout := stdout
	origStderr := stderr
	defer func() {
		coverageReportFn = origReport
		stdout = origStdout
		stderr = origStderr
	}()

	coverageReportFn = func(string) (string, error) {
		return `x/main.go:1: f 100.0%
total: (statements) 85.0%`, nil
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	stdout = &out
	stderr = &errOut

	code := run([]string{"-coverprofile=coverage.out", "-min-total=80"})
	if code != 0 {
		t.Fatalf("expected success exit code, got %d (stderr=%s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "coveragegate: PASS") {
		t.Fatalf("expected PASS output, got %q", out.String())
	}
}

func TestRunValidationFailure(t *testing.T) {
	origReport := coverageReportFn
	origStdout := stdout
	origStderr := stderr
	defer func() {
		coverageReportFn = origReport
		stdout = origStdout
		stderr = origStderr
	}()

	coverageReportFn = func(string) (string, error) {
		return `x/main.go:1: f 0.0%
total: (statements) 85.0%`, nil
	}

	stdout = &bytes.Buffer{}
	var errOut bytes.Buffer
	stderr = &errOut

	code := run([]string{"-coverprofile=coverage.out", "-min-total=80"})
	if code != 1 {
		t.Fatalf("expected failure exit code, got %d", code)
	}
	if !strings.Contains(errOut.String(), "validation failed") {
		t.Fatalf("expected validation failure output, got %q", errOut.String())
	}
}

func TestRunFlagParseFailure(t *testing.T) {
	origStdout := stdout
	origStderr := stderr
	defer func() {
		stdout = origStdout
		stderr = origStderr
	}()

	stdout = &bytes.Buffer{}
	var errOut bytes.Buffer
	stderr = &errOut

	code := run([]string{"-min-total=not-a-number"})
	if code != 1 {
		t.Fatalf("expected parse failure exit code, got %d", code)
	}
	if !strings.Contains(errOut.String(), "parse failed") {
		t.Fatalf("expected parse failure output, got %q", errOut.String())
	}
}

func TestCoverageFuncReportFailure(t *testing.T) {
	report, err := coverageFuncReport("definitely-missing-coverage-file.out")
	if err == nil {
		t.Fatalf("expected error, got report: %s", report)
	}
}
