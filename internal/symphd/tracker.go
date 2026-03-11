package symphd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ClaimState represents the lifecycle state of a tracked issue.
type ClaimState string

const (
	ClaimStateUnclaimed ClaimState = "unclaimed"
	ClaimStateClaimed   ClaimState = "claimed"
	ClaimStateRunning   ClaimState = "running"
	ClaimStateDone      ClaimState = "done"
	ClaimStateFailed    ClaimState = "failed"
)

// TrackedIssue holds the GitHub issue data plus symphd claim state.
type TrackedIssue struct {
	Number      int
	Title       string
	Body        string
	Labels      []string
	ClaimState  ClaimState
	ClaimedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Attempts    int
}

// Tracker is the interface for managing GitHub issue claim state.
type Tracker interface {
	// Poll fetches open issues from GitHub and updates internal state.
	Poll(ctx context.Context) error
	// Candidates returns issues in ClaimStateClaimed, sorted by number ASC.
	Candidates() []*TrackedIssue
	// Claim marks an issue as claimed (Unclaimed → Claimed).
	Claim(number int) error
	// Start marks an issue as running (Claimed → Running).
	Start(number int) error
	// Complete marks an issue as done (Running → Done).
	Complete(number int) error
	// Fail marks an issue as failed (Running → Failed).
	Fail(number int, reason string) error
	// Issues returns all tracked issues.
	Issues() []*TrackedIssue
}

// githubIssueResponse is the JSON shape returned by the GitHub Issues API.
type githubIssueResponse struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// GitHubTracker polls GitHub Issues and manages the claim state machine.
type GitHubTracker struct {
	mu      sync.RWMutex
	owner   string
	repo    string
	label   string
	token   string
	issues  map[int]*TrackedIssue
	client  *http.Client
	baseURL string // overridable for testing
}

// NewGitHubTracker creates a new GitHubTracker.
func NewGitHubTracker(owner, repo, label, token string) *GitHubTracker {
	return &GitHubTracker{
		owner:   owner,
		repo:    repo,
		label:   label,
		token:   token,
		issues:  make(map[int]*TrackedIssue),
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.github.com",
	}
}

// Poll fetches open issues with the configured label from GitHub and adds
// any new ones as Unclaimed. Issues already tracked retain their state.
func (t *GitHubTracker) Poll(ctx context.Context) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues?state=open&labels=%s&per_page=100",
		t.baseURL, t.owner, t.repo, t.label)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("tracker: build request: %w", err)
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("tracker: fetch issues: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tracker: GitHub API returned status %d", resp.StatusCode)
	}

	var raw []githubIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("tracker: decode response: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, item := range raw {
		if _, exists := t.issues[item.Number]; exists {
			// Already tracked — preserve existing claim state.
			continue
		}
		labels := make([]string, len(item.Labels))
		for i, l := range item.Labels {
			labels[i] = l.Name
		}
		t.issues[item.Number] = &TrackedIssue{
			Number:     item.Number,
			Title:      item.Title,
			Body:       item.Body,
			Labels:     labels,
			ClaimState: ClaimStateUnclaimed,
		}
	}
	return nil
}

// Candidates returns all issues currently in ClaimStateClaimed, sorted by
// issue number ascending.
func (t *GitHubTracker) Candidates() []*TrackedIssue {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var out []*TrackedIssue
	for _, issue := range t.issues {
		if issue.ClaimState == ClaimStateClaimed {
			copy := *issue
			out = append(out, &copy)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Number < out[j].Number
	})
	return out
}

// Claim transitions an issue from Unclaimed → Claimed.
func (t *GitHubTracker) Claim(number int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	issue, ok := t.issues[number]
	if !ok {
		return fmt.Errorf("tracker: issue #%d not tracked", number)
	}
	if issue.ClaimState != ClaimStateUnclaimed {
		return fmt.Errorf("tracker: issue #%d is %s, cannot claim", number, issue.ClaimState)
	}
	issue.ClaimState = ClaimStateClaimed
	issue.ClaimedAt = time.Now()
	issue.Attempts++
	return nil
}

// Start transitions an issue from Claimed → Running.
func (t *GitHubTracker) Start(number int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	issue, ok := t.issues[number]
	if !ok {
		return fmt.Errorf("tracker: issue #%d not tracked", number)
	}
	if issue.ClaimState != ClaimStateClaimed {
		return fmt.Errorf("tracker: issue #%d is %s, cannot start (must be claimed first)", number, issue.ClaimState)
	}
	issue.ClaimState = ClaimStateRunning
	issue.StartedAt = time.Now()
	return nil
}

// Complete transitions an issue from Running → Done.
func (t *GitHubTracker) Complete(number int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	issue, ok := t.issues[number]
	if !ok {
		return fmt.Errorf("tracker: issue #%d not tracked", number)
	}
	if issue.ClaimState != ClaimStateRunning {
		return fmt.Errorf("tracker: issue #%d is %s, cannot complete (must be running)", number, issue.ClaimState)
	}
	issue.ClaimState = ClaimStateDone
	issue.CompletedAt = time.Now()
	return nil
}

// Fail transitions an issue from Running → Failed.
func (t *GitHubTracker) Fail(number int, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	issue, ok := t.issues[number]
	if !ok {
		return fmt.Errorf("tracker: issue #%d not tracked", number)
	}
	if issue.ClaimState != ClaimStateRunning {
		return fmt.Errorf("tracker: issue #%d is %s, cannot fail (must be running): %s", number, issue.ClaimState, reason)
	}
	issue.ClaimState = ClaimStateFailed
	issue.CompletedAt = time.Now()
	return nil
}

// Issues returns a snapshot of all tracked issues.
func (t *GitHubTracker) Issues() []*TrackedIssue {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make([]*TrackedIssue, 0, len(t.issues))
	for _, issue := range t.issues {
		copy := *issue
		out = append(out, &copy)
	}
	return out
}
