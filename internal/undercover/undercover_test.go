// Package undercover_test provides TDD tests for the undercover mode package.
// Tests are written BEFORE implementation to drive design.
package undercover_test

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/undercover"
)

// ---------------------------------------------------------------------------
// DefaultUndercoverConfig
// ---------------------------------------------------------------------------

// TestDefaultConfig_DisabledByDefault verifies that the default config has
// Enabled=false so that undercover mode is opt-in and never accidentally active.
func TestDefaultConfig_DisabledByDefault(t *testing.T) {
	cfg := undercover.DefaultUndercoverConfig()
	if cfg.Enabled {
		t.Errorf("DefaultUndercoverConfig().Enabled = true; want false (must be opt-in)")
	}
}

// TestDefaultConfig_SensibleDefaults verifies the other default field values
// match what the TOML spec documents.
func TestDefaultConfig_SensibleDefaults(t *testing.T) {
	cfg := undercover.DefaultUndercoverConfig()
	if !cfg.AutoDetectPublicRepos {
		t.Errorf("AutoDetectPublicRepos = false; want true")
	}
	if !cfg.StripCoAuthoredBy {
		t.Errorf("StripCoAuthoredBy = false; want true")
	}
	if !cfg.StripModelIdentifiers {
		t.Errorf("StripModelIdentifiers = false; want true")
	}
	if !cfg.HumanStyleCommits {
		t.Errorf("HumanStyleCommits = false; want true")
	}
	if len(cfg.CustomPatterns) != 0 {
		t.Errorf("CustomPatterns = %v; want empty slice", cfg.CustomPatterns)
	}
}

// ---------------------------------------------------------------------------
// IsPublicRepo
// ---------------------------------------------------------------------------

// TestIsPublicRepo_GitHubHTTPS verifies that https://github.com/... URLs are
// identified as public repos.
func TestIsPublicRepo_GitHubHTTPS(t *testing.T) {
	url := "https://github.com/user/repo"
	if !undercover.IsPublicRepo(url) {
		t.Errorf("IsPublicRepo(%q) = false; want true for public GitHub HTTPS URL", url)
	}
}

// TestIsPublicRepo_GitHubSSH verifies that git@github.com:... SSH URLs are
// identified as public repos.
func TestIsPublicRepo_GitHubSSH(t *testing.T) {
	url := "git@github.com:user/repo.git"
	if !undercover.IsPublicRepo(url) {
		t.Errorf("IsPublicRepo(%q) = false; want true for public GitHub SSH URL", url)
	}
}

// TestIsPublicRepo_GitLabHTTPS verifies that https://gitlab.com/... URLs are
// identified as public repos.
func TestIsPublicRepo_GitLabHTTPS(t *testing.T) {
	url := "https://gitlab.com/group/project"
	if !undercover.IsPublicRepo(url) {
		t.Errorf("IsPublicRepo(%q) = false; want true for public GitLab HTTPS URL", url)
	}
}

// TestIsPublicRepo_GitLabSSH verifies that git@gitlab.com:... SSH URLs are
// identified as public repos.
func TestIsPublicRepo_GitLabSSH(t *testing.T) {
	url := "git@gitlab.com:group/project.git"
	if !undercover.IsPublicRepo(url) {
		t.Errorf("IsPublicRepo(%q) = false; want true for public GitLab SSH URL", url)
	}
}

// TestIsPublicRepo_PrivateGitServer verifies that internal company git servers
// are NOT identified as public repos.
func TestIsPublicRepo_PrivateGitServer(t *testing.T) {
	url := "git@internal.company.com:repo.git"
	if undercover.IsPublicRepo(url) {
		t.Errorf("IsPublicRepo(%q) = true; want false for private git server", url)
	}
}

// TestIsPublicRepo_EmptyString verifies that an empty URL returns false.
func TestIsPublicRepo_EmptyString(t *testing.T) {
	if undercover.IsPublicRepo("") {
		t.Errorf("IsPublicRepo(\"\") = true; want false for empty URL")
	}
}

// TestIsPublicRepo_LocalPath verifies that local filesystem paths are not public repos.
func TestIsPublicRepo_LocalPath(t *testing.T) {
	url := "/home/user/projects/myrepo"
	if undercover.IsPublicRepo(url) {
		t.Errorf("IsPublicRepo(%q) = true; want false for local path", url)
	}
}

// ---------------------------------------------------------------------------
// ShouldActivate
// ---------------------------------------------------------------------------

// TestShouldActivate_ExplicitlyEnabled verifies that when Enabled=true the
// function returns true regardless of repo visibility.
func TestShouldActivate_ExplicitlyEnabled(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		Enabled:               true,
		AutoDetectPublicRepos: false,
	}
	// Private repo — should still activate because Enabled=true.
	if !undercover.ShouldActivate(cfg, "git@internal.company.com:repo.git") {
		t.Errorf("ShouldActivate with Enabled=true returned false; want true regardless of repo")
	}
}

