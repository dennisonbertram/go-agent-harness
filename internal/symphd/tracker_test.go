package symphd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"
)

// newMockGitHubServer returns an httptest.Server that serves the given issues
// as a JSON array, mimicking the GitHub Issues API response format.
func newMockGitHubServer(t *testing.T, issues []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(issues); err != nil {
			t.Errorf("mock server encode: %v", err)
		}
	}))
}

// newTrackerWithServer creates a GitHubTracker pointed at the mock server URL.
func newTrackerWithServer(srv *httptest.Server) *GitHubTracker {
	tr := NewGitHubTracker("owner", "repo", "symphd", "test-token")
	tr.baseURL = srv.URL
	return tr
}

func issueFixture(number int, title string) map[string]any {
	return map[string]any{
		"number": number,
		"title":  title,
		"body":   "body text",
		"labels": []map[string]any{
			{"name": "symphd"},
		},
	}
}

// TestGitHubTracker_Poll_FetchesIssues verifies that Poll adds new issues with
// ClaimStateUnclaimed.
func TestGitHubTracker_Poll_FetchesIssues(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{
		issueFixture(1, "Issue One"),
		issueFixture(2, "Issue Two"),
	})
	defer srv.Close()

	tr := newTrackerWithServer(srv)
	if err := tr.Poll(context.Background()); err != nil {
		t.Fatalf("Poll error: %v", err)
	}

	issues := tr.Issues()
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	// Build map for assertion independence of order.
	byNum := make(map[int]*TrackedIssue)
	for _, iss := range issues {
		byNum[iss.Number] = iss
	}

	for _, num := range []int{1, 2} {
		iss, ok := byNum[num]
		if !ok {
			t.Errorf("issue #%d not found", num)
			continue
		}
		if iss.ClaimState != ClaimStateUnclaimed {
			t.Errorf("issue #%d state = %s, want unclaimed", num, iss.ClaimState)
		}
		if len(iss.Labels) == 0 {
			t.Errorf("issue #%d has no labels", num)
		}
	}
}

// TestGitHubTracker_Poll_EmptyResponse verifies that an empty list doesn't panic.
func TestGitHubTracker_Poll_EmptyResponse(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{})
	defer srv.Close()

	tr := newTrackerWithServer(srv)
	if err := tr.Poll(context.Background()); err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if got := len(tr.Issues()); got != 0 {
		t.Errorf("expected 0 issues, got %d", got)
	}
}

// TestGitHubTracker_Poll_Idempotent verifies that polling the same issues
// twice doesn't reset their claim state.
func TestGitHubTracker_Poll_Idempotent(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{
		issueFixture(10, "Repeated Issue"),
	})
	defer srv.Close()

	tr := newTrackerWithServer(srv)

	// First poll — issue arrives as unclaimed.
	if err := tr.Poll(context.Background()); err != nil {
		t.Fatalf("Poll 1 error: %v", err)
	}
	if err := tr.Claim(10); err != nil {
		t.Fatalf("Claim error: %v", err)
	}

	// Second poll — issue is already tracked; state must not reset.
	if err := tr.Poll(context.Background()); err != nil {
		t.Fatalf("Poll 2 error: %v", err)
	}

	issues := tr.Issues()
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ClaimState != ClaimStateClaimed {
		t.Errorf("state = %s after second Poll, want claimed", issues[0].ClaimState)
	}
}

// TestGitHubTracker_Claim_ValidTransition checks Unclaimed → Claimed.
func TestGitHubTracker_Claim_ValidTransition(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{issueFixture(5, "T")})
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := tr.Claim(5); err != nil {
		t.Fatalf("Claim error: %v", err)
	}
	after := time.Now()

	issues := tr.Issues()
	if len(issues) != 1 {
		t.Fatal("missing issue")
	}
	iss := issues[0]
	if iss.ClaimState != ClaimStateClaimed {
		t.Errorf("state = %s, want claimed", iss.ClaimState)
	}
	if iss.ClaimedAt.Before(before) || iss.ClaimedAt.After(after) {
		t.Errorf("ClaimedAt %v not in range [%v, %v]", iss.ClaimedAt, before, after)
	}
	if iss.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", iss.Attempts)
	}
}

// TestGitHubTracker_Claim_AlreadyClaimed verifies that claiming a claimed
// issue returns an error.
func TestGitHubTracker_Claim_AlreadyClaimed(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{issueFixture(5, "T")})
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := tr.Claim(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Claim(5); err == nil {
		t.Error("expected error claiming already-claimed issue")
	}
}

// TestGitHubTracker_Start_ValidTransition checks Claimed → Running.
func TestGitHubTracker_Start_ValidTransition(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{issueFixture(5, "T")})
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := tr.Claim(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Start(5); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	issues := tr.Issues()
	if issues[0].ClaimState != ClaimStateRunning {
		t.Errorf("state = %s, want running", issues[0].ClaimState)
	}
	if issues[0].StartedAt.IsZero() {
		t.Error("StartedAt not set")
	}
}

