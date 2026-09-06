package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// containsDir reports whether dirs contains want after canonicalizing both
// sides the same way (Abs + best-effort EvalSymlinks), so tests do not have
// to hand-resolve platform-specific symlinks (e.g. /tmp -> /private/tmp on
// macOS) themselves.
func containsDir(t *testing.T, dirs []string, want string) bool {
	t.Helper()
	wantResolved := resolveForTest(t, want)
	for _, d := range dirs {
		if d == wantResolved || d == want {
			return true
		}
	}
	return false
}

func resolveForTest(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// TestToolchainWritableDirsIncludesTempDir verifies that os.TempDir() (which
// honors TMPDIR) is always included, since Go's own scratch directory for
// `go build`/`go test` lives there.
func TestToolchainWritableDirsIncludesTempDir(t *testing.T) {
	dirs := toolchainWritableDirs()
	if !containsDir(t, dirs, os.TempDir()) {
		t.Errorf("expected toolchainWritableDirs() to include os.TempDir() (%q), got %v", os.TempDir(), dirs)
	}
}

// TestToolchainWritableDirsCreatesModuleCacheWhenMissing verifies the Go
// module cache (GOMODCACHE, or GOPATH/pkg, or ~/go/pkg as the final
// fallback) is created if it does not already exist, per the contract: this
// is one of only two directories toolchainWritableDirs is allowed to create
// rather than merely detect.
func TestToolchainWritableDirsCreatesModuleCacheWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOPATH", "")
	t.Setenv("GOMODCACHE", "")

	wantModCache := filepath.Join(home, "go", "pkg")
	if _, err := os.Stat(wantModCache); err == nil {
		t.Fatalf("test setup invariant violated: %q already exists", wantModCache)
	}

	dirs := toolchainWritableDirs()

	if _, err := os.Stat(wantModCache); err != nil {
		t.Fatalf("expected toolchainWritableDirs() to create %q, got stat error: %v", wantModCache, err)
	}
	if !containsDir(t, dirs, wantModCache) {
		t.Errorf("expected toolchainWritableDirs() to include the created module cache %q, got %v", wantModCache, dirs)
	}
}

// TestToolchainWritableDirsRespectsGOMODCACHEOverride verifies an explicit
// GOMODCACHE override is used (and created if missing) instead of the
// GOPATH/pkg or ~/go/pkg fallback chain.
func TestToolchainWritableDirsRespectsGOMODCACHEOverride(t *testing.T) {
	home := t.TempDir()
	override := filepath.Join(t.TempDir(), "custom-modcache")
	t.Setenv("HOME", home)
	t.Setenv("GOPATH", "")
	t.Setenv("GOMODCACHE", override)

	dirs := toolchainWritableDirs()

	if _, err := os.Stat(override); err != nil {
		t.Fatalf("expected toolchainWritableDirs() to create GOMODCACHE override %q, got stat error: %v", override, err)
	}
	if !containsDir(t, dirs, override) {
		t.Errorf("expected toolchainWritableDirs() to include GOMODCACHE override %q, got %v", override, dirs)
	}
	defaultModCache := filepath.Join(home, "go", "pkg")
	if containsDir(t, dirs, defaultModCache) {
		t.Errorf("expected toolchainWritableDirs() NOT to also include the default module cache %q when GOMODCACHE is set, got %v", defaultModCache, dirs)
	}
}

// TestToolchainWritableDirsIncludesExistingGOCACHE verifies an explicit
// GOCACHE is included when it already exists on disk.
func TestToolchainWritableDirsIncludesExistingGOCACHE(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gocache := t.TempDir()
	t.Setenv("GOCACHE", gocache)

	dirs := toolchainWritableDirs()
	if !containsDir(t, dirs, gocache) {
		t.Errorf("expected toolchainWritableDirs() to include existing GOCACHE %q, got %v", gocache, dirs)
	}
}

// TestToolchainWritableDirsExcludesMissingGOCACHE verifies GOCACHE is only
// included (and never created) when it already exists — unlike the module
// cache, GOCACHE is not one of the two directories the contract allows this
// function to create.
func TestToolchainWritableDirsExcludesMissingGOCACHE(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	missing := filepath.Join(t.TempDir(), "does-not-exist-gocache")
	t.Setenv("GOCACHE", missing)

	dirs := toolchainWritableDirs()
	if containsDir(t, dirs, missing) {
		t.Errorf("expected toolchainWritableDirs() NOT to include a nonexistent GOCACHE %q, got %v", missing, dirs)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Errorf("expected toolchainWritableDirs() NOT to create a missing GOCACHE %q, but it now exists", missing)
	}
}

