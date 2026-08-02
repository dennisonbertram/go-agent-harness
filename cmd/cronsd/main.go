package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-agent-harness/internal/cron"
)

var (
	runMain  = run
	exitFunc = os.Exit
)

func main() {
	if err := runMain(); err != nil {
		log.Printf("fatal: %v", err)
		exitFunc(1)
	}
}

func run() error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	return runWithSignals(sig, os.Getenv)
}

func runWithSignals(sig <-chan os.Signal, getenv func(string) string) error {
	if sig == nil {
		return fmt.Errorf("signal channel is required")
	}

	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	if err := validateSecurityConfiguration(cfg); err != nil {
		return err
	}
	addr := cfg.Addr
	dbPath := cfg.DBPath
	maxConcurrent := cfg.MaxConcurrent

	store, err := cron.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	if err := store.Migrate(context.Background()); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	remoteStarter := cron.NewRemoteRunStarter(cfg.Harness)
	jobs, err := store.ListJobs(context.Background())
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	jobs, err = prepareTenantJobs(context.Background(), store, jobs, cfg.Ingress.TenantID)
	if err != nil {
		return err
	}
	if err := validateHarnessConfiguration(jobs, remoteStarter); err != nil {
		return err
	}
	executor := &cron.DispatchExecutor{
		Shell:   &cron.ShellExecutor{},
		Harness: &cron.HarnessExecutor{Starter: remoteStarter, Observer: remoteStarter},
	}
	clock := cron.RealClock{}
	scheduler := cron.NewScheduler(store, executor, clock, cron.SchedulerConfig{
		MaxConcurrent: maxConcurrent,
	})

	if err := scheduler.Start(context.Background()); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}

	handler := cron.NewServer(store, scheduler, clock, cfg.Ingress)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		log.Printf("cronsd listening on %s (db: %s)", addr, dbPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		scheduler.Stop()
		return err
	case <-sig:
	}

	log.Println("shutting down...")
	scheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	select {
	case err := <-serverErr:
		return err
	case <-serverDone:
	}
	return nil
}

type cronsdConfig struct {
	Addr          string
	DBPath        string
	MaxConcurrent int
	Ingress       cron.IngressAuthConfig
	Harness       cron.RemoteRunStarterConfig
}

func loadConfig(getenv func(string) string) (cronsdConfig, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	home, _ := os.UserHomeDir()
	defaultDBPath := filepath.Join(home, ".go-harness", "cronsd.db")
	envOrDefault := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
	envIntOrDefault := func(key string, fallback int) int {
		v := getenv(key)
		if v == "" {
			return fallback
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fallback
		}
		return n
	}
	parseDuration := func(key string, fallback time.Duration) (time.Duration, error) {
		value := getenv(key)
		if value == "" {
			return fallback, nil
		}
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("%s must be a positive Go duration", key)
		}
		return parsed, nil
	}
	connectTimeout, err := parseDuration("CRONSD_HARNESS_CONNECT_TIMEOUT", 5*time.Second)
	if err != nil {
		return cronsdConfig{}, err
	}
	requestTimeout, err := parseDuration("CRONSD_HARNESS_REQUEST_TIMEOUT", 15*time.Second)
	if err != nil {
		return cronsdConfig{}, err
	}
	return cronsdConfig{
		Addr:          envOrDefault("CRONSD_ADDR", ":9090"),
		DBPath:        envOrDefault("CRONSD_DB_PATH", defaultDBPath),
		MaxConcurrent: envIntOrDefault("CRONSD_MAX_CONCURRENT", 5),
		Ingress: cron.IngressAuthConfig{
			APIKey:   strings.TrimSpace(getenv("CRONSD_INGRESS_API_KEY")),
			TenantID: strings.TrimSpace(getenv("CRONSD_INGRESS_TENANT_ID")),
		},
		Harness: cron.RemoteRunStarterConfig{
			BaseURL:        getenv("CRONSD_HARNESS_URL"),
			APIKey:         getenv("CRONSD_HARNESS_API_KEY"),
			ConnectTimeout: connectTimeout,
			RequestTimeout: requestTimeout,
		},
	}, nil
}

func validateSecurityConfiguration(cfg cronsdConfig) error {
	if err := cfg.Ingress.Validate(); err != nil {
		return fmt.Errorf("configure cron ingress: %w", err)
	}
	outboundKey := strings.TrimSpace(cfg.Harness.APIKey)
	if outboundKey != "" && outboundKey == cfg.Ingress.APIKey {
		return fmt.Errorf("CRONSD_INGRESS_API_KEY must differ from CRONSD_HARNESS_API_KEY")
	}
	return nil
}

func prepareTenantJobs(ctx context.Context, store cron.Store, jobs []cron.Job, tenantID string) ([]cron.Job, error) {
	claimer, canClaim := store.(cron.JobTenantClaimer)
	for i := range jobs {
		job := &jobs[i]
		if job.TenantID == tenantID {
			continue
		}
		if job.TenantID == "" && job.ExecType == cron.ExecTypeShell {
			if !canClaim {
				return nil, fmt.Errorf("cron store cannot durably claim legacy shell job %q", job.ID)
			}
			claimedJob, claimed, err := claimer.ClaimJobTenant(ctx, job.ID, tenantID)
			if err != nil {
				return nil, fmt.Errorf("claim legacy shell job %q: %w", job.ID, err)
			}
			if !claimed || claimedJob.TenantID != tenantID {
				return nil, fmt.Errorf("cron job %q is not owned by configured ingress tenant", job.ID)
			}
			*job = claimedJob
			continue
		}
		return nil, fmt.Errorf("cron job %q is not owned by configured ingress tenant", job.ID)
	}
	return jobs, nil
}

func validateHarnessConfiguration(jobs []cron.Job, starter *cron.RemoteRunStarter) error {
	for _, job := range jobs {
		if job.Status != cron.StatusActive || job.ExecType != cron.ExecTypeHarness {
			continue
		}
		if err := starter.ValidateJob(job); err != nil {
			return fmt.Errorf("harness job %q (%s) is not ready: %w", job.ID, job.Name, err)
		}
	}
	return nil
}