// TestGitHubTracker_Complete_ValidTransition checks Running → Done.
func TestGitHubTracker_Complete_ValidTransition(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{issueFixture(5, "T")})
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := tr.Claim(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Start(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Complete(5); err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	issues := tr.Issues()
	if issues[0].ClaimState != ClaimStateDone {
		t.Errorf("state = %s, want done", issues[0].ClaimState)
	}
	if issues[0].CompletedAt.IsZero() {
		t.Error("CompletedAt not set")
	}
}

// TestGitHubTracker_Fail_ValidTransition checks Running → Failed.
func TestGitHubTracker_Fail_ValidTransition(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{issueFixture(5, "T")})
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := tr.Claim(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Start(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Fail(5, "timed out"); err != nil {
		t.Fatalf("Fail error: %v", err)
	}

	issues := tr.Issues()
	if issues[0].ClaimState != ClaimStateFailed {
		t.Errorf("state = %s, want failed", issues[0].ClaimState)
	}
	if issues[0].CompletedAt.IsZero() {
		t.Error("CompletedAt not set after Fail")
	}
}

// TestGitHubTracker_InvalidTransition verifies that skipping states returns errors.
func TestGitHubTracker_InvalidTransition(t *testing.T) {
	srv := newMockGitHubServer(t, []map[string]any{issueFixture(5, "T")})
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Unclaimed → Running (invalid: must claim first)
	if err := tr.Start(5); err == nil {
		t.Error("expected error starting unclaimed issue")
	}

	// Unclaimed → Done (invalid)
	if err := tr.Complete(5); err == nil {
		t.Error("expected error completing unclaimed issue")
	}

	// Unclaimed → Failed (invalid)
	if err := tr.Fail(5, "reason"); err == nil {
		t.Error("expected error failing unclaimed issue")
	}

	// Now claim it; try Complete without Start
	if err := tr.Claim(5); err != nil {
		t.Fatal(err)
	}
	if err := tr.Complete(5); err == nil {
		t.Error("expected error completing claimed (not running) issue")
	}
}

// TestGitHubTracker_Candidates_Sorted checks that Candidates returns only
// claimed issues, sorted by number ascending.
func TestGitHubTracker_Candidates_Sorted(t *testing.T) {
	fixtures := []map[string]any{
		issueFixture(30, "C"),
		issueFixture(10, "A"),
		issueFixture(20, "B"),
	}
	srv := newMockGitHubServer(t, fixtures)
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Claim 10 and 30 but not 20.
	if err := tr.Claim(10); err != nil {
		t.Fatal(err)
	}
	if err := tr.Claim(30); err != nil {
		t.Fatal(err)
	}

	cands := tr.Candidates()
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}

	// Verify sorted order.
	if !sort.SliceIsSorted(cands, func(i, j int) bool { return cands[i].Number < cands[j].Number }) {
		t.Error("candidates not sorted by number ASC")
	}
	if cands[0].Number != 10 || cands[1].Number != 30 {
		t.Errorf("wrong order: %d, %d", cands[0].Number, cands[1].Number)
	}
}

// TestGitHubTracker_Issues_ReturnsAll verifies that Issues returns all tracked
// issues regardless of state.
func TestGitHubTracker_Issues_ReturnsAll(t *testing.T) {
	fixtures := []map[string]any{
		issueFixture(1, "A"),
		issueFixture(2, "B"),
		issueFixture(3, "C"),
	}
	srv := newMockGitHubServer(t, fixtures)
	defer srv.Close()
	tr := newTrackerWithServer(srv)

	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Advance #1 through all the way to Done.
	if err := tr.Claim(1); err != nil {
		t.Fatal(err)
	}
	if err := tr.Start(1); err != nil {
		t.Fatal(err)
	}
	if err := tr.Complete(1); err != nil {
		t.Fatal(err)
	}
	// Advance #2 to Failed.
	if err := tr.Claim(2); err != nil {
		t.Fatal(err)
	}
	if err := tr.Start(2); err != nil {
		t.Fatal(err)
	}
	if err := tr.Fail(2, "error"); err != nil {
		t.Fatal(err)
	}

	all := tr.Issues()
	if len(all) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(all))
	}
}

// TestGitHubTracker_Claim_UnknownIssue verifies the error for unknown issue.
func TestGitHubTracker_Claim_UnknownIssue(t *testing.T) {
	tr := NewGitHubTracker("owner", "repo", "symphd", "token")
	if err := tr.Claim(999); err == nil {
		t.Error("expected error for unknown issue")
	}
}

// TestGitHubTracker_Poll_HTTPError verifies that a non-200 status is surfaced.
func TestGitHubTracker_Poll_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	tr := NewGitHubTracker("owner", "repo", "symphd", "bad-token")
	tr.baseURL = srv.URL

	if err := tr.Poll(context.Background()); err == nil {
		t.Error("expected error for 403 response")
	}
}

// TestGitHubTracker_Concurrent exercises Poll, Claim, and Start concurrently
// under the race detector.
func TestGitHubTracker_Concurrent(t *testing.T) {
	fixtures := make([]map[string]any, 20)
	for i := range fixtures {
		fixtures[i] = issueFixture(i+1, "issue")
	}
	srv := newMockGitHubServer(t, fixtures)
	defer srv.Close()

	tr := newTrackerWithServer(srv)
	if err := tr.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	// Concurrent polls.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tr.Poll(context.Background())
		}()
	}

	// Concurrent claims (only first succeeds per issue).
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		num := i
		go func() {
			defer wg.Done()
			_ = tr.Claim(num)
		}()
	}

	// Concurrent Issues reads.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tr.Issues()
		}()
	}

	// Concurrent Candidates reads.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tr.Candidates()
		}()
	}

	wg.Wait()
}