// TestShouldActivate_AutoDetectPublic verifies that when Enabled=false but
// AutoDetectPublicRepos=true, a public repo activates undercover mode.
func TestShouldActivate_AutoDetectPublic(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		Enabled:               false,
		AutoDetectPublicRepos: true,
	}
	if !undercover.ShouldActivate(cfg, "https://github.com/user/repo") {
		t.Errorf("ShouldActivate with AutoDetectPublicRepos=true + public repo returned false; want true")
	}
}

// TestShouldActivate_AutoDetectPrivate verifies that when Enabled=false and
// AutoDetectPublicRepos=true, a private repo does NOT activate undercover mode.
func TestShouldActivate_AutoDetectPrivate(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		Enabled:               false,
		AutoDetectPublicRepos: true,
	}
	if undercover.ShouldActivate(cfg, "git@internal.company.com:repo.git") {
		t.Errorf("ShouldActivate with AutoDetectPublicRepos=true + private repo returned true; want false")
	}
}

// TestShouldActivate_Disabled verifies that when both Enabled=false and
// AutoDetectPublicRepos=false, the function always returns false.
func TestShouldActivate_Disabled(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		Enabled:               false,
		AutoDetectPublicRepos: false,
	}
	// Even a public repo should not activate.
	if undercover.ShouldActivate(cfg, "https://github.com/user/repo") {
		t.Errorf("ShouldActivate with Enabled=false + AutoDetect=false returned true; want false")
	}
}

// ---------------------------------------------------------------------------
// StripCoAuthoredByLines
// ---------------------------------------------------------------------------

// TestStripCoAuthoredByLines_SingleLine verifies a single Co-Authored-By trailer is removed.
func TestStripCoAuthoredByLines_SingleLine(t *testing.T) {
	input := "Fix the bug\n\nCo-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
	result := undercover.StripCoAuthoredByLines(input)
	if strings.Contains(result, "Co-Authored-By") {
		t.Errorf("StripCoAuthoredByLines did not remove Co-Authored-By line\ngot: %q", result)
	}
	if !strings.Contains(result, "Fix the bug") {
		t.Errorf("StripCoAuthoredByLines removed the commit subject line\ngot: %q", result)
	}
}

// TestStripCoAuthoredByLines_MultipleLines verifies all Co-Authored-By trailers are removed.
func TestStripCoAuthoredByLines_MultipleLines(t *testing.T) {
	input := "Add feature\n\nCo-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>\nCo-Authored-By: GPT-4 <noreply@openai.com>"
	result := undercover.StripCoAuthoredByLines(input)
	if strings.Contains(result, "Co-Authored-By") {
		t.Errorf("StripCoAuthoredByLines did not remove all Co-Authored-By lines\ngot: %q", result)
	}
}

// TestStripCoAuthoredByLines_CaseInsensitive verifies various cases are all stripped.
func TestStripCoAuthoredByLines_CaseInsensitive(t *testing.T) {
	input := "Refactor\n\nco-authored-by: agent1\nCO-AUTHORED-BY: agent2\nCo-Authored-By: agent3"
	result := undercover.StripCoAuthoredByLines(input)
	if strings.Contains(strings.ToLower(result), "co-authored-by") {
		t.Errorf("StripCoAuthoredByLines failed case-insensitive removal\ngot: %q", result)
	}
}

// TestStripCoAuthoredByLines_PreservesOtherTrailers verifies Signed-off-by and
// other standard git trailers are preserved.
func TestStripCoAuthoredByLines_PreservesOtherTrailers(t *testing.T) {
	input := "Update readme\n\nSigned-off-by: Developer <dev@example.com>\nCo-Authored-By: Claude <noreply@anthropic.com>"
	result := undercover.StripCoAuthoredByLines(input)
	if !strings.Contains(result, "Signed-off-by:") {
		t.Errorf("StripCoAuthoredByLines removed Signed-off-by trailer\ngot: %q", result)
	}
	if strings.Contains(strings.ToLower(result), "co-authored-by") {
		t.Errorf("StripCoAuthoredByLines kept Co-Authored-By trailer\ngot: %q", result)
	}
}

// TestStripCoAuthoredByLines_NoTrailers verifies that messages without any
// Co-Authored-By lines are returned unchanged.
func TestStripCoAuthoredByLines_NoTrailers(t *testing.T) {
	input := "Simple commit message"
	result := undercover.StripCoAuthoredByLines(input)
	if result != input {
		t.Errorf("StripCoAuthoredByLines changed a message with no trailers\ngot: %q\nwant: %q", result, input)
	}
}

