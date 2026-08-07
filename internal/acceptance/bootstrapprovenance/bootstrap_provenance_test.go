package bootstrapprovenance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitBuildsCleanLinkedWorktreeAgainstItsOwnGitMetadata(t *testing.T) {
	fixture := newFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(fixture.root, "README.md"), []byte("dirty parent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := runInit(t, fixture, nil, "--skip-download", "clean-linked-worktree")
	if result.err != nil {
		t.Fatalf("scripts/init.sh failed: %v\n%s", result.err, result.output)
	}

	child := filepath.Join(fixture.worktreeRoot, "clean-linked-worktree", "go-agent-harness")
	metadata := goBuildInfo(t, filepath.Join(child, ".tmp", "bootstrap", "bin", "harnessd"))
	if !strings.Contains(metadata, "vcs.revision="+fixture.revision) {
		t.Fatalf("harnessd revision did not come from clean child worktree:\n%s", metadata)
	}
	if !strings.Contains(metadata, "vcs.modified=false") {
		t.Fatalf("harnessd inherited dirty parent metadata:\n%s", metadata)
	}
}

func TestInitDefaultBaseUsesFetchedOriginMain(t *testing.T) {
	fixture := newFixtureRepository(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, fixture.root, "init", "--bare", remote)
	runGit(t, fixture.root, "remote", "add", "origin", remote)
	runGit(t, fixture.root, "push", "-u", "origin", "main")

	upstream := filepath.Join(t.TempDir(), "upstream")
	// A bare fixture's symbolic HEAD is host-config dependent. Check out the
	// branch under test explicitly so this commit is a fast-forward of origin/main
	// on every CI host rather than an unrelated initial branch.
	runGit(t, fixture.root, "clone", "--branch", "main", remote, upstream)
	runGit(t, upstream, "config", "user.email", "tests@example.invalid")
	runGit(t, upstream, "config", "user.name", "Bootstrap provenance test")
	writeFile(t, filepath.Join(upstream, "REMOTE_MAIN_MARKER"), "fetched origin main\n")
	runGit(t, upstream, "add", "REMOTE_MAIN_MARKER")
	runGit(t, upstream, "commit", "-m", "advance origin main")
	runGit(t, upstream, "push", "origin", "HEAD:main")
	expectedRevision := runGit(t, upstream, "rev-parse", "HEAD")

	result := runInit(t, fixture, nil, "--skip-download", "fetched-origin-main")
	if result.err != nil {
		t.Fatalf("scripts/init.sh failed: %v\n%s", result.err, result.output)
	}
	child := filepath.Join(fixture.worktreeRoot, "fetched-origin-main", "go-agent-harness")
	if _, err := os.Stat(filepath.Join(child, "REMOTE_MAIN_MARKER")); err != nil {
		t.Fatalf("default bootstrap did not use fetched origin/main: %v", err)
	}
	if got := runGit(t, child, "rev-parse", "HEAD"); got != expectedRevision {
		t.Fatalf("bootstrap HEAD = %s, want fetched origin/main %s", got, expectedRevision)
	}
	metadata := goBuildInfo(t, filepath.Join(child, ".tmp", "bootstrap", "bin", "harnessd"))
	if !strings.Contains(metadata, "vcs.revision="+expectedRevision) || !strings.Contains(metadata, "vcs.modified=false") {
		t.Fatalf("harnessd did not carry fetched clean origin/main metadata:\n%s", metadata)
	}
}

func TestInitVerifiesReusedWorktreeAgainstFreshRemoteBaseAndAcceptsExplicitSources(t *testing.T) {
	fixture := newFixtureRepository(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, fixture.root, "init", "--bare", remote)
	runGit(t, fixture.root, "remote", "add", "origin", remote)
	runGit(t, fixture.root, "push", "-u", "origin", "main")

	// Create the reusable worktree while local and remote main still agree.
	initial := runInit(t, fixture, nil, "--skip-download", "reused-provenance")
	if initial.err != nil {
		t.Fatalf("initial scripts/init.sh failed: %v\n%s", initial.err, initial.output)
	}

	upstream := filepath.Join(t.TempDir(), "upstream")
	runGit(t, fixture.root, "clone", "--branch", "main", remote, upstream)
	runGit(t, upstream, "config", "user.email", "tests@example.invalid")
	runGit(t, upstream, "config", "user.name", "Bootstrap provenance test")
	writeFile(t, filepath.Join(upstream, "REMOTE_MAIN_MARKER"), "new remote main\n")
	runGit(t, upstream, "add", "REMOTE_MAIN_MARKER")
	runGit(t, upstream, "commit", "-m", "advance origin main")
	runGit(t, upstream, "push", "origin", "HEAD:main")
	remoteRevision := runGit(t, upstream, "rev-parse", "HEAD")

	// Reuse intentionally preserves task commits, but it must refresh and record
	// the requested remote source independently of the worktree's current HEAD.
	reused := runInit(t, fixture, nil, "--skip-download", "reused-provenance")
	if reused.err != nil {
		t.Fatalf("scripts/init.sh did not preserve a reusable worktree: %v\n%s", reused.err, reused.output)
	}
	reusedChild := filepath.Join(fixture.worktreeRoot, "reused-provenance", "go-agent-harness")
	if got := runGit(t, reusedChild, "rev-parse", "HEAD"); got != fixture.revision {
		t.Fatalf("reused worktree HEAD = %s, want preserved task revision %s", got, fixture.revision)
	}
	envFile, err := os.ReadFile(filepath.Join(reusedChild, ".tmp", "bootstrap", "dev.env"))
	if err != nil {
		t.Fatalf("read reused worktree provenance: %v", err)
	}
	if !strings.Contains(string(envFile), "HARNESS_BOOTSTRAP_SOURCE_REVISION=\""+remoteRevision+"\"") ||
		!strings.Contains(string(envFile), "HARNESS_BOOTSTRAP_WORKTREE_REVISION=\""+fixture.revision+"\"") {
		t.Fatalf("reused worktree did not record distinct source and HEAD revisions:\n%s", envFile)
	}

	for _, source := range []struct {
		baseRef  string
		revision string
	}{
		{baseRef: "origin/main", revision: remoteRevision},
		{baseRef: remoteRevision, revision: remoteRevision},
		{baseRef: "refs/heads/main", revision: fixture.revision},
	} {
		t.Run(source.baseRef, func(t *testing.T) {
			slug := strings.NewReplacer("/", "-", ":", "-").Replace("explicit-" + source.baseRef)
			result := runInit(t, fixture, nil, "--base-ref", source.baseRef, "--skip-download", slug)
			if result.err != nil {
				t.Fatalf("scripts/init.sh --base-ref %q failed: %v\n%s", source.baseRef, result.err, result.output)
			}
			child := filepath.Join(fixture.worktreeRoot, slug, "go-agent-harness")
			if got := runGit(t, child, "rev-parse", "HEAD"); got != source.revision {
				t.Fatalf("bootstrap HEAD = %s, want explicit source %s", got, source.revision)
			}
		})
	}
}

func TestInitIgnoresInheritedExternalGitEnvironment(t *testing.T) {
	fixture := newFixtureRepository(t)
	external := newFixtureRepository(t)
	writeFile(t, filepath.Join(external.root, "EXTERNAL"), "not the bootstrap target\n")
	runGit(t, external.root, "add", "EXTERNAL")
	runGit(t, external.root, "commit", "-m", "external metadata")
	external.revision = runGit(t, external.root, "rev-parse", "HEAD")
	gitDir := runGit(t, external.root, "rev-parse", "--path-format=absolute", "--git-dir")

	result := runInit(t, fixture, []string{
		"GIT_DIR=" + gitDir,
		"GIT_WORK_TREE=" + external.root,
	}, "--skip-download", "external-git-environment")
	if result.err != nil {
		t.Fatalf("scripts/init.sh honored inherited external Git environment: %v\n%s", result.err, result.output)
	}

	child := filepath.Join(fixture.worktreeRoot, "external-git-environment", "go-agent-harness")
	metadata := goBuildInfo(t, filepath.Join(child, ".tmp", "bootstrap", "bin", "harnessd"))
	if !strings.Contains(metadata, "vcs.revision="+fixture.revision) || strings.Contains(metadata, "vcs.revision="+external.revision) {
		t.Fatalf("harnessd used external Git metadata:\n%s", metadata)
	}
	if !strings.Contains(metadata, "vcs.modified=false") {
		t.Fatalf("harnessd was not stamped clean:\n%s", metadata)
	}
}

func TestInitRejectsMismatchedBuildMetadataAndRemovesArtifact(t *testing.T) {
	fixture := newFixtureRepository(t)
	fakeBin := t.TempDir()
	writeExecutable(t, filepath.Join(fakeBin, "go"), `#!/bin/sh
set -eu
case "$1" in
  build)
    shift
    output=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-o" ]; then
        output="$2"
        shift 2
        continue
      fi
      shift
    done
    printf '#!/bin/sh\nexit 0\n' > "$output"
    chmod +x "$output"
    ;;
  version)
    printf '%s\n' 'binary: fake' 'build vcs=git' 'build vcs.revision=wrong-revision' 'build vcs.modified=false'
    ;;
  *) exit 0 ;;
esac
`)

	result := runInit(t, fixture, []string{"PATH=" + fakeBin + ":" + os.Getenv("PATH")}, "--skip-download", "mismatched-build-metadata")
	if result.err == nil {
		t.Fatalf("scripts/init.sh accepted mismatched build metadata:\n%s", result.output)
	}
	if !strings.Contains(result.output, "bootstrap provenance rejected") {
		t.Fatalf("error = %q, want fail-closed provenance rejection", result.output)
	}
	artifact := filepath.Join(fixture.worktreeRoot, "mismatched-build-metadata", "go-agent-harness", ".tmp", "bootstrap", "bin", "harnessd")
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("rejected bootstrap artifact remains available: %v", err)
	}
}

