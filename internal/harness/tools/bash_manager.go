package tools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var dangerousBashPatterns = []string{
	`(?i)\brm\s+-rf\s+/`,
	`(?i)\bshutdown\b`,
	`(?i)\breboot\b`,
	`(?i):\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`,
}

var dangerousBashRegexps = compileDangerousPatterns()

func compileDangerousPatterns() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(dangerousBashPatterns))
	for _, p := range dangerousBashPatterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// isDangerousCommand reports whether a command matches one of the coarse
// dangerous-command patterns above. It lives beside those patterns now; its
// previous home was one of the duplicated tool files removed by the
// single-catalog consolidation. Exported to the rest of the tree as
// IsDangerousCommand (helpers.go).
func isDangerousCommand(command string) bool {
	for _, pattern := range dangerousBashRegexps {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

const defaultMaxStreamLineBytes = 1 << 20

// SudoRegexp matches sudo invocations. The harness runs as root inside Docker
// containers, so sudo is stripped rather than rejected.
var SudoRegexp = regexp.MustCompile(`(?i)\bsudo\s+(?:-[A-Za-z0-9]+\s+)*`)

// StripSudo removes sudo prefix from a command.
func StripSudo(command string) string {
	return SudoRegexp.ReplaceAllString(command, "")
}

type backgroundJob struct {
	id         string
	command    string
	workingDir string
	startedAt  time.Time
	// tenantID is the tenant of the run that started the job, captured from
	// the tool execution context. Empty for unscoped/legacy callers.
	tenantID string
	// conversationID and runID identify where to report the result. A job
	// routinely outlives the run that started it, so the completion has to
	// carry its own origin rather than relying on whatever run is current.
	conversationID string
	runID          string

	stdout *headTailBuffer
	stderr *headTailBuffer

	mu       sync.Mutex
	exitCode int
	done     bool
	timedOut bool
	err      error
	cancel   context.CancelFunc
}

// JobCompletion describes a finished background job.
type JobCompletion struct {
	ShellID    string
	Command    string
	WorkingDir string
	TenantID   string
	// ConversationID and RunID are the job's origin, captured at start. A job
	// commonly finishes after its run has ended, so the completion cannot be
	// attributed by asking what is running now.
	ConversationID string
	RunID          string
	ExitCode       int
	TimedOut       bool
	Output         string
	Truncated      bool
}

// JobEvents receives a background job's completion.
//
// Without this the only way to learn a job had finished was to poll
// job_output with the right shell_id, so a job that completed after its run
// ended told nobody: not the model, which had moved on, and not the UI, which
// had no event to render. A capability that cannot report its own result gets
// chosen again and misleads again.
type JobEvents interface {
	JobCompleted(JobCompletion)
}

type JobManager struct {
	root           string
	nextID         uint64
	mu             sync.RWMutex
	jobs           map[string]*backgroundJob
	closed         bool
	wg             sync.WaitGroup
	maxJobs        int
	ttl            time.Duration
	maxOutputBytes int
	now            func() time.Time
	sandboxScope   SandboxScope  // optional sandbox enforcement
	networkPolicy  NetworkPolicy // optional network policy fallback (issue #1397)
	events         JobEvents
}

// SetJobEvents installs the sink notified when a background job finishes.
// Safe to call before any command is launched; nil disables notification.
func (m *JobManager) SetJobEvents(events JobEvents) {
	m.mu.Lock()
	m.events = events
	m.mu.Unlock()
}

func (m *JobManager) jobEvents() JobEvents {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.events
}

func NewJobManager(workspaceRoot string, now func() time.Time) *JobManager {
	if now == nil {
		now = time.Now
	}
	return &JobManager{
		root:           workspaceRoot,
		jobs:           make(map[string]*backgroundJob),
		maxJobs:        64,
		ttl:            30 * time.Minute,
		maxOutputBytes: defaultMaxCommandOutputBytes,
		now:            now,
	}
}

// SetSandboxScope configures the sandbox scope enforced for all commands run
// via this JobManager.  It is safe to call before any commands are launched.
func (m *JobManager) SetSandboxScope(scope SandboxScope) {
	m.sandboxScope = scope
}

// SetNetworkPolicy configures the fallback network policy enforced for
// commands run via this JobManager when the per-call context carries none
// (issue #1397). It is safe to call before any commands are launched.
func (m *JobManager) SetNetworkPolicy(policy NetworkPolicy) {
	m.networkPolicy = policy
}

func (m *JobManager) runForeground(ctx context.Context, command string, timeoutSeconds int, workingDir string) (map[string]any, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	if timeoutSeconds > 300 {
		timeoutSeconds = 300
	}
	scope := m.sandboxScopeForContext(ctx)
	network := m.networkPolicyForContext(ctx)
	if err := CheckSandboxCommand(scope, network, m.root, command); err != nil {
		return nil, err
	}
	workDir, err := resolveWorkingDir(m.root, workingDir)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(WithNetworkPolicy(ctx, network), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd, sandboxCleanup, sbResult, err := buildSandboxedCommand(timeoutCtx, scope, m.root, command)
	if err != nil {
		return nil, err
	}
	defer sandboxCleanup()
	configureGroupKill(cmd)
	cmd.Dir = workDir

	streamer, hasStreamer := OutputStreamerFromContext(ctx)

	stdout := newHeadTailBuffer(m.maxOutputBytes)
	stderr := newHeadTailBuffer(m.maxOutputBytes)
	var streamErr error
	var streamTruncated bool

	if hasStreamer {
		pr, pw := io.Pipe()
		cmd.Stdout = io.MultiWriter(stdout, pw)
		cmd.Stderr = stderr

		var streamDone sync.WaitGroup
		streamDone.Add(1)
		go func() {
			defer streamDone.Done()
			reader := bufio.NewReader(pr)
			for {
				line, truncated, readErr := readStreamLine(reader, defaultMaxStreamLineBytes)
				if line != "" {
					streamer(line)
				}
				if truncated {
					streamTruncated = true
					if streamErr == nil {
						streamErr = fmt.Errorf("stream line exceeded %d bytes", defaultMaxStreamLineBytes)
					}
				}
				if readErr != nil {
					if errors.Is(readErr, io.EOF) {
						return
					}
					streamErr = readErr
					return
				}
			}
		}()

		err = cmd.Run()
		pw.Close()
		streamDone.Wait()
	} else {
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err = cmd.Run()
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			// The process exited normally but a descendant kept the pipes
			// open past WaitDelay; preserve the real exit code (#786).
			exitCode = cmd.ProcessState.ExitCode()
		} else {
			exitCode = -1
		}
	}
	timedOut := errors.Is(timeoutCtx.Err(), context.DeadlineExceeded)
	output := mergeCommandStreams(strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))

	result := map[string]any{
		"command":     command,
		"exit_code":   exitCode,
		"timed_out":   timedOut,
		"output":      output,
		"working_dir": NormalizeRelPath(m.root, workDir),
	}
	if stdout.Truncated() || stderr.Truncated() {
		result["truncated"] = true
		result["max_bytes"] = m.maxOutputBytes
		result["truncation_strategy"] = "head_tail"
		result["hint"] = "[output truncated — use grep/head/tail to narrow results]"
	}
	if streamTruncated {
		result["stream_truncated"] = true
		result["max_line_bytes"] = defaultMaxStreamLineBytes
	}
	if streamErr != nil {
		result["stream_error"] = streamErr.Error()
	}
	if sbResult.Mechanism != "" {
		result["sandbox_mechanism"] = sbResult.Mechanism
	}
	if sbResult.Warning != "" {
		result["sandbox_warning"] = sbResult.Warning
	}
	if sbResult.NetworkPolicy != "" {
		result["sandbox_network"] = string(sbResult.NetworkPolicy)
	}
	return result, nil
}

func (m *JobManager) runBackground(ctx context.Context, command string, timeoutSeconds int, workingDir string) (map[string]any, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	if timeoutSeconds > 3600 {
		timeoutSeconds = 3600
	}
	scope := m.sandboxScopeForContext(ctx)
	network := m.networkPolicyForContext(ctx)
	if err := CheckSandboxCommand(scope, network, m.root, command); err != nil {
		return nil, err
	}
	workDir, err := resolveWorkingDir(m.root, workingDir)
	if err != nil {
		return nil, err
	}
	ctx = WithNetworkPolicy(ctx, network)

	m.cleanupExpired()

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("job manager is shut down")
	}
	if len(m.jobs) >= m.maxJobs {
		m.mu.Unlock()
		return nil, fmt.Errorf("background job limit reached")
	}
	id := "job_" + strconv.FormatUint(atomic.AddUint64(&m.nextID, 1), 10)
	// Detached from the caller's context, keeping only its values.
	//
	// Inheriting cancellation defeated the point of the tool: the context
	// belongs to the run that started the job, so when the run finished the
	// job was killed mid-flight. A "sleep 15; echo ..." reminder never reached
	// its echo, and the job reported exit -1 with no output — which the model
	// read, correctly, as "it did not fire", and then wrongly blamed the tool.
	//
	// A background job's lifetime is its own timeout, the manager's shutdown,
	// or an explicit job_kill. Not the turn that happened to launch it.
	ctx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), time.Duration(timeoutSeconds)*time.Second)
	// Capture the originating run's tenant so daemon-wide listings (the
	// /v1/tasks union) can scope jobs to their owning tenant. Absent for
	// callers without run metadata (e.g. the exported RunBackground wrapper).
	var tenantID, conversationID, originRunID string
	if md, ok := RunMetadataFromContext(ctx); ok {
		tenantID = md.TenantID
		conversationID = md.ConversationID
		originRunID = md.RunID
	}
	job := &backgroundJob{
		id:             id,
		command:        command,
		workingDir:     workDir,
		startedAt:      m.now(),
		tenantID:       tenantID,
		conversationID: conversationID,
		runID:          originRunID,
		stdout:         newHeadTailBuffer(m.maxOutputBytes),
		stderr:         newHeadTailBuffer(m.maxOutputBytes),
		cancel:         cancel,
		exitCode:       0,
	}
	m.jobs[id] = job
	m.wg.Add(1)
	m.mu.Unlock()

	cmd, sandboxCleanup, sbResult, err := buildSandboxedCommand(ctx, scope, m.root, command)
	if err != nil {
		cancel()
		m.mu.Lock()
		delete(m.jobs, id)
		m.mu.Unlock()
		m.wg.Done()
		return nil, err
	}
	configureGroupKill(cmd)
	cmd.Dir = workDir
	cmd.Stdout = job.stdout
	cmd.Stderr = job.stderr
	if err := cmd.Start(); err != nil {
		sandboxCleanup()
		cancel()
		m.mu.Lock()
		delete(m.jobs, id)
		m.mu.Unlock()
		m.wg.Done()
		return nil, fmt.Errorf("start background command: %w", err)
	}

	// Read the sink before the goroutine, not inside it. Reading it under
	// job.mu would take m.mu while holding job.mu, which is the opposite order
	// from cleanupExpired and deadlocks the manager.
	sink := m.jobEvents()

	go func() {
		defer m.wg.Done()
		err := cmd.Wait()
		sandboxCleanup()
		job.mu.Lock()
		defer job.mu.Unlock()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				job.exitCode = exitErr.ExitCode()
			} else if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				// The process exited normally but a descendant kept the
				// pipes open past WaitDelay; preserve the real exit code (#786).
				job.exitCode = cmd.ProcessState.ExitCode()
			} else {
				job.exitCode = -1
			}
			job.err = err
		}
		job.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		job.done = true

		// Report the result rather than leaving it in a buffer nobody knows to
		// read. Built while the job lock is held so the snapshot is coherent,
		// but delivered from a separate goroutine so a slow sink cannot block
		// the reaper or deadlock against a sink that calls back in.
		if sink != nil {
			completion := JobCompletion{
				ShellID:        job.id,
				Command:        job.command,
				WorkingDir:     NormalizeRelPath(m.root, job.workingDir),
				TenantID:       job.tenantID,
				ConversationID: job.conversationID,
				RunID:          job.runID,
				ExitCode:       job.exitCode,
				TimedOut:       job.timedOut,
				Output: mergeCommandStreams(
					strings.TrimSpace(job.stdout.String()),
					strings.TrimSpace(job.stderr.String())),
				Truncated: job.stdout.Truncated() || job.stderr.Truncated(),
			}
			go sink.JobCompleted(completion)
		}
	}()

	result := map[string]any{
		"shell_id":    id,
		"started":     true,
		"command":     command,
		"working_dir": NormalizeRelPath(m.root, workDir),
	}
	if sbResult.Mechanism != "" {
		result["sandbox_mechanism"] = sbResult.Mechanism
	}
	if sbResult.Warning != "" {
		result["sandbox_warning"] = sbResult.Warning
	}
	if sbResult.NetworkPolicy != "" {
		result["sandbox_network"] = string(sbResult.NetworkPolicy)
	}
	return result, nil
}

