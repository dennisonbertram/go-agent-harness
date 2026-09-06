package tools

import (
	"os"
	"path/filepath"
)

// toolchainWritableDirs returns the per-user temp and cache directories that
// language-toolchain invocations (go build/test, npm, cargo) need to write
// to even though nothing else about them lives inside the workspace (issue
// #1399). Without these, a `go build` run under SandboxScopeWorkspace fails
// with "failed to initialize build cache ... operation not permitted" (the
// build cache lives under the per-user cache dir) or "creating work dir:
// mkdir /var/folders/...: operation not permitted" (Go's scratch dir lives
// under the process temp dir) — neither of those roots is the workspace, so
// neither darwin's seatbelt "(subpath ...)" allow-list nor Linux's bwrap
// binds cover them today.
//
// Included roots (only when they already exist, unless noted):
//   - os.TempDir() — honors TMPDIR.
//   - os.UserCacheDir() — honors XDG_CACHE_HOME / ~/Library/Caches.
//   - ~/.cache, when different from the above — CREATED if missing, since a
//     fresh machine may not have it yet and it is one of the two
//     conventional cache homes this function is allowed to create.
//   - $GOCACHE, if the env var is set and the directory already exists.
//   - The Go module cache: $GOMODCACHE if set, else $GOPATH/pkg, else
//     ~/go/pkg — CREATED if missing (the other of the two directories this
//     function is allowed to create).
//   - ~/.npm, ~/.cargo/registry, ~/.cargo/git — only when they already
//     exist.
//
// $HOME itself is never included: only these specific, narrow
// subdirectories are opened up for writes.
//
// Every returned path is symlink-canonicalized (filepath.EvalSymlinks) the
// same way the workspace root already is in buildSandboxedCommand, so the
// darwin seatbelt profile's "(subpath ...)" match (which operates on the
// kernel's resolved path, not a symlinked one) and the Linux bwrap
// "--bind src dst" invocation actually cover what gets written to at
// runtime.
//
// Computed fresh on every call (not cached) so it reflects the calling
// process's current environment (TMPDIR, GOCACHE, GOMODCACHE, GOPATH,
// XDG_CACHE_HOME) rather than a snapshot taken at process start — tests
// exercise this via t.Setenv, and a long-lived harnessd should not need a
// restart for env changes to take effect here.
func toolchainWritableDirs() []string {
	var dirs []string
	seen := make(map[string]bool)

	addExisting := func(p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(p); err != nil {
			return
		}
		addCanonicalDir(&dirs, seen, p)
	}
	addCreated := func(p string) {
		if p == "" {
			return
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return
		}
		addCanonicalDir(&dirs, seen, p)
	}

	addExisting(os.TempDir())

	if cacheDir, err := os.UserCacheDir(); err == nil {
		addExisting(cacheDir)
	}

	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		addCreated(filepath.Join(home, ".cache"))
	}

	addExisting(os.Getenv("GOCACHE"))

	modCache := os.Getenv("GOMODCACHE")
	if modCache == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" && homeErr == nil {
			gopath = filepath.Join(home, "go")
		}
		if gopath != "" {
			modCache = filepath.Join(gopath, "pkg")
		}
	}
	addCreated(modCache)

	if homeErr == nil {
		addExisting(filepath.Join(home, ".npm"))
		addExisting(filepath.Join(home, ".cargo", "registry"))
		addExisting(filepath.Join(home, ".cargo", "git"))
	}

	return dirs
}

// addCanonicalDir resolves p to an absolute, symlink-canonicalized path
// (falling back to the Abs/Clean form if EvalSymlinks fails) and appends it
// to *dirs unless already present.
func addCanonicalDir(dirs *[]string, seen map[string]bool, p string) {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if seen[abs] {
		return
	}
	seen[abs] = true
	*dirs = append(*dirs, abs)
}
