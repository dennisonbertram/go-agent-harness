package cron

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegrationCronUpdateCompareAndSwapAllowsOneConcurrentWriter(t *testing.T) {
	_, scheduler, client := newIntegrationCronClient(t)
	defer scheduler.Stop()

	job, err := client.CreateJob(context.Background(), CreateJobRequest{
		TenantID:       "tenant-a",
		ConversationID: "conversation-a",
		AgentID:        "agent-a",
		Name:           "cas-job",
		Schedule:       "0 0 * * *",
		ExecType:       ExecTypeShell,
		ExecConfig:     `{"command":"echo one"}`,
		TimeoutSec:     30,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, tag := range []string{"first", "second"} {
		tag := tag
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := client.UpdateJob(context.Background(), job.ID, UpdateJobRequest{
				Tags:              &tag,
				ExpectedUpdatedAt: &job.UpdatedAt,
			})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var successes, conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case IsJobConflict(err):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent update results = successes %d, conflicts %d; want one of each", successes, conflicts)
	}

	final, err := client.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get final job: %v", err)
	}
	if final.Status != StatusActive {
		t.Fatalf("final status = %q, want active; stale PATCH must not restore a prior status", final.Status)
	}
	if final.Tags != "first" && final.Tags != "second" {
		t.Fatalf("final tags = %q, want one winning writer", final.Tags)
	}
	if !scheduler.HasEntry(job.ID) {
		t.Fatal("winning active update must leave the job scheduled")
	}

}

func newIntegrationCronClient(t *testing.T) (*SQLiteStore, *Scheduler, *Client) {
	t.Helper()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "cron.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		store.Close()
		t.Fatalf("migrate: %v", err)
	}
	scheduler := NewScheduler(store, &ShellExecutor{}, RealClock{}, SchedulerConfig{MaxConcurrent: 2})
	if err := scheduler.Start(context.Background()); err != nil {
		store.Close()
		t.Fatalf("start scheduler: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	handler := authenticatedServer(store, scheduler, RealClock{})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return store, scheduler, NewClient(server.URL)
}

func TestServerUpdateJobRejectsNonPositiveTimeout(t *testing.T) {
	store := &mockStore{}
	clock := newMockClock(time.Date(2026, 7, 31, 1, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(store, &mockExecutor{}, clock, SchedulerConfig{MaxConcurrent: 1})
	handler := authenticatedServer(store, scheduler, clock)
	job := testJob("invalid-timeout")
	store.GetJobFunc = func(context.Context, string) (Job, error) { return job, nil }

	for _, timeout := range []int{0, -1} {
		t.Run(fmt.Sprintf("timeout_%d", timeout), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/v1/jobs/"+job.ID, strings.NewReader(fmt.Sprintf(`{"timeout_seconds":%d}`, timeout)))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "timeout_seconds") {
				t.Fatalf("timeout %d response = %d %s, want actionable 400", timeout, rec.Code, rec.Body.String())
			}
		})
	}
}
