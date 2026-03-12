package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-agent-harness/internal/symphd"
)

var (
	runMain  = func() error { return run(os.Args[1:]) }
	exitFunc = os.Exit
)

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintf(os.Stderr, "symphd: %v\n", err)
		exitFunc(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("symphd", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to YAML config file")
	addrOverride := fs.String("addr", "", "override listen address (e.g. :8888)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var cfg *symphd.Config
	var err error
	if *configPath != "" {
		cfg, err = symphd.Load(*configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
	} else {
		cfg = symphd.DefaultConfig()
	}

	if *addrOverride != "" {
		cfg.Addr = *addrOverride
	}

	orch := symphd.NewOrchestrator(cfg)
	handler := symphd.NewHandler(orch)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := orch.Start(ctx); err != nil {
		return fmt.Errorf("orchestrator start: %w", err)
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("symphd listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	log.Println("shutting down gracefully...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := orch.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("orchestrator shutdown: %w", err)
	}

	log.Println("symphd stopped")
	return nil
}
