// Package scheduledlifecycle provisions the single owned harness daemon used
// by scheduled-conversation acceptance proofs. It intentionally contains no
// cron or callback semantics; callers supply the daemon binary and then attach
// API/SSE and PTY clients to the returned identity.
package scheduledlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Config describes disposable inputs supplied by an acceptance caller. The
// lifecycle owns only resources below ArtifactRoot and the process it starts.
type Config struct {
	Command, SourceRoot, ArtifactRoot, ExpectedSourceSHA, Address string
	Arguments                                                     []string
	Environment                                                   []string
	Timeout                                                       time.Duration
}

// Resources are the isolated paths supplied to the owned daemon. The daemon
// may create its SQLite files lazily; their parent remains lifecycle-owned.
type Resources struct {
	Workspace, ConversationDB, RunDB, CronDB, CallbackDB string
}

// PTYAttachment is the complete daemon identity a TUI driver needs. It keeps
// the later proof from accidentally starting or attaching to another daemon.
type PTYAttachment struct {
	BaseURL, Workspace, ConversationDB, RunDB, CronDB, CallbackDB string
}

// Provenance binds an acceptance artifact to the source and configuration that
// started it.
type Provenance struct {
	SourceSHA, Address, ConfigSHA256, CommandPath, CommandSHA256 string
}

// Lifecycle is a running, owned daemon and the one public identity shared by
// API/SSE and PTY clients.
type Lifecycle struct {
	ArtifactRoot string
	PublicURL    string
	Resources    Resources
	Provenance   Provenance

	command *exec.Cmd
	done    chan struct{}
	waitMu  sync.Mutex
	waitErr error
	log     *os.File
	close   sync.Once

	// signalProcessGroup is test-injected only; production always signals the
	// process group recorded at Start rather than resolving a listener or PID.
	signalProcessGroup func(pid int, signal syscall.Signal) error
}

// Start validates provenance, reserves one listener, and starts exactly one
// daemon with that listener inherited as descriptor 3. A command that does not
// support HARNESS_LISTEN_FD fails readiness rather than silently selecting a
// different daemon or listener.
func Start(ctx context.Context, cfg Config) (*Lifecycle, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	artifactRoot, err := filepath.Abs(cfg.ArtifactRoot)
	if err != nil {
		return nil, fmt.Errorf("absolute artifact root: %w", err)
	}
	if err := os.MkdirAll(artifactRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create artifact root: %w", err)
	}
	sourceSHA, err := sourceSHA(cfg.SourceRoot)
	if err != nil {
		return nil, err
	}
	if cfg.ExpectedSourceSHA != "" && cfg.ExpectedSourceSHA != sourceSHA {
		return nil, fmt.Errorf("source SHA mismatch: got %s, want %s", sourceSHA, cfg.ExpectedSourceSHA)
	}
	commandPath, commandSHA, err := executableIdentity(cfg.Command)
	if err != nil {
		return nil, err
	}

	resources := Resources{
		Workspace:      filepath.Join(artifactRoot, "workspace"),
		ConversationDB: filepath.Join(artifactRoot, "stores", "conversations.db"),
		RunDB:          filepath.Join(artifactRoot, "stores", "runs.db"),
		CronDB:         filepath.Join(artifactRoot, "workspace", ".harness", "cron.db"),
		CallbackDB:     filepath.Join(artifactRoot, "workspace", ".harness", "callbacks.db"),
	}
	if err := os.MkdirAll(filepath.Dir(resources.CallbackDB), 0o700); err != nil {
		return nil, fmt.Errorf("create owned callback store directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(resources.ConversationDB), 0o700); err != nil {
		return nil, fmt.Errorf("create owned store directory: %w", err)
	}

	requestedAddress := cfg.Address
	if requestedAddress == "" {
		requestedAddress = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", requestedAddress)
	if err != nil {
		return nil, fmt.Errorf("reserve listener %q: %w", requestedAddress, err)
	}
	address := listener.Addr().String()
	listenerFile, err := listener.(*net.TCPListener).File()
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("duplicate reserved listener: %w", err)
	}
	if err := listener.Close(); err != nil {
		_ = listenerFile.Close()
		return nil, fmt.Errorf("release parent listener copy: %w", err)
	}

	configHash := configDigest(cfg, sourceSHA, address, resources)
	provenance := Provenance{SourceSHA: sourceSHA, Address: address, ConfigSHA256: configHash, CommandPath: commandPath, CommandSHA256: commandSHA}
	if err := writeProvenance(filepath.Join(artifactRoot, "provenance.json"), provenance, resources); err != nil {
		_ = listenerFile.Close()
		return nil, err
	}
	log, err := os.OpenFile(filepath.Join(artifactRoot, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = listenerFile.Close()
		return nil, fmt.Errorf("create daemon log: %w", err)
	}

	command := exec.Command(cfg.Command, cfg.Arguments...)
	command.Dir = artifactRoot
	command.Stdout, command.Stderr = log, log
	command.ExtraFiles = []*os.File{listenerFile}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Env = append(scrubInheritedResourceEnvironment(os.Environ()), lifecycleEnvironment(cfg.Environment, address, resources)...)
	if err := command.Start(); err != nil {
		_ = listenerFile.Close()
		_ = log.Close()
		return nil, fmt.Errorf("start owned daemon: %w", err)
	}
	_ = listenerFile.Close()
	lifecycle := &Lifecycle{
		ArtifactRoot: artifactRoot,
		PublicURL:    "http://" + address,
		Resources:    resources,
		Provenance:   provenance,
		command:      command,
		done:         make(chan struct{}),
		log:          log,
		signalProcessGroup: func(pid int, signal syscall.Signal) error {
			return syscall.Kill(-pid, signal)
		},
	}
	go func() {
		err := command.Wait()
		lifecycle.waitMu.Lock()
		lifecycle.waitErr = err
		lifecycle.waitMu.Unlock()
		close(lifecycle.done)
	}()
	if err := lifecycle.waitReady(ctx, cfg.Timeout); err != nil {
		_ = lifecycle.Close()
		return nil, err
	}
	return lifecycle, nil
}

// SSEURL returns the conversation-scoped stream on the same daemon as
// PublicURL. The escaped identifier prevents a caller-controlled path escape.
func (l *Lifecycle) SSEURL(conversationID string) string {
	return l.PublicURL + "/v1/conversations/" + url.PathEscape(conversationID) + "/events"
}

// PTY returns the exact daemon and resource identity intended for the TUI
// attachment. It does not start a second daemon.
func (l *Lifecycle) PTY() PTYAttachment {
	return PTYAttachment{
		BaseURL: l.PublicURL, Workspace: l.Resources.Workspace,
		ConversationDB: l.Resources.ConversationDB, RunDB: l.Resources.RunDB,
		CronDB: l.Resources.CronDB, CallbackDB: l.Resources.CallbackDB,
	}
}

// Close terminates only the process group created by Start. It never looks up
// or kills a listener by address, so unrelated processes remain untouched.
func (l *Lifecycle) Close() error {
	var closeErr error
	l.close.Do(func() {
		childExited := false
		select {
		case <-l.done:
			childExited = true
		default:
			if l.command.Process != nil {
				_ = l.signalProcessGroup(l.command.Process.Pid, syscall.SIGTERM)
			}
			select {
			case <-l.done:
				childExited = true
			case <-time.After(2 * time.Second):
				if l.command.Process != nil {
					_ = l.signalProcessGroup(l.command.Process.Pid, syscall.SIGKILL)
				}
				select {
				case <-l.done:
					childExited = true
				case <-time.After(2 * time.Second):
					closeErr = errors.New("owned daemon did not exit after SIGKILL escalation")
				}
			}
		}
		if childExited {
			if err := l.processExitErr(); err != nil && !isExpectedTermination(err) {
				closeErr = err
			}
			if err := l.log.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	})
	return closeErr
}

func (l *Lifecycle) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	client := &http.Client{Timeout: 300 * time.Millisecond}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, l.PublicURL+"/healthz", nil)
		if err == nil {
			response, requestErr := client.Do(request)
			if requestErr == nil {
				_ = response.Body.Close()
				if response.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		select {
		case <-l.done:
			return fmt.Errorf("owned daemon exited before readiness: %w", l.processExitErr())
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("owned daemon did not become healthy within %s", timeout)
		case <-ticker.C:
		}
	}
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Command) == "" {
		return errors.New("daemon command is required")
	}
	if strings.TrimSpace(cfg.SourceRoot) == "" || strings.TrimSpace(cfg.ArtifactRoot) == "" {
		return errors.New("source root and artifact root are required")
	}
	return nil
}

func sourceSHA(root string) (string, error) {
	output, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolve source SHA: %w", err)
	}
	sha := strings.TrimSpace(string(output))
	if sha == "" {
		return "", errors.New("resolve source SHA: empty revision")
	}
	return sha, nil
}