// ---------------------------------------------------------------------------
// StripModelIDs
// ---------------------------------------------------------------------------

// TestStripModelIDs_Claude verifies claude model identifiers are stripped.
func TestStripModelIDs_Claude(t *testing.T) {
	cases := []struct {
		input    string
		contains string // substring that should NOT appear after stripping
	}{
		{"Generated by claude-opus-4.6 model", "claude-opus-4.6"},
		{"Using claude-sonnet-4-5 for inference", "claude-sonnet-4-5"},
		{"Model: claude-haiku-3", "claude-haiku-3"},
	}
	for _, tc := range cases {
		result := undercover.StripModelIDs(tc.input)
		if strings.Contains(result, tc.contains) {
			t.Errorf("StripModelIDs(%q) still contains %q\ngot: %q", tc.input, tc.contains, result)
		}
	}
}

// TestStripModelIDs_GPT verifies GPT model identifiers are stripped.
func TestStripModelIDs_GPT(t *testing.T) {
	cases := []struct {
		input    string
		contains string
	}{
		{"Using gpt-4o for this task", "gpt-4o"},
		{"Model gpt-4.1-mini was used", "gpt-4.1-mini"},
		{"gpt-5 is not available", "gpt-5"},
	}
	for _, tc := range cases {
		result := undercover.StripModelIDs(tc.input)
		if strings.Contains(result, tc.contains) {
			t.Errorf("StripModelIDs(%q) still contains %q\ngot: %q", tc.input, tc.contains, result)
		}
	}
}