// TestToolchainWritableDirsCreatesDotCacheWhenMissing verifies ~/.cache is
// created if missing — the other directory (besides the module cache) the
// contract allows this function to create.
func TestToolchainWritableDirsCreatesDotCacheWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")

	wantDotCache := filepath.Join(home, ".cache")
	if _, err := os.Stat(wantDotCache); err == nil {
		t.Fatalf("test setup invariant violated: %q already exists", wantDotCache)
	}

	dirs := toolchainWritableDirs()

	if _, err := os.Stat(wantDotCache); err != nil {
		t.Fatalf("expected toolchainWritableDirs() to create ~/.cache (%q), got stat error: %v", wantDotCache, err)
	}
	if !containsDir(t, dirs, wantDotCache) {
		t.Errorf("expected toolchainWritableDirs() to include the created ~/.cache %q, got %v", wantDotCache, dirs)
	}
}

// TestToolchainWritableDirsIncludesNpmAndCargoWhenPresent verifies ~/.npm,
// ~/.cargo/registry, and ~/.cargo/git are included when they already exist,
// since those are the per-user cache roots npm and cargo write to.
func TestToolchainWritableDirsIncludesNpmAndCargoWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	npm := filepath.Join(home, ".npm")
	cargoRegistry := filepath.Join(home, ".cargo", "registry")
	cargoGit := filepath.Join(home, ".cargo", "git")
	for _, d := range []string{npm, cargoRegistry, cargoGit} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", d, err)
		}
	}

	dirs := toolchainWritableDirs()
	for _, want := range []string{npm, cargoRegistry, cargoGit} {
		if !containsDir(t, dirs, want) {
			t.Errorf("expected toolchainWritableDirs() to include existing %q, got %v", want, dirs)
		}
	}
}

// TestToolchainWritableDirsExcludesNpmAndCargoWhenAbsent verifies npm/cargo
// dirs that do not exist are skipped rather than created (unlike ~/.cache
// and the module cache).
func TestToolchainWritableDirsExcludesNpmAndCargoWhenAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dirs := toolchainWritableDirs()
	for _, absent := range []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".cargo", "registry"),
		filepath.Join(home, ".cargo", "git"),
	} {
		if containsDir(t, dirs, absent) {
			t.Errorf("expected toolchainWritableDirs() NOT to include nonexistent %q, got %v", absent, dirs)
		}
	}
}

// TestToolchainWritableDirsNoBlanketHome guards against a regression to
// blanket $HOME write access: the returned list must never contain the home
// directory itself, only the specific narrow subdirectories the contract
// names.
func TestToolchainWritableDirsNoBlanketHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dirs := toolchainWritableDirs()
	homeResolved := resolveForTest(t, home)
	for _, d := range dirs {
		if d == home || d == homeResolved {
			t.Fatalf("toolchainWritableDirs() must never include $HOME itself, got %v", dirs)
		}
	}
}

// TestToolchainWritableDirsCanonicalizesSymlinks verifies each returned
// directory is symlink-canonicalized (filepath.EvalSymlinks), the same way
// the workspace root already is, so the darwin seatbelt "(subpath ...)"
// match (which operates on the kernel's resolved path) and Linux's bwrap
// bind source/target actually cover what gets written to at runtime.
func TestToolchainWritableDirsCanonicalizesSymlinks(t *testing.T) {
	realDir := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "tmp-link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlink not supported on this filesystem: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMPDIR", link)

	dirs := toolchainWritableDirs()

	resolvedReal := resolveForTest(t, realDir)
	found := false
	for _, d := range dirs {
		if d == link {
			t.Errorf("expected toolchainWritableDirs() to return the symlink-resolved form of TMPDIR, got the raw symlink path %q", d)
		}
		if d == resolvedReal {
			found = true
		}
	}
	if !found {
		t.Errorf("expected toolchainWritableDirs() to include the symlink-resolved TMPDIR %q, got %v", resolvedReal, dirs)
	}
}