func TestInitBuildsFromDirectoryFormGitMetadataInLinkedWorktree(t *testing.T) {
	fixture := newFixtureRepository(t)
	fakeBin := t.TempDir()
	writeExecutable(t, filepath.Join(fakeBin, "go"), `#!/bin/sh
set -eu
case "$1" in
  build)
    shift
    output=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-o" ]; then
        output="$2"
        shift 2
        continue
      fi
      shift
    done
    if [ -d .git ]; then
      printf 'directory-form-git\n' > "$output"
    else
      printf 'linked-worktree-git-file\n' > "$output"
    fi
    chmod +x "$output"
    ;;
  version)
    if grep -q '^directory-form-git$' "$3"; then
      printf '%s\n' 'binary: fake' 'build vcs=git' 'build vcs.revision=`+fixture.revision+`' 'build vcs.modified=false'
    else
      printf '%s\n' 'binary: fake' 'build vcs=git' 'build vcs.revision=missing' 'build vcs.modified=missing'
    fi
    ;;
  *) exit 0 ;;
esac
`)

	result := runInit(t, fixture, []string{"PATH=" + fakeBin + ":" + os.Getenv("PATH")}, "--skip-download", "directory-form-git")
	if result.err != nil {
		t.Fatalf("scripts/init.sh did not build from an isolated directory-form .git checkout: %v\n%s", result.err, result.output)
	}

	artifact := filepath.Join(fixture.worktreeRoot, "directory-form-git", "go-agent-harness", ".tmp", "bootstrap", "bin", "harnessd")
	content, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read staged bootstrap artifact: %v", err)
	}
	if string(content) != "directory-form-git\n" {
		t.Fatalf("bootstrap build ran from linked-worktree metadata instead of an isolated .git directory: %q", content)
	}
}

