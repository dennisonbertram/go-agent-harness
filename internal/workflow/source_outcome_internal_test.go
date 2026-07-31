package workflow

import (
	"errors"
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
	result := map[string]any{"ok": true}

	tests := []struct {
		name             string
		result           any
		deadlineExceeded bool
		protocolErr      error
		closeErr         error
		waitErr          error
		wantResult       any
		wantError        string
		wantErrorIs      error
	}{
		{
			name:             "deadline precedes protocol process and close errors",
			result:           result,
			deadlineExceeded: true,
			protocolErr:      protocolErr,
			closeErr:         syscall.EPIPE,
			waitErr:          waitErr,
			wantError:        `workflow "test-workflow" timed out after 3s`,
		},
		{
			name:        "protocol precedes process and close errors",
			result:      result,
			protocolErr: protocolErr,
			closeErr:    syscall.EPIPE,
			waitErr:     waitErr,
			wantError:   protocolErr.Error(),
			wantErrorIs: protocolErr,
		},
		{
			name:        "process exit precedes stdin close and includes stderr",
			result:      result,
			closeErr:    syscall.EPIPE,
			waitErr:     waitErr,
			wantError:   `workflow "test-workflow" exited: exit status 7: child stderr diagnostic`,
			wantErrorIs: waitErr,
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
				result:           tt.result,
				workflowName:     "test-workflow",
				timeout:          3 * time.Second,
				deadlineExceeded: tt.deadlineExceeded,
				protocolErr:      tt.protocolErr,
				closeErr:         tt.closeErr,
				waitErr:          tt.waitErr,
				stderr:           "child stderr diagnostic",
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
		workflowName: "test-workflow",
		timeout:      time.Second,
		closeErr:     syscall.EPIPE,
		waitErr:      errors.New("exit status 7"),
		stderr:       stderr,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "...[truncated]")
	require.NotContains(t, err.Error(), "secret-tail")
	require.LessOrEqual(t, len(err.Error()), maxWorkflowStderrBytes+128)
}
