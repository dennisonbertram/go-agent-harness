package tui

import "strings"

import "testing"

// The two fixture keys are the shape of real provider keys but are not real
// credentials. Tests assert neither ever reaches user-visible text.
const (
	fixtureStoredKey = "sk-or-v1-stored0000000000000000000000000000000000000000000000000000"
	fixtureEnvKey    = "sk-or-v1-env000000000000000000000000000000000000000000000000000000"
)

// TestStartupWarnsOnDivergentProviderKeys is the regression test for issue #1297:
// two different keys for one provider must not pass silently.
func TestStartupWarnsOnDivergentProviderKeys(t *testing.T) {
	notices := divergentKeyNotices(
		map[string]string{"openrouter": fixtureStoredKey},
		map[string]string{"openrouter": fixtureEnvKey},
	)

	if len(notices) != 1 {
		t.Fatalf("divergentKeyNotices returned %d notices, want 1: %v", len(notices), notices)
	}
	notice := notices[0]
	for _, want := range []string{"openrouter", "OPENROUTER_API_KEY", "stored"} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice %q must mention %q", notice, want)
		}
	}
	// A warning that leaks the credential is worse than no warning.
	for _, secret := range []string{fixtureStoredKey, fixtureEnvKey} {
		if strings.Contains(notice, secret) {
			t.Errorf("notice leaked key material: %q", notice)
		}
	}
}

// TestNoWarningWhenKeysAgree is the false-positive control: the check must not
// be satisfiable by warning unconditionally.
func TestNoWarningWhenKeysAgree(t *testing.T) {
	for _, tc := range []struct {
		name           string
		stored, envMap map[string]string
	}{
		{
			name:   "same key in both sources",
			stored: map[string]string{"openrouter": fixtureStoredKey},
			envMap: map[string]string{"openrouter": fixtureStoredKey},
		},
		{
			name:   "stored only",
			stored: map[string]string{"openrouter": fixtureStoredKey},
			envMap: map[string]string{},
		},
		{
			name:   "environment only",
			stored: map[string]string{},
			envMap: map[string]string{"openrouter": fixtureEnvKey},
		},
		{
			name:   "different providers, no overlap",
			stored: map[string]string{"openai": fixtureStoredKey},
			envMap: map[string]string{"anthropic": fixtureEnvKey},
		},
		{
			name:   "empty stored value is not a conflict",
			stored: map[string]string{"openrouter": ""},
			envMap: map[string]string{"openrouter": fixtureEnvKey},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if notices := divergentKeyNotices(tc.stored, tc.envMap); len(notices) != 0 {
				t.Errorf("want no notices, got %v", notices)
			}
		})
	}
}

// TestProviderKeySourceClassification covers the provenance states the /keys
// overlay renders.
func TestProviderKeySourceClassification(t *testing.T) {
	for _, tc := range []struct {
		name           string
		stored, envMap map[string]string
		want           keySource
	}{
		{"neither", nil, nil, keySourceNone},
		{"stored only", map[string]string{"openrouter": fixtureStoredKey}, nil, keySourceStored},
		{"environment only", nil, map[string]string{"openrouter": fixtureEnvKey}, keySourceEnv},
		{
			"both agree",
			map[string]string{"openrouter": fixtureStoredKey},
			map[string]string{"openrouter": fixtureStoredKey},
			keySourceBoth,
		},
		{
			"both differ",
			map[string]string{"openrouter": fixtureStoredKey},
			map[string]string{"openrouter": fixtureEnvKey},
			keySourceConflict,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{pendingAPIKeys: tc.stored, envAPIKeys: tc.envMap}
			if got := m.providerKeySource("openrouter"); got != tc.want {
				t.Errorf("providerKeySource() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestKeysOverlayShowsKeySource covers gap 2: the overlay must attribute each
// configured key to its source, and must never print key material.
func TestKeysOverlayShowsKeySource(t *testing.T) {
	m := Model{
		width:           100,
		height:          40,
		apiKeyProviders: []apiKeyProvider{{Name: "openrouter", APIKeyEnv: "OPENROUTER_API_KEY", Configured: true}},
		pendingAPIKeys:  map[string]string{"openrouter": fixtureStoredKey},
		envAPIKeys:      map[string]string{"openrouter": fixtureEnvKey},
	}

	out := stripANSIForTest(m.viewAPIKeysOverlay())

	if !strings.Contains(out, "conflict") {
		t.Errorf("overlay must flag the conflicting sources, got:\n%s", out)
	}
	for _, secret := range []string{fixtureStoredKey, fixtureEnvKey} {
		if strings.Contains(out, secret) {
			t.Error("overlay leaked key material")
		}
	}
}

// TestKeysOverlaySingleSourceAttribution checks the non-conflict labels.
func TestKeysOverlaySingleSourceAttribution(t *testing.T) {
	for _, tc := range []struct {
		name           string
		stored, envMap map[string]string
		want           string
	}{
		{"stored", map[string]string{"openrouter": fixtureStoredKey}, nil, "stored"},
		{"environment", nil, map[string]string{"openrouter": fixtureEnvKey}, "env"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				width:           100,
				height:          40,
				apiKeyProviders: []apiKeyProvider{{Name: "openrouter", APIKeyEnv: "OPENROUTER_API_KEY", Configured: true}},
				pendingAPIKeys:  tc.stored,
				envAPIKeys:      tc.envMap,
			}
			out := stripANSIForTest(m.viewAPIKeysOverlay())
			if !strings.Contains(out, tc.want) {
				t.Errorf("overlay must attribute the key to %q, got:\n%s", tc.want, out)
			}
		})
	}
}

// stripANSIForTest removes CSI escape sequences so assertions run against the
// visible text.
func stripANSIForTest(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !(s[i] >= '@' && s[i] <= '~') {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
