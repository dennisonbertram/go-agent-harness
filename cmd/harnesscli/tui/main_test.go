package tui_test

import (
	"os"
	"testing"
)

// TestMain redirects HOME to a scratch directory for the whole package before
// any test runs.
//
// The TUI reads and writes ~/.config/harnesscli/config.json — starred models,
// theme, gateway, and since issue #1424 the last used model. Without this, a
// test that opens the model switcher writes into the developer's real config,
// and later tests inherit whatever it selected. That is not hypothetical: it
// was caught here by three tests failing against a model no human had chosen.
//
// It has to happen once, here, rather than via t.Setenv in a shared helper:
// t.Setenv cannot be used by tests that call t.Parallel, and several in this
// package do. Setting it before any test starts is also race-free, where
// mutating the environment from running tests would not be. Tests that need
// their own HOME still override it locally for their own duration.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "harnesscli-tui-test-home")
	if err != nil {
		panic("create scratch HOME: " + err.Error())
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic("redirect HOME: " + err.Error())
	}

	code := m.Run()

	_ = os.RemoveAll(home)
	os.Exit(code)
}
