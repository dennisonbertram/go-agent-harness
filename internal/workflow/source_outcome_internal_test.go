package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveSourceWorkflowOutcomePrecedence(t *testing.T) {
	t.Parallel()

	waitErr := errors.New("exit status 7")
	protocolErr := errors.New("malformed protocol")
	initialWriteErr := errors.New("initial start write failed")
	result := map[string]any{"ok": true}

	tests := []struct {
		name                       string
		result                     any
		deadlineExceeded           bool
		protocolErr                error
		initialWriteErr            error
		initialWriteKillCausedWait bool
		closeErr                   error
		waitErr                    error
		wantResult                 any
		wantError                  string
		wantErrorIs                error
	}{
		{
			name:                       "deadline precedes protocol process transport and cleanup errors",
			result:                     result,
			deadlineExceeded:           true,
			protocolErr:                protocolErr,
			initialWriteErr:            initialWriteErr,
			initialWriteKillCausedWait: true,
			closeErr:                   syscall.EPIPE,
			waitErr:                    waitErr,
			wantError:                  `workflow "test-workflow" timed out after 3s`,
		},
		{
			name:                       "initial start write precedes its cleanup kill",
			initialWriteErr:            initialWriteErr,
			initialWriteKillCausedWait: true,
			waitErr:                    errors.New("signal: killed"),
			wantError:                  initialWriteErr.Error(),
			wantErrorIs:                initialWriteErr,
		},
		{
			name:            "protocol precedes process transport and cleanup errors",
			result:          result,
			protocolErr:     protocolErr,
			initialWriteErr: initialWriteErr,
			closeErr:        syscall.EPIPE,
			waitErr:         waitErr,
			wantError:       protocolErr.Error(),
			wantErrorIs:     protocolErr,
		},
		{
			name:            "process exit precedes initial write and stdin close and includes stderr",
			result:          result,
			initialWriteErr: initialWriteErr,
			closeErr:        syscall.EPIPE,
			waitErr:         waitErr,
			wantError:       `workflow "test-workflow" exited: exit status 7: child stderr diagnostic`,
			wantErrorIs:     waitErr,
		},
		{
			name:            "clean child still surfaces initial start write error",
			initialWriteErr: initialWriteErr,
			wantError:       initialWriteErr.Error(),
			wantErrorIs:     initialWriteErr,
		},
		{
			name:        "successful child still surfaces stdin close error",
			result:      result,
			closeErr:    syscall.EPIPE,
			wantError:   syscall.EPIPE.Error(),
			wantErrorIs: syscall.EPIPE,
		},
		{
			name:      "clean exit without result remains an error",
			wantError: `workflow "test-workflow" exited without a result`,
		},
		{
			name:       "successful result is unchanged",
			result:     result,
			wantResult: result,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSourceWorkflowOutcome(sourceWorkflowOutcome{
				result:                     tt.result,
				workflowName:               "test-workflow",
				timeout:                    3 * time.Second,
				deadlineExceeded:           tt.deadlineExceeded,
				protocolErr:                tt.protocolErr,
				initialWriteErr:            tt.initialWriteErr,
				initialWriteKillCausedWait: tt.initialWriteKillCausedWait,
				closeErr:                   tt.closeErr,
				waitErr:                    tt.waitErr,
				stderr:                     "child stderr diagnostic",
			})
			require.Equal(t, tt.wantResult, got)
			if tt.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantError)
			if tt.wantErrorIs != nil {
				require.ErrorIs(t, err, tt.wantErrorIs)
			}
		})
	}
}

