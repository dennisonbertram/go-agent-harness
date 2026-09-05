package modelstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type fakeSecurityCommand struct {
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
	err    error
	out    string
	errOut string
	input  string
}

func (c *fakeSecurityCommand) SetStdin(r io.Reader)  { c.stdin = r }
func (c *fakeSecurityCommand) SetStdout(w io.Writer) { c.stdout = w }
func (c *fakeSecurityCommand) SetStderr(w io.Writer) { c.stderr = w }
func (c *fakeSecurityCommand) Run() error {
	if c.stdin != nil {
		data, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.input = string(data)
	}
	if c.stdout != nil {
		_, _ = io.WriteString(c.stdout, c.out)
	}
	if c.stderr != nil {
		_, _ = io.WriteString(c.stderr, c.errOut)
	}
	return c.err
}

func installFakeSecurityCommand(t *testing.T, command *fakeSecurityCommand) (args func() []string, deadline func() (time.Time, bool)) {
	t.Helper()
	oldAvailable, oldFactory := keychainAvailable, newSecurityCommand
	var capturedArgs []string
	var capturedDeadline time.Time
	var hasDeadline bool
	newSecurityCommand = func(ctx context.Context, got ...string) securityCommand {
		capturedArgs = append([]string(nil), got...)
		capturedDeadline, hasDeadline = ctx.Deadline()
		return command
	}
	keychainAvailable = func() bool { return true }
	t.Cleanup(func() {
		keychainAvailable = oldAvailable
		newSecurityCommand = oldFactory
	})
	return func() []string { return append([]string(nil), capturedArgs...) }, func() (time.Time, bool) { return capturedDeadline, hasDeadline }
}

func TestKeychainCommandContractUsesArgumentsAndStdinOnly(t *testing.T) {
	command := &fakeSecurityCommand{}
	args, deadline := installFakeSecurityCommand(t, command)

	if err := StoreCredential(context.Background(), "keychain:service/account", "secret-value"); err != nil {
		t.Fatalf("store: %v", err)
	}
	if got, want := strings.Join(args(), " "), "add-generic-password -U -s service -a account -w"; got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
	if command.input != "secret-value\nsecret-value\n" {
		t.Fatalf("stdin = %q, want two newline-delimited values", command.input)
	}
	if strings.Contains(strings.Join(args(), " "), "secret-value") {
		t.Fatalf("secret must not be passed as an argument: %q", args())
	}
	until, ok := deadline()
	if !ok || time.Until(until) < 14*time.Second || time.Until(until) > 16*time.Second {
		t.Fatalf("command deadline = %v (present %t), want existing bounded 15 second context", until, ok)
	}
}

func TestKeychainCommandContractReadsAndDeletes(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		command := &fakeSecurityCommand{out: "saved-secret\n"}
		args, _ := installFakeSecurityCommand(t, command)
		got, err := ResolveCredential(context.Background(), "keychain:service/account")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if got != "saved-secret" {
			t.Fatalf("secret = %q", got)
		}
		if want := "find-generic-password -s service -a account -w"; strings.Join(args(), " ") != want {
			t.Fatalf("args = %q, want %q", args(), want)
		}
	})
	t.Run("delete", func(t *testing.T) {
		command := &fakeSecurityCommand{}
		args, _ := installFakeSecurityCommand(t, command)
		if err := DeleteCredential(context.Background(), "keychain:service/account"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if want := "delete-generic-password -s service -a account"; strings.Join(args(), " ") != want {
			t.Fatalf("args = %q, want %q", args(), want)
		}
	})
}

func TestKeychainCommandContractPreservesTimeoutAndErrors(t *testing.T) {
	command := &fakeSecurityCommand{err: context.DeadlineExceeded}
	installFakeSecurityCommand(t, command)
	if err := StoreCredential(context.Background(), "keychain:service/account", "secret"); err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("store error = %v, want surfaced command deadline", err)
	}

	command = &fakeSecurityCommand{err: errors.New("command failed"), errOut: "permission denied"}
	installFakeSecurityCommand(t, command)
	if _, err := ResolveCredential(context.Background(), "keychain:service/account"); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("read error = %v, want security stderr", err)
	}
}

func TestRealKeychainMutationRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("HARNESS_TEST_REAL_KEYCHAIN", "")
	if realKeychainMutationEnabled() {
		t.Fatal("real login-Keychain mutation must be disabled without HARNESS_TEST_REAL_KEYCHAIN=1")
	}
	t.Setenv("HARNESS_TEST_REAL_KEYCHAIN", "1")
	if !realKeychainMutationEnabled() {
		t.Fatal("HARNESS_TEST_REAL_KEYCHAIN=1 must enable the named host-live lane")
	}
}

func TestRealKeychainAccountIsUniqueToThisProcess(t *testing.T) {
	first := realKeychainAccount(t, "round-trip")
	second := realKeychainAccount(t, "provider-save")
	if first == second || !strings.Contains(first, "round-trip") || !strings.Contains(second, "provider-save") {
		t.Fatalf("accounts must retain test identity and differ: %q, %q", first, second)
	}
	if !strings.Contains(first, "-") {
		t.Fatalf("account %q lacks a process-unique suffix", first)
	}
}

func TestKeychainFakeDoesNotNeedRealCommand(t *testing.T) {
	command := &fakeSecurityCommand{out: "x\n"}
	installFakeSecurityCommand(t, command)
	if got, err := ResolveCredential(context.Background(), "keychain:service/account"); err != nil || got != "x" {
		t.Fatalf("fake resolve = %q, %v", got, err)
	}
	if command.stdout == nil {
		t.Fatal("fake contract must receive stdout writer")
	}
	if _, ok := command.stdout.(*bytes.Buffer); !ok {
		t.Fatalf("stdout type = %T, want *bytes.Buffer", command.stdout)
	}
}

// The standard suite does not run security(1), but it must still cover the
// production adapter that binds streams to an exec.Cmd. Constructing a command
// without running it is deterministic and does not touch the login Keychain.
func TestExecSecurityCommandWiresStreamsWithoutRunningSecurity(t *testing.T) {
	oldFactory := newSecurityCommand
	newSecurityCommand = func(ctx context.Context, args ...string) securityCommand {
		return &execSecurityCommand{cmd: exec.CommandContext(ctx, "security", args...)}
	}
	t.Cleanup(func() { newSecurityCommand = oldFactory })

	command := newSecurityCommand(context.Background(), "find-generic-password")
	command.SetStdin(strings.NewReader("secret\n"))
	command.SetStdout(&bytes.Buffer{})
	command.SetStderr(&bytes.Buffer{})
}

// TestExecSecurityCommandRunExecutesTheUnderlyingProcess covers
// execSecurityCommand.Run itself. On this dev machine (darwin) that method
// happens to get exercised incidentally by a real, un-faked
// ResolveCredential(keychain:...) call elsewhere in this package, because
// KeychainAvailable() is true here — but on the Linux CI runner
// KeychainAvailable() is false (runtime.GOOS != "darwin"), so every keychain
// path short-circuits before newSecurityCommand ever runs a real process, and
// Run itself never executes. That is why the nightly coverage gate reported
// it as zero-coverage. This test builds an execSecurityCommand directly with
// a harmless, portable binary instead of "security", bypassing the
// KeychainAvailable gate so the adapter's Run method is exercised on every
// platform the gate runs on.
func TestExecSecurityCommandRunExecutesTheUnderlyingProcess(t *testing.T) {
	var out bytes.Buffer
	command := &execSecurityCommand{cmd: exec.Command("echo", "-n", "adapter-ran")}
	command.SetStdout(&out)
	if err := command.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.String() != "adapter-ran" {
		t.Fatalf("Run() stdout = %q, want %q", out.String(), "adapter-ran")
	}

	failing := &execSecurityCommand{cmd: exec.Command("false")}
	if err := failing.Run(); err == nil {
		t.Fatal("Run() error = nil for a command that exits non-zero, want an error")
	}
}
