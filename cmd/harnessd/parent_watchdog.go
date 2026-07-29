package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// parentWatchdogInterval is how often the daemon checks that its parent is
// still alive. Slow enough to be free, fast enough that a killed app does not
// leave a server holding a port for long.
const parentWatchdogInterval = 2 * time.Second

// watchParent makes the daemon exit when the process that started it goes away.
//
// harnessd is normally supervised by the macOS app, which stops it on a clean
// quit. Nothing covered the unclean paths: kill, crash, or a debugger stop left
// the daemon running, reparented to init, holding its port and its SQLite
// stores. Over one working session that accumulated 31 orphaned daemons.
//
// This is opt-in via HARNESS_EXIT_WITH_PARENT rather than always-on, because
// harnessd is also started deliberately detached — by nohup, by launchd, by
// scripts — where being reparented to init is the intended state and exiting
// would be a regression. Only a supervisor that intends to own the daemon's
// lifetime sets it.
//
// Signalling the caught process rather than os.Exit lets the normal shutdown
// path run, so stores are flushed and closed instead of being left mid-write.
func watchParent(getenv func(string) string, logf func(string, ...any)) (stop func()) {
	if !parentWatchdogEnabled(getenv) {
		return func() {}
	}

	// Captured now: once the parent dies this process is reparented to init
	// (pid 1) and the original value is unrecoverable.
	original := os.Getppid()
	if original <= 1 {
		// Already orphaned, or started directly by init. Either way there is
		// no parent to outlive, so watching would either fire immediately or
		// never — neither is useful.
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(parentWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if os.Getppid() != original {
					if logf != nil {
						logf("parent process %d exited; shutting down", original)
					}
					// Go through the daemon's own signal handling so shutdown
					// is graceful.
					_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
					return
				}
			}
		}
	}()

	var once bool
	return func() {
		if !once {
			once = true
			close(done)
		}
	}
}

func parentWatchdogEnabled(getenv func(string) string) bool {
	if getenv == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(getenv("HARNESS_EXIT_WITH_PARENT")))
	if v == "" {
		return false
	}
	enabled, err := strconv.ParseBool(v)
	return err == nil && enabled
}