func TestResolveSourceWorkflowOutcomeBoundsProcessStderr(t *testing.T) {
	t.Parallel()

	stderr := strings.Repeat("x", maxWorkflowStderrBytes) + "secret-tail"
	_, err := resolveSourceWorkflowOutcome(sourceWorkflowOutcome{
		workflowName:    "test-workflow",
		timeout:         time.Second,
		initialWriteErr: errors.New("initial start write failed"),
		closeErr:        syscall.EPIPE,
		waitErr:         errors.New("exit status 7"),
		stderr:          stderr,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "...[truncated]")
	require.NotContains(t, err.Error(), "secret-tail")
	require.LessOrEqual(t, len(err.Error()), maxWorkflowStderrBytes+128)
}

func TestSourceManagerRunWorkflowInitialStartWriteReapsChildExit(t *testing.T) {
	root := t.TempDir()
	readyPath := filepath.Join(root, "child-ready.fifo")
	lockPath := filepath.Join(root, "child-exit.lock")
	require.NoError(t, syscall.Mkfifo(readyPath, 0o600))

	manager, err := NewSourceManager(SourceManagerOptions{
		Engine:       NewEngine(EngineOptions{}),
		WorkflowDirs: []string{filepath.Join(root, "global"), filepath.Join(root, "workspace")},
		CacheDir:     filepath.Join(root, "cache"),
		ModuleRoot:   findModuleRoot(),
	})
	require.NoError(t, err)

	source := fmt.Sprintf(`package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	lock, err := os.OpenFile(%q, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		panic(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		panic(err)
	}
	fmt.Fprint(os.Stderr, "child stderr diagnostic")
	ready, err := os.OpenFile(%q, os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	if _, err := ready.Write([]byte{1}); err != nil {
		panic(err)
	}
	if err := ready.Close(); err != nil {
		panic(err)
	}
	os.Exit(7)
}
`, lockPath, readyPath)
	bundle, err := manager.CreateWorkflow(context.Background(), CreateWorkflowRequest{
		Name:        "initial-write-exit",
		Description: "Exits before the initial start write.",
		Source:      source,
		Scope:       "workspace",
	})
	require.NoError(t, err)

	childPID := 0
	result, err := manager.runSourceWorkflowWithBeforeInitialWrite(
		&Context{ctx: context.Background(), Args: map[string]any{"input": "value"}},
		bundle,
		func(cmd *exec.Cmd) {
			childPID = cmd.Process.Pid
			ready, openErr := os.OpenFile(readyPath, os.O_RDONLY, 0)
			require.NoError(t, openErr)
			defer ready.Close()
			var signal [1]byte
			_, readErr := io.ReadFull(ready, signal[:])
			require.NoError(t, readErr)

			lock, lockErr := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
			require.NoError(t, lockErr)
			defer lock.Close()
			require.NoError(t, syscall.Flock(int(lock.Fd()), syscall.LOCK_EX))
			defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
		},
	)
	require.Nil(t, result)
	require.ErrorContains(t, err, `workflow "initial-write-exit" exited: exit status 7`)
	require.ErrorContains(t, err, "child stderr diagnostic")
	require.NotErrorIs(t, err, syscall.EPIPE)
	require.Positive(t, childPID)
	require.ErrorIs(t, syscall.Kill(childPID, 0), syscall.ESRCH, "child process must be reaped")
}

func TestSourceManagerRunWorkflowInitialStartWriteCleansLiveChildAndPreservesWriteError(t *testing.T) {
	root := t.TempDir()
	readyPath := filepath.Join(root, "child-ready.fifo")
	require.NoError(t, syscall.Mkfifo(readyPath, 0o600))

	manager, err := NewSourceManager(SourceManagerOptions{
		Engine:       NewEngine(EngineOptions{}),
		WorkflowDirs: []string{filepath.Join(root, "global"), filepath.Join(root, "workspace")},
		CacheDir:     filepath.Join(root, "cache"),
		ModuleRoot:   findModuleRoot(),
	})
	require.NoError(t, err)

	source := fmt.Sprintf(`package main

import (
	"os"
	"syscall"
	"time"
)

func main() {
	if err := syscall.Close(0); err != nil {
		panic(err)
	}
	ready, err := os.OpenFile(%q, os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	if _, err := ready.Write([]byte{1}); err != nil {
		panic(err)
	}
	if err := ready.Close(); err != nil {
		panic(err)
	}
	for {
		time.Sleep(time.Hour)
	}
}
`, readyPath)
	bundle, err := manager.CreateWorkflow(context.Background(), CreateWorkflowRequest{
		Name:        "initial-write-live-child",
		Description: "Closes stdin but remains live.",
		Source:      source,
		Scope:       "workspace",
	})
	require.NoError(t, err)

	childPID := 0
	result, err := manager.runSourceWorkflowWithBeforeInitialWrite(
		&Context{ctx: context.Background(), Args: map[string]any{"input": "value"}},
		bundle,
		func(cmd *exec.Cmd) {
			childPID = cmd.Process.Pid
			ready, openErr := os.OpenFile(readyPath, os.O_RDONLY, 0)
			require.NoError(t, openErr)
			defer ready.Close()
			var signal [1]byte
			_, readErr := io.ReadFull(ready, signal[:])
			require.NoError(t, readErr)
		},
	)
	require.Nil(t, result)
	require.ErrorIs(t, err, syscall.EPIPE)
	require.NotContains(t, err.Error(), "signal: killed")
	require.Positive(t, childPID)
	require.ErrorIs(t, syscall.Kill(childPID, 0), syscall.ESRCH, "child process must be reaped")
}
