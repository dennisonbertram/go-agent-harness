package kimi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// CLIRefresher renews the Kimi CLI's token by running the CLI itself.
//
// The harness cannot refresh directly: Kimi rejects its OAuth client, and the
// real client identifier is assembled inside the CLI rather than stored. The
// CLI does refresh, but only when it actually needs a token — listing
// providers does not trigger one, running a prompt does. So the cheapest
// reliable trigger is a one-word prompt.
//
// That costs a tiny completion each time, which is why this only fires when the
// token is genuinely unusable and never more often than MinInterval.
type CLIRefresher struct {
	// Binary is the CLI to invoke. Empty means the default install location.
	Binary string
	// Prompt is the throwaway request used to force a refresh.
	Prompt string
	// Timeout bounds the CLI run; it starts a whole agent, so this is generous.
	Timeout time.Duration
	// MinInterval is the floor between attempts. Without it a provider that is
	// failing for some other reason would spawn a CLI process per request.
	MinInterval time.Duration

	mu          sync.Mutex
	lastAttempt time.Time
	// lastErr is why the last attempt failed, or nil if it succeeded. A waiter
	// that arrives inside MinInterval needs to know which of those happened.
	lastErr error
}

// DefaultCLIPath is where the Kimi installer puts its launcher.
func DefaultCLIPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "kimi"
	}
	return filepath.Join(home, ".kimi-code", "bin", "kimi")
}

// NewCLIRefresher returns a refresher with conservative defaults.
func NewCLIRefresher() *CLIRefresher {
	return &CLIRefresher{
		Binary:      DefaultCLIPath(),
		Prompt:      "ok",
		Timeout:     90 * time.Second,
		MinInterval: 30 * time.Second,
	}
}

// Refresh runs the CLI so it renews its own credential.
//
// Serialised and rate-limited: several requests arriving against an expired
// token must not each start an agent process.
func (r *CLIRefresher) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Callers queue on the mutex, so by the time a waiter gets here the refresh
	// it was waiting for has usually already finished. Report that as success:
	// the waiter's next step is to re-read the file, which now holds the fresh
	// token. Returning an error instead failed every concurrent caller for a
	// full MinInterval even though the credential was already good.
	if !r.lastAttempt.IsZero() && time.Since(r.lastAttempt) < r.MinInterval {
		if r.lastErr == nil {
			return nil
		}
		return fmt.Errorf(
			"a refresh was attempted %s ago and did not help; not retrying yet: %w",
			time.Since(r.lastAttempt).Truncate(time.Second), r.lastErr)
	}
	r.lastAttempt = time.Now()
	r.lastErr = nil

	binary := r.Binary
	if binary == "" {
		binary = DefaultCLIPath()
	}
	if _, err := os.Stat(binary); err != nil {
		r.lastErr = fmt.Errorf("the Kimi CLI is not installed at %s", binary)
		return r.lastErr
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := r.Prompt
	if prompt == "" {
		prompt = "ok"
	}
	cmd := exec.CommandContext(ctx, binary, "-p", prompt)
	// The CLI writes its session transcript to stdout; none of it is wanted.
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		if parent.Err() != nil {
			// The caller went away; that is not a refresher failure and must
			// not be reported as the CLI exceeding its own timeout.
			r.lastErr = fmt.Errorf("refresh cancelled: %w", parent.Err())
		} else if ctx.Err() != nil {
			r.lastErr = fmt.Errorf("the Kimi CLI did not finish within %s", timeout)
		} else {
			r.lastErr = fmt.Errorf("run the Kimi CLI to refresh: %w", err)
		}
		return r.lastErr
	}
	return nil
}

// WithRefresher makes the source renew an expired token instead of reporting
// it, so a subscription that has simply gone idle recovers on its own.
func (s *VendorTokenSource) WithRefresher(r *CLIRefresher) *VendorTokenSource {
	s.refresher = r
	return s
}