func lifecycleEnvironment(extra []string, address string, resources Resources) []string {
	return append(scrubReservedResourceEnvironment(extra), []string{
		"HARNESS_ADDR=" + address,
		"HARNESS_LISTEN_FD=3",
		"HARNESS_WORKSPACE=" + resources.Workspace,
		"HARNESS_CONVERSATION_DB=" + resources.ConversationDB,
		"HARNESS_RUN_DB=" + resources.RunDB,
		"CRONSD_DB_PATH=" + resources.CronDB,
	}...)
}

func (l *Lifecycle) processExitErr() error {
	l.waitMu.Lock()
	defer l.waitMu.Unlock()
	return l.waitErr
}

func scrubInheritedResourceEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(name, "HARNESS_") || strings.HasPrefix(name, "CRONSD_") {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func scrubReservedResourceEnvironment(environment []string) []string {
	reserved := map[string]struct{}{
		"HARNESS_ADDR": {}, "HARNESS_LISTEN_FD": {}, "HARNESS_WORKSPACE": {},
		"HARNESS_CONVERSATION_DB": {}, "HARNESS_RUN_DB": {}, "CRONSD_DB_PATH": {},
	}
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		name, _, found := strings.Cut(value, "=")
		if found {
			if _, isReserved := reserved[name]; isReserved {
				continue
			}
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func executableIdentity(command string) (string, string, error) {
	path := command
	if !filepath.IsAbs(path) {
		resolved, err := exec.LookPath(path)
		if err != nil {
			return "", "", fmt.Errorf("resolve daemon command %q: %w", command, err)
		}
		path = resolved
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize daemon command %q: %w", command, err)
	}
	bytes, err := os.ReadFile(canonical)
	if err != nil {
		return "", "", fmt.Errorf("read daemon command %q: %w", canonical, err)
	}
	sum := sha256.Sum256(bytes)
	return canonical, hex.EncodeToString(sum[:]), nil
}

func configDigest(cfg Config, sourceSHA, address string, resources Resources) string {
	raw, _ := json.Marshal(struct {
		Command, SourceSHA, Address string
		Arguments, Environment      []string
		Resources                   Resources
	}{cfg.Command, sourceSHA, address, cfg.Arguments, cfg.Environment, resources})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func writeProvenance(path string, provenance Provenance, resources Resources) error {
	payload, err := json.MarshalIndent(struct {
		Provenance Provenance `json:"provenance"`
		Resources  Resources  `json:"resources"`
	}{provenance, resources}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provenance: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write provenance: %w", err)
	}
	return nil
}

func isExpectedTermination(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}
