package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var dangerousBashPatterns = []string{
	`(?i)\brm\s+-rf\s+/`,
	`(?i)\bsudo\b`,
	`(?i)\bshutdown\b`,
	`(?i)\breboot\b`,
	`(?i):\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`,
}

type backgroundJob struct {
	id         string
	command    string
	workingDir string
	startedAt  time.Time

	stdout bytes.Buffer
	stderr bytes.Buffer

	mu       sync.Mutex
	exitCode int
	done     bool
	timedOut bool
	err      error
	cancel   context.CancelFunc
}

type JobManager struct {
	root    string
	nextID  uint64
	mu      sync.RWMutex
	jobs    map[string]*backgroundJob
	maxJobs int
	ttl     time.Duration
	now     func() time.Time
}

func NewJobManager(workspaceRoot string, now func() time.Time) *JobManager {
	if now == nil {
		now = time.Now
	}
	return &JobManager{
		root:    workspaceRoot,
		jobs:    make(map[string]*backgroundJob),
		maxJobs: 64,
		ttl:     30 * time.Minute,
		now:     now,
	}
}

func (m *JobManager) runForeground(ctx context.Context, command string, timeoutSeconds int, workingDir string) (map[string]any, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	if timeoutSeconds > 300 {
		timeoutSeconds = 300
	}
	workDir, err := resolveWorkingDir(m.root, workingDir)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "/bin/bash", "-lc", command)
	cmd.Dir = workDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	timedOut := errors.Is(timeoutCtx.Err(), context.DeadlineExceeded)
	output := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimSpace(stderr.String())
	}

	return map[string]any{
		"command":     command,
		"exit_code":   exitCode,
		"timed_out":   timedOut,
		"output":      output,
		"working_dir": normalizeRelPath(m.root, workDir),
	}, nil
}

func (m *JobManager) runBackground(command string, timeoutSeconds int, workingDir string) (map[string]any, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	if timeoutSeconds > 3600 {
		timeoutSeconds = 3600
	}
	workDir, err := resolveWorkingDir(m.root, workingDir)
	if err != nil {
		return nil, err
	}

	m.cleanupExpired()

	m.mu.Lock()
	if len(m.jobs) >= m.maxJobs {
		m.mu.Unlock()
		return nil, fmt.Errorf("background job limit reached")
	}
	id := "job_" + strconv.FormatUint(atomic.AddUint64(&m.nextID, 1), 10)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	job := &backgroundJob{id: id, command: command, workingDir: workDir, startedAt: m.now(), cancel: cancel, exitCode: 0}
	m.jobs[id] = job
	m.mu.Unlock()

	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", command)
	cmd.Dir = workDir
	cmd.Stdout = &job.stdout
	cmd.Stderr = &job.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		m.mu.Lock()
		delete(m.jobs, id)
		m.mu.Unlock()
		return nil, fmt.Errorf("start background command: %w", err)
	}

	go func() {
		err := cmd.Wait()
		job.mu.Lock()
		defer job.mu.Unlock()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				job.exitCode = exitErr.ExitCode()
			} else {
				job.exitCode = -1
			}
			job.err = err
		}
		job.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		job.done = true
	}()

	return map[string]any{
		"shell_id":    id,
		"started":     true,
		"command":     command,
		"working_dir": normalizeRelPath(m.root, workDir),
	}, nil
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

	output := strings.TrimSpace(job.stdout.String())
	stderr := strings.TrimSpace(job.stderr.String())
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}
	return map[string]any{
		"shell_id":   shellID,
		"running":    !job.done,
		"exit_code":  job.exitCode,
		"timed_out":  job.timedOut,
		"output":     output,
		"started_at": job.startedAt,
	}, nil
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

func resolveWorkingDir(workspaceRoot, workingDir string) (string, error) {
	if strings.TrimSpace(workingDir) == "" {
		return filepath.Abs(workspaceRoot)
	}
	return resolveWorkspacePath(workspaceRoot, workingDir)
}
