package main

import (
	"testing"
)

func TestParentWatchdogIsOptIn(t *testing.T) {
	// Off by default. harnessd is also started deliberately detached — nohup,
	// launchd, scripts — where being reparented to init is intended and
	// exiting would be a regression.
	cases := map[string]bool{
		"":      false,
		"false": false,
		"0":     false,
		"no":    false, // not a valid bool: must not enable on a typo
		"true":  true,
		"TRUE":  true,
		" 1 ":   true,
	}
	for value, want := range cases {
		got := parentWatchdogEnabled(func(string) string { return value })
		if got != want {
			t.Errorf("HARNESS_EXIT_WITH_PARENT=%q enabled=%v, want %v", value, got, want)
		}
	}
}

func TestParentWatchdogDisabledReturnsNoOpStop(t *testing.T) {
	// The returned stop must be safe to call even when nothing was started,
	// since it is deferred unconditionally at startup.
	stop := watchParent(func(string) string { return "" }, nil)
	stop()
	stop()
}

func TestParentWatchdogStopIsIdempotent(t *testing.T) {
	stop := watchParent(func(string) string { return "true" }, nil)
	stop()
	stop() // must not panic on a double close
}
