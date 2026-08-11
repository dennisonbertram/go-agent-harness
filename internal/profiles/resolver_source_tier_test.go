package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSourceTierProfile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
}

func TestResolveProfileWithSourceTier_ReportsDefiningTierAndStaysFresh(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()

	p, tier, err := ResolveProfileWithDirsAndSourceTier("full", projectDir, userDir)
	if err != nil {
		t.Fatalf("resolve built-in: %v", err)
	}
	if tier != "built-in" || p.Meta.Name != "full" {
		t.Fatalf("built-in = (%q, %q), want (full, built-in)", p.Meta.Name, tier)
	}

	writeSourceTierProfile(t, userDir, "full", "[meta]\nname = \"full\"\n\n[runner]\nmodel = \"user-model\"\n")
	p, tier, err = ResolveProfileWithDirsAndSourceTier("full", projectDir, userDir)
	if err != nil {
		t.Fatalf("resolve user override: %v", err)
	}
	if tier != "user" || p.Runner.Model != "user-model" {
		t.Fatalf("user override = (%q, %q), want (user-model, user)", p.Runner.Model, tier)
	}

	writeSourceTierProfile(t, projectDir, "full", "[meta]\nname = \"full\"\n\n[runner]\nmodel = \"project-model\"\n")
	p, tier, err = ResolveProfileWithDirsAndSourceTier("full", projectDir, userDir)
	if err != nil {
		t.Fatalf("resolve project override: %v", err)
	}
	if tier != "project" || p.Runner.Model != "project-model" {
		t.Fatalf("project override = (%q, %q), want (project-model, project)", p.Runner.Model, tier)
	}

	if err := os.Remove(filepath.Join(projectDir, "full.toml")); err != nil {
		t.Fatalf("remove project override: %v", err)
	}
	p, tier, err = ResolveProfileWithDirsAndSourceTier("full", projectDir, userDir)
	if err != nil {
		t.Fatalf("resolve after remove: %v", err)
	}
	if tier != "user" || p.Runner.Model != "user-model" {
		t.Fatalf("after remove = (%q, %q), want (user-model, user)", p.Runner.Model, tier)
	}
}

func TestResolveProfileWithSourceTier_InheritedChildReportsChildTier(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()
	writeSourceTierProfile(t, userDir, "source-tier-base", "[meta]\nname = \"source-tier-base\"\n\n[runner]\nmodel = \"user-base\"\n")
	writeSourceTierProfile(t, projectDir, "source-tier-child", "extends = \"source-tier-base\"\n\n[meta]\nname = \"source-tier-child\"\n\n[runner]\nmax_steps = 7\n")

	p, tier, err := ResolveProfileWithDirsAndSourceTier("source-tier-child", projectDir, userDir)
	if err != nil {
		t.Fatalf("resolve inherited child: %v", err)
	}
	if tier != "project" || p.Runner.Model != "user-base" || p.Runner.MaxSteps != 7 {
		t.Fatalf("child = (tier=%q model=%q steps=%d), want (project, user-base, 7)", tier, p.Runner.Model, p.Runner.MaxSteps)
	}
}
