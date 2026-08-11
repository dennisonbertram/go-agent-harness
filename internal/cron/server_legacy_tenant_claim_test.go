package cron

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCronServersRaceToClaimLegacyShellJobForExactlyOneTenant(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cron.db")
	firstStore, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	if err := firstStore.Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	secondStore, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })
	if err := secondStore.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	legacy := testJob("legacy-unowned-shell")
	legacy.TenantID = ""
	legacy, err = firstStore.CreateJob(ctx, legacy)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	clock := newMockClock(time.Now().UTC())
	firstScheduler := NewScheduler(firstStore, &mockExecutor{}, clock, SchedulerConfig{MaxConcurrent: 1})
	secondScheduler := NewScheduler(secondStore, &mockExecutor{}, clock, SchedulerConfig{MaxConcurrent: 1})
	firstServer := httptest.NewServer(NewServer(firstStore, firstScheduler, clock, IngressAuthConfig{APIKey: "key-a", TenantID: "tenant-a"}))
	t.Cleanup(firstServer.Close)
	secondServer := httptest.NewServer(NewServer(secondStore, secondScheduler, clock, IngressAuthConfig{APIKey: "key-b", TenantID: "tenant-b"}))
	t.Cleanup(secondServer.Close)

	type result struct {
		tenant string
		status int
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, request := range []struct {
		tenant string
		url    string
		key    string
	}{
		{tenant: "tenant-a", url: firstServer.URL, key: "key-a"},
		{tenant: "tenant-b", url: secondServer.URL, key: "key-b"},
	} {
		wg.Add(1)
		go func(request struct{ tenant, url, key string }) {
			defer wg.Done()
			<-start
			httpRequest, requestErr := http.NewRequest(http.MethodGet, request.url+"/v1/jobs/"+legacy.ID, nil)
			if requestErr != nil {
				results <- result{tenant: request.tenant, status: 0}
				return
			}
			httpRequest.Header.Set("Authorization", "Bearer "+request.key)
			response, requestErr := http.DefaultClient.Do(httpRequest)
			if requestErr != nil {
				results <- result{tenant: request.tenant, status: 0}
				return
			}
			_ = response.Body.Close()
			results <- result{tenant: request.tenant, status: response.StatusCode}
		}(request)
	}
	close(start)
	wg.Wait()
	close(results)

	statuses := map[string]int{}
	for result := range results {
		statuses[result.tenant] = result.status
	}
	winner := ""
	for tenant, status := range statuses {
		switch status {
		case http.StatusOK:
			if winner != "" {
				t.Fatalf("both tenants claimed legacy job: statuses=%v", statuses)
			}
			winner = tenant
		case http.StatusNotFound:
		default:
			t.Fatalf("tenant %s status = %d, want 200 or 404; statuses=%v", tenant, status, statuses)
		}
	}
	if winner == "" {
		t.Fatalf("no tenant claimed legacy job: statuses=%v", statuses)
	}
	persisted, err := firstStore.GetJob(ctx, legacy.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if persisted.TenantID != winner {
		t.Fatalf("persisted tenant = %q, winner = %q", persisted.TenantID, winner)
	}
	loser := "tenant-a"
	loserURL, loserKey := firstServer.URL, "key-a"
	if winner == loser {
		loser, loserURL, loserKey = "tenant-b", secondServer.URL, "key-b"
	}
	request, err := http.NewRequest(http.MethodGet, loserURL+"/v1/jobs/"+legacy.ID, nil)
	if err != nil {
		t.Fatalf("loser request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+loserKey)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("loser GetJob: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("loser %s repeat status = %d, want 404", loser, response.StatusCode)
	}
}