type fixtureRepository struct {
	root         string
	worktreeRoot string
	revision     string
}

func newFixtureRepository(t *testing.T) fixtureRepository {
	t.Helper()
	root := canonicalFixturePath(t, filepath.Join(t.TempDir(), "repo"))
	for _, dir := range []string{"scripts", "cmd/harnessd", "cmd/harnesscli", "cmd/coveragegate"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(t, filepath.Join(repositoryRoot(t), "scripts", "init.sh"), filepath.Join(root, "scripts", "init.sh"), 0o755)
	writeFile(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.26\n")
	for _, command := range []string{"harnessd", "harnesscli", "coveragegate"} {
		writeFile(t, filepath.Join(root, "cmd", command, "main.go"), "package main\nfunc main() {}\n")
	}
	writeFile(t, filepath.Join(root, ".gitignore"), ".tmp/\n.codex-worktrees/\n")
	writeFile(t, filepath.Join(root, "README.md"), "clean child\n")
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "tests@example.invalid")
	runGit(t, root, "config", "user.name", "Bootstrap provenance test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	return fixtureRepository{
		root:         root,
		worktreeRoot: filepath.Join(root, ".test-worktrees"),
		revision:     runGit(t, root, "rev-parse", "HEAD"),
	}
}

// canonicalFixturePath makes the test's expected worktree identity match the
// canonical path reported by Git on macOS, where /var is a symlink to
// /private/var. The bootstrap assertion stays strict: it still compares the
// exact canonical child worktree that Git registered.
func canonicalFixturePath(t *testing.T, path string) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		t.Fatalf("canonicalize fixture parent %q: %v", path, err)
	}
	return filepath.Join(parent, filepath.Base(path))
}

type commandResult struct {
	output string
	err    error
}

func runInit(t *testing.T, fixture fixtureRepository, extraEnv []string, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"scripts/init.sh", "--worktree-root", fixture.worktreeRoot}, args...)...)
	cmd.Dir = fixture.root
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	return commandResult{output: string(out), err: err}
}

func goBuildInfo(t *testing.T, binary string) string {
	t.Helper()
	cmd := exec.Command("go", "version", "-m", binary)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go version -m %s: %v\n%s", binary, err, out)
	}
	return string(out)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

func copyFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, content, mode); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
