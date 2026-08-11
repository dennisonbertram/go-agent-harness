package tui

import (
	harnessconfig "go-agent-harness/cmd/harnesscli/config"
)

// persistConfigField applies mutate to the stored config and writes it back,
// refusing to write when the existing config could not be read.
//
// config.Load returns an empty Config on a read or parse failure. Mutating that
// empty value and saving it destroys every other stored field — that is how a
// valid OpenRouter key was lost (issue #1300). Every save site goes through here
// so the check cannot be forgotten at one of them, which is exactly what happened
// before: three sites checked the error and two did not.
func persistConfigField(mutate func(*harnessconfig.Config)) error {
	cfg, err := harnessconfig.Load()
	if err != nil {
		return err
	}
	mutate(cfg)
	return harnessconfig.Save(cfg)
}
