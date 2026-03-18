package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMainExecutesRootCommand(t *testing.T) {
	origNewRootCmd := newRootCmdFunc
	origExit := exitFunc
	origStderr := stderr
	defer func() {
		newRootCmdFunc = origNewRootCmd
		exitFunc = origExit
		stderr = origStderr
	}()

	executed := false
	newRootCmdFunc = func() *cobra.Command {
		cmd := &cobra.Command{
			Use: "trainerd",
			RunE: func(_ *cobra.Command, _ []string) error {
				executed = true
				return nil
			},
		}
		cmd.SetArgs(nil)
		return cmd
	}

	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	stderr = &bytes.Buffer{}

	main()

	if !executed {
		t.Fatal("expected root command to execute")
	}
	if exitCode != -1 {
		t.Fatalf("expected main not to exit on success, got %d", exitCode)
	}
}

func TestMainPrintsErrorAndExits(t *testing.T) {
	origNewRootCmd := newRootCmdFunc
	origExit := exitFunc
	origStderr := stderr
	defer func() {
		newRootCmdFunc = origNewRootCmd
		exitFunc = origExit
		stderr = origStderr
	}()

	newRootCmdFunc = func() *cobra.Command {
		cmd := &cobra.Command{
			Use: "trainerd",
			RunE: func(_ *cobra.Command, _ []string) error {
				return errors.New("boom")
			},
		}
		cmd.SetArgs(nil)
		return cmd
	}

	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	var errBuf bytes.Buffer
	stderr = &errBuf

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(errBuf.String(), "Error: boom") {
		t.Fatalf("expected stderr to contain cobra error, got %q", errBuf.String())
	}
}
