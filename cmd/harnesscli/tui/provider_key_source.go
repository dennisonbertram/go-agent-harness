package tui

import "fmt"

// providerEnvKeyVars maps a provider name to the environment variable the
// daemon and the TUI both read its key from.
//
// harnessd resolves provider keys from its own environment
// (catalog.ProviderRegistry.GetClient), and the TUI additionally replays keys
// stored in ~/.config/harnesscli/config.json over
// PUT /v1/providers/{name}/key. Two sources means the two can disagree.
var providerEnvKeyVars = map[string]string{
	"openrouter": "OPENROUTER_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"anthropic":  "ANTHROPIC_API_KEY",
}

// keySource describes where a provider's configured key came from.
type keySource int

const (
	// keySourceNone means no key is configured from either source.
	keySourceNone keySource = iota
	// keySourceEnv means only the shell environment supplied a key.
	keySourceEnv
	// keySourceStored means only ~/.config/harnesscli/config.json supplied a key.
	keySourceStored
	// keySourceBoth means both sources supplied the same key.
	keySourceBoth
	// keySourceConflict means both sources supplied keys and they differ. The
	// stored key wins, because Init replays it over the daemon's environment.
	keySourceConflict
)

// label returns the short attribution shown next to a key in the /keys overlay.
// It never contains key material.
func (s keySource) label() string {
	switch s {
	case keySourceEnv:
		return "env"
	case keySourceStored:
		return "stored"
	case keySourceBoth:
		return "env+stored"
	case keySourceConflict:
		return "conflict: stored wins"
	default:
		return ""
	}
}

// providerKeySource classifies where the named provider's key came from.
func (m Model) providerKeySource(providerKey string) keySource {
	stored := m.pendingAPIKeys[providerKey]
	env := m.envAPIKeys[providerKey]

	switch {
	case stored == "" && env == "":
		return keySourceNone
	case stored == "":
		return keySourceEnv
	case env == "":
		return keySourceStored
	case stored == env:
		return keySourceBoth
	default:
		return keySourceConflict
	}
}

// divergentKeyNotices returns one startup notice per provider whose stored key
// differs from the key in the shell environment.
//
// Without this the two collapse into a single "configured" mark and a request
// fails with a provider 401 that names neither the key nor its source. The
// notices name the provider, the environment variable, and the winning source,
// and never contain key material.
func divergentKeyNotices(stored, env map[string]string) []string {
	var notices []string
	// Iterate the fixed provider list so notice order does not depend on Go's
	// randomised map iteration.
	for _, provider := range sortedProviderNames() {
		s, e := stored[provider], env[provider]
		if s == "" || e == "" || s == e {
			continue
		}
		notices = append(notices, fmt.Sprintf(
			"%s: stored key differs from %s — the stored key wins; run /keys to check",
			provider, providerEnvKeyVars[provider],
		))
	}
	return notices
}

// sortedProviderNames returns the providers that have a known key environment
// variable, in a stable order.
func sortedProviderNames() []string {
	// Kept explicit rather than sorting the map so the display order is a
	// deliberate choice rather than alphabetical accident.
	return []string{"openrouter", "openai", "anthropic"}
}