func readStreamLine(reader *bufio.Reader, maxBytes int) (string, bool, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxStreamLineBytes
	}

	var b strings.Builder
	truncated := false
	for {
		fragment, err := reader.ReadString('\n')
		if fragment != "" {
			remaining := maxBytes - b.Len()
			if remaining > 0 {
				if len(fragment) > remaining {
					b.WriteString(fragment[:remaining])
					truncated = true
				} else {
					b.WriteString(fragment)
				}
			} else {
				truncated = true
			}
		}

		switch {
		case err == nil:
			return b.String(), truncated, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			return b.String(), truncated, io.EOF
		default:
			return b.String(), truncated, err
		}
	}
}

// Shutdown cancels every tracked background job, waits for their Wait
// goroutines to return, then clears the job map.
func (m *JobManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	jobs := make([]*backgroundJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	m.mu.Unlock()

	for _, job := range jobs {
		job.cancel()
	}

	waitDone := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		m.mu.Lock()
		for id := range m.jobs {
			delete(m.jobs, id)
		}
		m.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *JobManager) output(shellID string, wait bool) (map[string]any, error) {
	job := m.get(shellID)
	if job == nil {
		return nil, fmt.Errorf("unknown shell_id %q", shellID)
	}
	if wait {
		deadline := time.Now().Add(5 * time.Second)
		for {
			job.mu.Lock()
			done := job.done
			job.mu.Unlock()
			if done || time.Now().After(deadline) {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	job.mu.Lock()
	defer job.mu.Unlock()

	output := mergeCommandStreams(strings.TrimSpace(job.stdout.String()), strings.TrimSpace(job.stderr.String()))
	result := map[string]any{
		"shell_id":   shellID,
		"running":    !job.done,
		"exit_code":  job.exitCode,
		"timed_out":  job.timedOut,
		"output":     output,
		"started_at": job.startedAt,
	}
	if job.stdout.Truncated() || job.stderr.Truncated() {
		result["truncated"] = true
		result["max_bytes"] = m.maxOutputBytes
		result["truncation_strategy"] = "head_tail"
		result["hint"] = "[output truncated — use grep/head/tail to narrow results]"
	}
	return result, nil
}

// KillForRun terminates every background job started by a run, and returns
// how many it killed.
//
// Background jobs are deliberately detached from the context of the run that
// starts them, so that a run finishing normally does not kill work it launched
// on purpose. Cancellation is the other case: a user who aborts a run does not
// expect its processes to keep going, so the runner calls this explicitly
// rather than relying on context propagation to mean both things at once.
func (m *JobManager) KillForRun(runID string) int {
	if runID == "" {
		return 0
	}
	m.mu.RLock()
	var ids []string
	for id, job := range m.jobs {
		if job.runID == runID {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	killed := 0
	for _, id := range ids {
		if _, err := m.kill(id); err == nil {
			killed++
		}
	}
	return killed
}

func (m *JobManager) kill(shellID string) (map[string]any, error) {
	job := m.get(shellID)
	if job == nil {
		return nil, fmt.Errorf("unknown shell_id %q", shellID)
	}
	job.cancel()
	job.mu.Lock()
	job.done = true
	job.mu.Unlock()
	return map[string]any{
		"shell_id": shellID,
		"killed":   true,
	}, nil
}

func (m *JobManager) get(id string) *backgroundJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[id]
}

// Job status values reported by JobInfo.Status. These are the bash_job
// statuses surfaced by the /v1/tasks union (epic #814).
const (
	JobStatusRunning  = "running"
	JobStatusExited   = "exited"
	JobStatusTimedOut = "timed_out"
)

// JobInfo is a point-in-time snapshot of a background job, safe to read
// without holding any manager locks. It carries everything the /v1/tasks
// union needs to render a bash_job row: identity, command, start time,
// owning tenant, and outcome.
type JobInfo struct {
	ID         string
	Command    string
	WorkingDir string
	StartedAt  time.Time
	// TenantID is the tenant of the run that started the job; empty when the
	// job was started without run metadata.
	TenantID string
	Running  bool
	ExitCode int
	TimedOut bool
}

// Status collapses the job's outcome flags into a single status string:
// running while in flight, timed_out when the timeout killed it, exited
// otherwise (success, failure, or kill).
func (j JobInfo) Status() string {
	if j.Running {
		return JobStatusRunning
	}
	if j.TimedOut {
		return JobStatusTimedOut
	}
	return JobStatusExited
}

// list returns a snapshot of every job currently tracked by the manager,
// sorted by start time then ID for deterministic output. Jobs already evicted
// by the finished-job TTL are absent. Safe for concurrent use with
// runBackground, kill, and cleanupExpired.
func (m *JobManager) list() []JobInfo {
	m.mu.RLock()
	jobs := make([]*backgroundJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	m.mu.RUnlock()

	out := make([]JobInfo, 0, len(jobs))
	for _, job := range jobs {
		job.mu.Lock()
		out = append(out, JobInfo{
			ID:         job.id,
			Command:    job.command,
			WorkingDir: job.workingDir,
			StartedAt:  job.startedAt,
			TenantID:   job.tenantID,
			Running:    !job.done,
			ExitCode:   job.exitCode,
			TimedOut:   job.timedOut,
		})
		job.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.Before(out[j].StartedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (m *JobManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for id, job := range m.jobs {
		job.mu.Lock()
		done := job.done
		started := job.startedAt
		job.mu.Unlock()
		if done && now.Sub(started) > m.ttl {
			delete(m.jobs, id)
		}
	}
}

func (m *JobManager) sandboxScopeForContext(ctx context.Context) SandboxScope {
	if scope, ok := SandboxScopeFromContext(ctx); ok && scope != "" {
		return scope
	}
	return m.sandboxScope
}

// networkPolicyForContext resolves the effective network policy: an explicit
// per-call context value wins, falling back to the JobManager-level default,
// and finally to NetworkPolicyAllow (issue #1397's safety-biased default).
func (m *JobManager) networkPolicyForContext(ctx context.Context) NetworkPolicy {
	if policy, ok := NetworkPolicyFromContext(ctx); ok && policy != "" {
		return policy
	}
	if m.networkPolicy != "" {
		return m.networkPolicy
	}
	return NetworkPolicyAllow
}

func resolveWorkingDir(workspaceRoot, workingDir string) (string, error) {
	if strings.TrimSpace(workingDir) == "" {
		return filepath.Abs(workspaceRoot)
	}
	return ResolveWorkspacePath(workspaceRoot, workingDir)
}