// TestStripModelIDs_PreservesNonModelText verifies that normal prose containing
// words like "Claude" as a name is NOT stripped (only model ID patterns).
func TestStripModelIDs_PreservesNonModelText(t *testing.T) {
	// "Claude" as a name (not followed by a model version pattern) should be preserved.
	input := "Claude is the name of the AI assistant"
	result := undercover.StripModelIDs(input)
	if !strings.Contains(result, "Claude") {
		t.Errorf("StripModelIDs stripped 'Claude' from normal prose; got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// StripAttribution
// ---------------------------------------------------------------------------

// TestStripAttribution_CombinesAll verifies that StripAttribution applies all
// stripping functions when the config enables all flags.
func TestStripAttribution_CombinesAll(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		StripCoAuthoredBy:     true,
		StripModelIdentifiers: true,
		CustomPatterns:        []string{},
	}
	input := "Add feature\n\nModel: claude-opus-4.6\nCo-Authored-By: Claude <noreply@anthropic.com>"
	result := undercover.StripAttribution(input, cfg)
	if strings.Contains(strings.ToLower(result), "co-authored-by") {
		t.Errorf("StripAttribution did not strip Co-Authored-By\ngot: %q", result)
	}
	if strings.Contains(result, "claude-opus-4.6") {
		t.Errorf("StripAttribution did not strip model ID\ngot: %q", result)
	}
	if !strings.Contains(result, "Add feature") {
		t.Errorf("StripAttribution removed the commit subject\ngot: %q", result)
	}
}

// TestStripAttribution_DisabledConfig verifies that when all strip flags are
// false, the input is returned unchanged.
func TestStripAttribution_DisabledConfig(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		StripCoAuthoredBy:     false,
		StripModelIdentifiers: false,
	}
	input := "Add feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
	result := undercover.StripAttribution(input, cfg)
	if result != input {
		t.Errorf("StripAttribution modified text when all flags disabled\ngot: %q\nwant: %q", result, input)
	}
}

// TestStripAttribution_CustomPatterns verifies that custom regex patterns from
// the config are applied during stripping.
func TestStripAttribution_CustomPatterns(t *testing.T) {
	cfg := undercover.UndercoverConfig{
		StripCoAuthoredBy:     false,
		StripModelIdentifiers: false,
		CustomPatterns:        []string{`TICKET-\d+`},
	}
	input := "Fix TICKET-1234: resolve the null pointer issue"
	result := undercover.StripAttribution(input, cfg)
	if strings.Contains(result, "TICKET-1234") {
		t.Errorf("StripAttribution did not apply custom pattern\ngot: %q", result)
	}
	if !strings.Contains(result, "resolve the null pointer issue") {
		t.Errorf("StripAttribution removed non-matching text\ngot: %q", result)
	}
}

// ---------------------------------------------------------------------------
// HumanizeCommitMessage
// ---------------------------------------------------------------------------

// TestHumanizeCommitMessage_NoChange verifies that a naturally human-written
// commit message is returned unchanged (no conventional prefix).
func TestHumanizeCommitMessage_NoChange(t *testing.T) {
	input := "Update the documentation for clarity"
	result := undercover.HumanizeCommitMessage(input)
	if result != input {
		t.Errorf("HumanizeCommitMessage changed a human-style message\ngot: %q\nwant: %q", result, input)
	}
}

// TestHumanizeCommitMessage_StripsFeatPrefix verifies "feat(scope): message"
// is humanized by removing the conventional commits prefix.
func TestHumanizeCommitMessage_StripsFeatPrefix(t *testing.T) {
	input := "feat(auth): add OAuth2 support"
	result := undercover.HumanizeCommitMessage(input)
	// The conventional prefix should be stripped; the meaning should be preserved.
	if strings.HasPrefix(result, "feat(") {
		t.Errorf("HumanizeCommitMessage did not strip feat() prefix\ngot: %q", result)
	}
	if !strings.Contains(result, "OAuth2") {
		t.Errorf("HumanizeCommitMessage removed meaningful content\ngot: %q", result)
	}
}

// TestHumanizeCommitMessage_StripsFeatNoScope verifies "feat: message" is humanized.
func TestHumanizeCommitMessage_StripsFeatNoScope(t *testing.T) {
	input := "feat: add OAuth2 support"
	result := undercover.HumanizeCommitMessage(input)
	if strings.HasPrefix(result, "feat:") {
		t.Errorf("HumanizeCommitMessage did not strip 'feat:' prefix\ngot: %q", result)
	}
}

// TestHumanizeCommitMessage_StripFixPrefix verifies "fix: message" is humanized.
func TestHumanizeCommitMessage_StripFixPrefix(t *testing.T) {
	input := "fix: resolve null pointer exception in parser"
	result := undercover.HumanizeCommitMessage(input)
	if strings.HasPrefix(result, "fix:") {
		t.Errorf("HumanizeCommitMessage did not strip 'fix:' prefix\ngot: %q", result)
	}
	if !strings.Contains(result, "null pointer exception") {
		t.Errorf("HumanizeCommitMessage removed meaningful content\ngot: %q", result)
	}
}

// TestHumanizeCommitMessage_StripsTestPrefix verifies "test(#issue): message" is humanized.
func TestHumanizeCommitMessage_StripsTestPrefix(t *testing.T) {
	input := "test(#505): add regression tests for undercover mode"
	result := undercover.HumanizeCommitMessage(input)
	if strings.HasPrefix(result, "test(") {
		t.Errorf("HumanizeCommitMessage did not strip 'test()' prefix\ngot: %q", result)
	}
}

// ---------------------------------------------------------------------------
// TOML Configuration Parsing
// ---------------------------------------------------------------------------

// TestUndercoverConfig_FromTOML verifies that the [undercover] section of a
// TOML document is correctly decoded into UndercoverConfig.
func TestUndercoverConfig_FromTOML(t *testing.T) {
	tomlInput := `
[undercover]
enabled = true
auto_detect_public_repos = false
strip_co_authored_by = true
strip_model_identifiers = true
human_style_commits = false
custom_attribution_patterns = ["INTERNAL-\\d+", "TICKET-\\d+"]
`
	type rawConfig struct {
		Undercover undercover.UndercoverConfig `toml:"undercover"`
	}
	var cfg rawConfig
	if _, err := toml.Decode(tomlInput, &cfg); err != nil {
		t.Fatalf("toml.Decode failed: %v", err)
	}

	uc := cfg.Undercover
	if !uc.Enabled {
		t.Errorf("Enabled = false; want true")
	}
	if uc.AutoDetectPublicRepos {
		t.Errorf("AutoDetectPublicRepos = true; want false")
	}
	if !uc.StripCoAuthoredBy {
		t.Errorf("StripCoAuthoredBy = false; want true")
	}
	if !uc.StripModelIdentifiers {
		t.Errorf("StripModelIdentifiers = false; want true")
	}
	if uc.HumanStyleCommits {
		t.Errorf("HumanStyleCommits = true; want false")
	}
	if len(uc.CustomPatterns) != 2 {
		t.Errorf("CustomPatterns length = %d; want 2", len(uc.CustomPatterns))
	}
}

// TestUndercoverConfig_DefaultTOMLFields verifies that an empty [undercover]
// section leaves fields at their zero values (TOML decode — not DefaultUndercoverConfig).
func TestUndercoverConfig_DefaultTOMLFields(t *testing.T) {
	type rawConfig struct {
		Undercover undercover.UndercoverConfig `toml:"undercover"`
	}
	var cfg rawConfig
	if _, err := toml.Decode("[undercover]", &cfg); err != nil {
		t.Fatalf("toml.Decode failed: %v", err)
	}
	// Zero value: Enabled should be false.
	if cfg.Undercover.Enabled {
		t.Errorf("Enabled from empty TOML section = true; want false (zero value)")
	}
}
