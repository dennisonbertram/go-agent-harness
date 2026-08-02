package modelstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Credential reference schemes. A reference names where a secret lives; the
// secret itself never enters the model store.
const (
	SchemeEnv      = "env"
	SchemeKeychain = "keychain"
	SchemeFile     = "file"
)

// KeychainService is the macOS Keychain service name all harness keys share.
// One service with a per-provider account keeps everything under a single
// recognisable entry in Keychain Access.
const KeychainService = "go-harness"

// securityCommand is the minimal process contract used by the Keychain
// implementation. Keeping the boundary small lets normal regression coverage
// verify command arguments, stdin-only secrets, and error translation without
// depending on a logged-in user's Keychain session.
type securityCommand interface {
	SetStdin(io.Reader)
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	Run() error
}

type execSecurityCommand struct{ cmd *exec.Cmd }

func (c *execSecurityCommand) SetStdin(r io.Reader)  { c.cmd.Stdin = r }
func (c *execSecurityCommand) SetStdout(w io.Writer) { c.cmd.Stdout = w }
func (c *execSecurityCommand) SetStderr(w io.Writer) { c.cmd.Stderr = w }
func (c *execSecurityCommand) Run() error            { return c.cmd.Run() }

// Package-private seams are deliberately limited to process construction and
// availability. Production delegates directly to exec.CommandContext; tests
// replace these seams with deterministic fakes rather than touching login
// Keychain state during the standard regression gate.
var (
	keychainAvailable  = KeychainAvailable
	newSecurityCommand = func(ctx context.Context, args ...string) securityCommand {
		return &execSecurityCommand{cmd: exec.CommandContext(ctx, "security", args...)}
	}
)

// KeychainRef builds the reference for a provider's key in the login keychain.
func KeychainRef(provider string) string {
	return fmt.Sprintf("%s:%s/%s", SchemeKeychain, KeychainService, provider)
}

// EnvRef builds a reference to an environment variable.
func EnvRef(name string) string { return SchemeEnv + ":" + name }

// KeychainAvailable reports whether the macOS Keychain can be used here.
// harnessd also runs on Linux, so every keychain path needs this guard and a
// fallback rather than assuming a Mac.
func KeychainAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("security")
	return err == nil
}

func splitRef(ref string) (scheme, rest string, err error) {
	scheme, rest, found := strings.Cut(strings.TrimSpace(ref), ":")
	if !found || scheme == "" || rest == "" {
		return "", "", fmt.Errorf("credential reference %q is not in <scheme>:<target> form", ref)
	}
	return scheme, rest, nil
}

// ResolveCredential reads the secret a reference points at. An empty reference
// yields an empty secret with no error: a provider that needs no credential is
// a normal configuration, not a failure.
func ResolveCredential(ctx context.Context, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", nil
	}
	scheme, target, err := splitRef(ref)
	if err != nil {
		return "", err
	}

	switch scheme {
	case SchemeEnv:
		return os.Getenv(target), nil

	case SchemeFile:
		data, err := os.ReadFile(target)
		if err != nil {
			return "", fmt.Errorf("read credential file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil

	case SchemeKeychain:
		return readKeychain(ctx, target)

	default:
		return "", fmt.Errorf("unknown credential scheme %q", scheme)
	}
}

func keychainParts(target string) (service, account string, err error) {
	service, account, found := strings.Cut(target, "/")
	if !found || service == "" || account == "" {
		return "", "", fmt.Errorf("keychain reference %q is not in <service>/<account> form", target)
	}
	return service, account, nil
}

func readKeychain(ctx context.Context, target string) (string, error) {
	ctx = orBackground(ctx)
	if !keychainAvailable() {
		return "", fmt.Errorf("the macOS Keychain is not available on this host")
	}
	service, account, err := keychainParts(target)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// -w prints only the secret, so nothing else has to be parsed out.
	cmd := newSecurityCommand(ctx, "find-generic-password", "-s", service, "-a", account, "-w")
	var out, stderr bytes.Buffer
	cmd.SetStdout(&out)
	cmd.SetStderr(&stderr)
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if strings.Contains(detail, "could not be found") {
			return "", fmt.Errorf("no keychain entry for %s/%s", service, account)
		}
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("read keychain entry %s/%s: %s", service, account, detail)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// StoreCredential writes a secret to the location a reference names and returns
// the reference to persist.
//
// The keychain path deliberately feeds the secret through stdin rather than
// `security add-generic-password -w <secret>`: an argument is visible in the
// process list to anything running as this user for as long as the command
// lives. `security` prompts for the value twice, so it is written twice.
func StoreCredential(ctx context.Context, ref, secret string) error {
	scheme, target, err := splitRef(ref)
	if err != nil {
		return err
	}

	ctx = orBackground(ctx)
	switch scheme {
	case SchemeKeychain:
		if !keychainAvailable() {
			return fmt.Errorf("the macOS Keychain is not available on this host")
		}
		service, account, err := keychainParts(target)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		// -U updates an existing entry instead of failing on a duplicate.
		cmd := newSecurityCommand(ctx, "add-generic-password", "-U", "-s", service, "-a", account, "-w")
		cmd.SetStdin(strings.NewReader(secret + "\n" + secret + "\n"))
		var stderr bytes.Buffer
		cmd.SetStderr(&stderr)
		if err := cmd.Run(); err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = err.Error()
			}
			return fmt.Errorf("write keychain entry %s/%s: %s", service, account, detail)
		}
		return nil

	case SchemeFile:
		// Write a fresh 0600 file and rename over the target. os.WriteFile
		// leaves an existing file's mode alone, so updating a key that had
		// been created group-readable kept it readable; and writing in place
		// truncates first, so a concurrent read could see an empty key and a
		// failed write destroyed the previous working one.
		temp := target + ".tmp"
		if err := os.WriteFile(temp, []byte(secret), 0o600); err != nil {
			return fmt.Errorf("write credential file: %w", err)
		}
		if err := os.Rename(temp, target); err != nil {
			os.Remove(temp)
			return fmt.Errorf("commit credential file: %w", err)
		}
		return nil

	case SchemeEnv:
		// An environment variable belongs to the process that launched the
		// daemon; writing one here would vanish on restart and read as the
		// save having silently failed.
		return fmt.Errorf(
			"cannot write to an environment variable — set %s in the daemon's environment, "+
				"or choose keychain storage instead", target)

	default:
		return fmt.Errorf("unknown credential scheme %q", scheme)
	}
}

// DeleteCredential removes a stored secret. Deleting something already absent
// is not an error — the desired end state is the same either way.
func DeleteCredential(ctx context.Context, ref string) error {
	scheme, target, err := splitRef(ref)
	if err != nil {
		return err
	}
	ctx = orBackground(ctx)
	switch scheme {
	case SchemeKeychain:
		if !keychainAvailable() {
			return nil
		}
		service, account, err := keychainParts(target)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		cmd := newSecurityCommand(ctx, "delete-generic-password", "-s", service, "-a", account)
		_ = cmd.Run() // absent entry exits non-zero; that is the desired state
		return nil
	case SchemeFile:
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove credential file: %w", err)
		}
		return nil
	case SchemeEnv:
		return nil
	default:
		return fmt.Errorf("unknown credential scheme %q", scheme)
	}
}

// orBackground tolerates a nil context. Deriving a timeout from nil panics,
// and a credential lookup is exactly the kind of call a caller makes from
// start-up code where no context is at hand — a panic there takes the whole
// daemon down.
func orBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
