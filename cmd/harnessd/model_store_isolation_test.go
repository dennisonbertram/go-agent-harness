package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the model store at a throwaway file for the whole package.
//
// Without this every test in cmd/harnessd boots a daemon that loads
// ~/.harness/models.json — the developer's own configuration. Exposed models
// there replace the catalog listing on /v1/models, so tests asserting on a
// fixture catalog fail against whatever providers happen to be configured on
// the machine, and pass on CI where the file is absent. Worse, a test could in
// principle write to the real store.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "harnessd-model-store")
	if err != nil {
		panic("create temp model store dir: " + err.Error())
	}
	os.Setenv("HARNESS_MODEL_STORE_PATH", filepath.Join(dir, "models.json"))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
