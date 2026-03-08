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
	if getenv == nil {
		getenv = os.Getenv
	}

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

	home, _ := os.UserHomeDir()
	defaultDBPath := filepath.Join(home, ".go-harness", "cronsd.db")

	addr := envOrDefault("CRONSD_ADDR", ":9090")
	dbPath := envOrDefault("CRONSD_DB_PATH", defaultDBPath)
	maxConcurrent := envIntOrDefault("CRONSD_MAX_CONCURRENT", 5)

	store, err := cron.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	if err := store.Migrate(context.Background()); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	executor := &cron.ShellExecutor{}
	clock := cron.RealClock{}
	scheduler := cron.NewScheduler(store, executor, clock, cron.SchedulerConfig{
		MaxConcurrent: maxConcurrent,
	})

	if err := scheduler.Start(context.Background()); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}

	handler := cron.NewServer(store, scheduler, clock)
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
