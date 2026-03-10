package packs

import (
	"fmt"
	"os"
	"os/exec"
)

// ValidatePrereqs checks that all prerequisites declared in the manifest are met.
// It returns a (possibly empty) slice of errors — one per unmet prerequisite.
// Returning a slice rather than a single error allows callers to report all
// missing prerequisites at once instead of failing on the first one.
func ValidatePrereqs(m *SkillManifest) []error {
	var errs []error

	for _, cli := range m.RequiresCLI {
		if _, err := exec.LookPath(cli); err != nil {
			errs = append(errs, fmt.Errorf(
				"required CLI tool %q not found on PATH; install it to use skill pack %q",
				cli, m.Name,
			))
		}
	}

	for _, env := range m.RequiresEnv {
		if os.Getenv(env) == "" {
			errs = append(errs, fmt.Errorf(
				"required environment variable %q is not set; set it to use skill pack %q",
				env, m.Name,
			))
		}
	}

	return errs
}
