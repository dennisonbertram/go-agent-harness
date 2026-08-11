package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/cron"
)

func TestLoadConfigParsesRemoteHarnessTimeouts(t *testing.T) {
	env := map[string]string{
		"CRONSD_ADDR":                    ":9091",
		"CRONSD_DB_PATH":                 "/tmp/cronsd.db",
		"CRONSD_MAX_CONCURRENT":          "7",
		"CRONSD_INGRESS_API_KEY":         "  ingress-secret  ",
		"CRONSD_INGRESS_TENANT_ID":       "  tenant-a  ",
		"CRONSD_HARNESS_URL":             "http://harnessd:8080",
		"CRONSD_HARNESS_API_KEY":         "secret",
		"CRONSD_HARNESS_CONNECT_TIMEOUT": "2s",
		"CRONSD_HARNESS_REQUEST_TIMEOUT": "9s",
	}
	cfg, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Addr != ":9091" || cfg.MaxConcurrent != 7 || cfg.Harness.BaseURL != env["CRONSD_HARNESS_URL"] ||
		cfg.Harness.APIKey != "secret" || cfg.Harness.ConnectTimeout != 2*time.Second || cfg.Harness.RequestTimeout != 9*time.Second ||
		cfg.Ingress.APIKey != "ingress-secret" || cfg.Ingress.TenantID != "tenant-a" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestPrepareTenantJobsClaimsOnlyLegacyShellRows(t *testing.T) {
	store, err := cron.NewSQLiteStore(filepath.Join(t.TempDir(), "cron.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	now := time.Now().UTC()
	legacy := cron.Job{ID: "legacy-shell", Name: "legacy", Schedule: "* * * * *", ExecType: cron.ExecTypeShell, Status: cron.StatusActive, TimeoutSec: 30, NextRunAt: now, CreatedAt: now, UpdatedAt: now}
	legacy, err = store.CreateJob(context.Background(), legacy)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	jobs, err := prepareTenantJobs(context.Background(), store, []cron.Job{legacy}, "tenant-a")
	if err != nil {
		t.Fatalf("prepareTenantJobs: %v", err)
	}
	if jobs[0].TenantID != "tenant-a" {
		t.Fatalf("claimed tenant = %q", jobs[0].TenantID)
	}
	persisted, err := store.GetJob(context.Background(), legacy.ID)
	if err != nil || persisted.TenantID != "tenant-a" {
		t.Fatalf("persisted job = %#v, %v", persisted, err)
	}

	harnessJob := legacy
	harnessJob.ID = "legacy-harness"
	harnessJob.ExecType = cron.ExecTypeHarness
	harnessJob.TenantID = ""
	if _, err := prepareTenantJobs(context.Background(), store, []cron.Job{harnessJob}, "tenant-a"); err == nil {
		t.Fatal("legacy unowned harness job was accepted")
	}

	otherTenant := legacy
	otherTenant.ID = "other-tenant"
	otherTenant.TenantID = "tenant-b"
	if _, err := prepareTenantJobs(context.Background(), store, []cron.Job{otherTenant}, "tenant-a"); err == nil {
		t.Fatal("other-tenant job was accepted")
	}
}

func TestPrepareTenantJobsConcurrentTenantRaceHasOneStartupWinner(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cron.db")
	first, err := cron.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	second, err := cron.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := second.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	now := time.Now().UTC()
	legacy := cron.Job{ID: "legacy-race", Name: "legacy-race", Schedule: "* * * * *", ExecType: cron.ExecTypeShell, Status: cron.StatusActive, TimeoutSec: 30, NextRunAt: now, CreatedAt: now, UpdatedAt: now}
	legacy, err = first.CreateJob(ctx, legacy)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	type result struct {
		tenant string
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, candidate := range []struct {
		tenant string
		store  cron.Store
	}{{tenant: "tenant-a", store: first}, {tenant: "tenant-b", store: second}} {
		wg.Add(1)
		go func(candidate struct {
			tenant string
			store  cron.Store
		}) {
			defer wg.Done()
			<-start
			_, prepareErr := prepareTenantJobs(ctx, candidate.store, []cron.Job{legacy}, candidate.tenant)
			results <- result{tenant: candidate.tenant, err: prepareErr}
		}(candidate)
	}
	close(start)
	wg.Wait()
	close(results)
	winner := ""
	for result := range results {
		if result.err == nil {
			if winner != "" {
				t.Fatal("both daemon startups claimed the legacy job")
			}
			winner = result.tenant
		}
	}
	if winner == "" {
		t.Fatal("neither daemon startup claimed the legacy job")
	}
	persisted, err := first.GetJob(ctx, legacy.ID)
	if err != nil || persisted.TenantID != winner {
		t.Fatalf("persisted tenant = %q, winner=%q err=%v", persisted.TenantID, winner, err)
	}
}

func TestLoadConfigRejectsInvalidRemoteTimeout(t *testing.T) {
	env := map[string]string{"CRONSD_HARNESS_CONNECT_TIMEOUT": "not-a-duration"}
	_, err := loadConfig(func(key string) string { return env[key] })
	if err == nil || !strings.Contains(err.Error(), "CRONSD_HARNESS_CONNECT_TIMEOUT") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateSecurityConfigurationRejectsSharedIngressAndOutboundSecret(t *testing.T) {
	err := validateSecurityConfiguration(cronsdConfig{
		Ingress: cron.IngressAuthConfig{APIKey: "shared-secret", TenantID: "tenant-a"},
		Harness: cron.RemoteRunStarterConfig{APIKey: "shared-secret"},
	})
	if err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateHarnessConfigurationFailsOnlyForHarnessJobs(t *testing.T) {
	starter := cron.NewRemoteRunStarter(cron.RemoteRunStarterConfig{})
	if err := validateHarnessConfiguration([]cron.Job{{ExecType: cron.ExecTypeShell}}, starter); err != nil {
		t.Fatalf("shell-only jobs should be ready: %v", err)
	}
	if err := validateHarnessConfiguration([]cron.Job{{ID: "job-1", Status: cron.StatusActive, ExecType: cron.ExecTypeHarness}}, starter); err == nil || !strings.Contains(err.Error(), "job-1") {
		t.Fatalf("harness readiness = %v", err)
	}
}
