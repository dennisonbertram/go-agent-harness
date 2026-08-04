package runtimeprovenance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardedLaunchRejectsDirtyMismatchedBinaryBeforeDaemonStarts(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "daemon-started")
	artifact := filepath.Join(tmp, "runtime-provenance.json")
	fakeGo := writeExecutable(t, tmp, "go", "#!/bin/sh\nprintf '%s\\n' 'binary: go1.test' 'build vcs=git' 'build vcs.revision=stale-revision' 'build vcs.modified=true'\n")
	daemon := writeExecutable(t, tmp, "harnessd", "#!/bin/sh\ntouch \"$DAEMON_MARKER\"\n")

	cmd := exec.Command("bash", "-c", ". \"$1\"; acceptance_runtime_provenance_check \"$2\" expected-revision \"$3\" && \"$2\"", "guard", filepath.Join(repoRoot, "scripts", "acceptance-runtime-provenance.sh"), daemon, artifact)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(fakeGo)+":"+os.Getenv("PATH"), "DAEMON_MARKER="+marker)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("guarded launch unexpectedly succeeded: %s", out)
	}
	if !strings.Contains(string(out), "runtime provenance rejected") {
		t.Fatalf("error = %q, want provenance rejection", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("daemon started before provenance rejection: %v", err)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("rejected binary wrote accepted artifact: %v", err)
	}
}

func TestGuardedLaunchRecordsCleanExactBuildBeforeDaemonStarts(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "daemon-started")
	artifact := filepath.Join(tmp, "runtime-provenance.json")
	fakeGo := writeExecutable(t, tmp, "go", "#!/bin/sh\nprintf '%s\\n' 'binary: go1.test' 'build vcs=git' 'build vcs.revision=expected-revision' 'build vcs.modified=false'\n")
	daemon := writeExecutable(t, tmp, "harnessd", "#!/bin/sh\ntest -f \"$RUNTIME_ARTIFACT\" || exit 17\ntouch \"$DAEMON_MARKER\"\n")

	cmd := exec.Command("bash", "-c", ". \"$1\"; acceptance_runtime_provenance_check \"$2\" expected-revision \"$3\" && RUNTIME_ARTIFACT=\"$3\" \"$2\"", "guard", filepath.Join(repoRoot, "scripts", "acceptance-runtime-provenance.sh"), daemon, artifact)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(fakeGo)+":"+os.Getenv("PATH"), "DAEMON_MARKER="+marker)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guarded launch failed: %v: %s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("daemon did not start after clean provenance: %v", err)
	}
	got, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.Contains(string(got), "expected-revision") || !strings.Contains(string(got), "sha256") {
		t.Fatalf("artifact missing revision/digest: %s", got)
	}
}

func TestGuardedLaunchRejectsArtifactWriteFailureBeforeDaemonStarts(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "daemon-started")
	blockedParent := filepath.Join(tmp, "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(blockedParent, "runtime-provenance.json")
	fakeGo := writeExecutable(t, tmp, "go", "#!/bin/sh\nprintf '%s\\n' 'binary: go1.test' 'build vcs=git' 'build vcs.revision=expected-revision' 'build vcs.modified=false'\n")
	daemon := writeExecutable(t, tmp, "harnessd", "#!/bin/sh\ntouch \"$DAEMON_MARKER\"\n")

	cmd := exec.Command("bash", "-c", ". \"$1\"; acceptance_runtime_provenance_check \"$2\" expected-revision \"$3\" && \"$2\"", "guard", filepath.Join(repoRoot, "scripts", "acceptance-runtime-provenance.sh"), daemon, artifact)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(fakeGo)+":"+os.Getenv("PATH"), "DAEMON_MARKER="+marker)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("guarded launch unexpectedly succeeded: %s", out)
	}
	if !strings.Contains(string(out), "could not create artifact directory") {
		t.Fatalf("error = %q, want artifact directory rejection", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("daemon started after artifact write failure: %v", err)
	}
}

func TestGuardedLaunchRejectsArtifactFileWriteFailureBeforeDaemonStarts(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "daemon-started")
	artifact := filepath.Join(tmp, "runtime-provenance.json")
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := writeExecutable(t, tmp, "go", "#!/bin/sh\nprintf '%s\\n' 'binary: go1.test' 'build vcs=git' 'build vcs.revision=expected-revision' 'build vcs.modified=false'\n")
	daemon := writeExecutable(t, tmp, "harnessd", "#!/bin/sh\ntouch \"$DAEMON_MARKER\"\n")

	cmd := exec.Command("bash", "-c", ". \"$1\"; acceptance_runtime_provenance_check \"$2\" expected-revision \"$3\" && \"$2\"", "guard", filepath.Join(repoRoot, "scripts", "acceptance-runtime-provenance.sh"), daemon, artifact)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(fakeGo)+":"+os.Getenv("PATH"), "DAEMON_MARKER="+marker)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("guarded launch unexpectedly succeeded: %s", out)
	}
	if !strings.Contains(string(out), "could not write artifact") {
		t.Fatalf("error = %q, want artifact write rejection", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("daemon started after artifact file write failure: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("repository root not found")
		}
		dir = next
	}
}

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
