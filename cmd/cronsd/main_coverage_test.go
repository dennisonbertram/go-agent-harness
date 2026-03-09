package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunWithSignalsCustomMaxConcurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cronsd.db")

	env := map[string]string{
		"CRONSD_ADDR":           "127.0.0.1:0",
		"CRONSD_DB_PATH":        dbPath,
		"CRONSD_MAX_CONCURRENT": "10",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv)
	}()

	time.Sleep(200 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

func TestRunWithSignalsInvalidMaxConcurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cronsd.db")

	// "not-a-number" should fall back to default (5).
	env := map[string]string{
		"CRONSD_ADDR":           "127.0.0.1:0",
		"CRONSD_DB_PATH":        dbPath,
		"CRONSD_MAX_CONCURRENT": "not-a-number",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv)
	}()

	time.Sleep(200 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

func TestRunWithSignalsNilGetenv(t *testing.T) {
	// When getenv is nil, runWithSignals should fall back to os.Getenv.
	// We set up env vars via t.Setenv so os.Getenv returns test values.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cronsd-nilgetenv.db")

	t.Setenv("CRONSD_ADDR", "127.0.0.1:0")
	t.Setenv("CRONSD_DB_PATH", dbPath)

	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, nil)
	}()

	time.Sleep(200 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}
