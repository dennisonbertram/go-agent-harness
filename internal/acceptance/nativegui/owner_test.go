package nativegui

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnerPreflightRejectsBeforeSpawnOrHTTP(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	spawned := 0
	hit := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit++ }))
	t.Cleanup(server.Close)
	owner := NewOwner(OwnerConfig{RepositoryRoot: source, TempParent: root, ForegroundOptIn: false, Spawn: func(context.Context, ChildSpec) (Child, error) { spawned++; return Child{}, nil }, HTTPGet: func(string) error { hit++; return nil }})
	if err := owner.Run(context.Background()); err == nil {
		t.Fatal("expected foreground preflight failure")
	}
	if spawned != 0 || hit != 0 {
		t.Fatalf("preflight caused effects: spawned=%d http=%d", spawned, hit)
	}
}

type fakeChild struct {
	pid     int
	stopped bool
	healthy bool
}

func (c *fakeChild) child() Child {
	return Child{PID: c.pid, Stop: func(context.Context) error { c.stopped = true; return nil }}
}
func sequenceSpawn(children ...*fakeChild) func(context.Context, ChildSpec) (Child, error) {
	i := 0
	return func(context.Context, ChildSpec) (Child, error) { c := children[i]; i++; return c.child(), nil }
}
func cleanRepository(t *testing.T, parent string) string {
	t.Helper()
	source := filepath.Join(parent, "source")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tracked"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.invalid"}, {"config", "user.name", "test"}, {"add", "."}, {"commit", "-m", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	return source
}

func TestOwnerFailureCleansOnlyRecordedChildrenAndPreservesSentinel(t *testing.T) {
	root := t.TempDir()
	source := cleanRepository(t, root)
	sentinel := fakeChild{pid: 9001, healthy: true}
	daemon := &fakeChild{pid: 1001}
	app := &fakeChild{pid: 1002}
	owner := NewOwner(OwnerConfig{RepositoryRoot: source, TempParent: root, ForegroundOptIn: true, Spawn: sequenceSpawn(daemon, app), Probe: func(context.Context, Attestation) error { return errors.New("probe failure") }})
	if err := owner.Run(context.Background()); err == nil {
		t.Fatal("expected probe failure")
	}
	if !daemon.stopped || !app.stopped {
		t.Fatalf("owned children not cleaned: daemon=%v app=%v", daemon.stopped, app.stopped)
	}
	if sentinel.stopped || !sentinel.healthy {
		t.Fatalf("sentinel was touched: %#v", sentinel)
	}
}

func TestOwnerRetainsPrivateArtifactRootAndCompletesAfterBoundedCleanup(t *testing.T) {
	parent := t.TempDir()
	source := cleanRepository(t, parent)
	daemon := &fakeChild{pid: 1001}
	app := &fakeChild{pid: 1002}
	var retained string
	owner := NewOwner(OwnerConfig{
		RepositoryRoot: source, TempParent: parent, ArtifactParent: parent,
		Nonce: strings.Repeat("n", 32), ForegroundOptIn: true,
		Spawn: sequenceSpawn(daemon, app),
		Probe: func(_ context.Context, attestation Attestation) error {
			retained = attestation.ArtifactRoot
			if attestation.Root == attestation.ArtifactRoot || !strings.HasPrefix(attestation.ArtifactRoot, parent) {
				t.Fatalf("runtime/artifact roots are not distinct and private: %#v", attestation)
			}
			return os.WriteFile(filepath.Join(attestation.ArtifactRoot, "diagnostic"), []byte("retained"), 0600)
		},
		Complete: func(_ context.Context, attestation Attestation, cleanup CoreCleanup, scenarioErr error) error {
			if scenarioErr != nil || !cleanup.Verified || !daemon.stopped || !app.stopped {
				t.Fatalf("completion preceded bounded cleanup: cleanup=%#v scenario=%v daemon=%v app=%v", cleanup, scenarioErr, daemon.stopped, app.stopped)
			}
			if _, err := os.Stat(attestation.Root); !os.IsNotExist(err) {
				t.Fatalf("disposable runtime root still exists: %v", err)
			}
			return nil
		},
	})
	if err := owner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(retained, "diagnostic")); err != nil {
		t.Fatalf("retained artifact missing: %v", err)
	}
}

func TestOwnerRetainsLaunchFailureDiagnosticRoot(t *testing.T) {
	parent := t.TempDir()
	launchErr := errors.New("owned daemon launch failed")
	var retained string
	owner := NewOwner(OwnerConfig{
		RepositoryRoot: cleanRepository(t, parent), TempParent: parent,
		Nonce: strings.Repeat("n", 32), ForegroundOptIn: true,
		Spawn: func(context.Context, ChildSpec) (Child, error) { return Child{}, launchErr },
		Complete: func(_ context.Context, attestation Attestation, cleanup CoreCleanup, primary error) error {
			retained = attestation.ArtifactRoot
			if !errors.Is(primary, launchErr) || !cleanup.Verified {
				t.Fatalf("completion did not retain launch/cleanup truth: primary=%v cleanup=%#v", primary, cleanup)
			}
			return os.WriteFile(filepath.Join(retained, "failure.json"), []byte(primary.Error()), 0600)
		},
	})
	if err := owner.Run(context.Background()); !errors.Is(err, launchErr) {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(retained, "failure.json")); err != nil {
		t.Fatalf("launch failure diagnostic missing: %v", err)
	}
}

func TestOwnerRetainsReservedListenerAgainstHijackerAndDoesNotTouchSentinel(t *testing.T) {
	root := t.TempDir()
	source := cleanRepository(t, root)
	sentinel := fakeChild{pid: 9001, healthy: true}
	daemon := &fakeChild{pid: 1001}
	app := &fakeChild{pid: 1002}
	spawned := 0
	owner := NewOwner(OwnerConfig{
		RepositoryRoot: source, TempParent: root, ForegroundOptIn: true,
		Spawn: func(_ context.Context, spec ChildSpec) (Child, error) {
			spawned++
			if spec.Kind == "daemon" {
				if spec.ListenerFile == nil {
					t.Fatal("daemon did not receive the inherited listener")
				}
				hijacker, err := net.Listen("tcp", spec.Endpoint)
				if err == nil {
					_ = hijacker.Close()
					t.Fatal("foreign hijacker bound the owner-reserved endpoint")
				}
				return daemon.child(), nil
			}
			if spec.ListenerFile != nil {
				t.Fatal("app received daemon-only inherited listener")
			}
			return app.child(), nil
		},
		Probe: func(context.Context, Attestation) error { return errors.New("probe failure") },
	})
	if err := owner.Run(context.Background()); err == nil {
		t.Fatal("expected probe failure")
	}
	if spawned != 2 || !daemon.stopped || !app.stopped || sentinel.stopped || !sentinel.healthy {
		t.Fatalf("unexpected lifecycle state: spawned=%d daemon=%#v app=%#v sentinel=%#v", spawned, daemon, app, sentinel)
	}
}

func TestJoinCleanupPreservesPrimaryAndAllCleanupErrors(t *testing.T) {
	primary := errors.New("primary failure")
	first := errors.New("first cleanup failure")
	second := errors.New("second cleanup failure")
	err := joinCleanup(primary,
		Child{Stop: func(context.Context) error { return first }},
		Child{Stop: func(context.Context) error { return second }},
	)
	for _, want := range []error{primary, first, second} {
		if !errors.Is(err, want) {
			t.Fatalf("cleanup result %v does not preserve %v", err, want)
		}
	}
}

func TestOwnerJoinsPrivateRootRemovalFailureWhenChmodFails(t *testing.T) {
	root := t.TempDir()
	chmodErr := errors.New("chmod private root")
	removeErr := errors.New("remove private root")
	owner := NewOwner(OwnerConfig{RepositoryRoot: cleanRepository(t, root), TempParent: root, ForegroundOptIn: true, Spawn: func(context.Context, ChildSpec) (Child, error) {
		t.Fatal("spawn after chmod failure")
		return Child{}, nil
	}})
	owner.system.chmod = func(string, os.FileMode) error { return chmodErr }
	owner.system.removeAll = func(string) error { return removeErr }
	err := owner.Run(context.Background())
	for _, want := range []error{chmodErr, removeErr} {
		if !errors.Is(err, want) {
			t.Fatalf("Run error %v does not preserve %v", err, want)
		}
	}
}

func TestOwnerJoinsListenerAndFileCloseFailures(t *testing.T) {
	root := t.TempDir()
	listenerCloseErr := errors.New("close owner listener")
	fileCloseErr := errors.New("close listener file")
	owner := NewOwner(OwnerConfig{RepositoryRoot: cleanRepository(t, root), TempParent: root, ForegroundOptIn: true, Spawn: func(context.Context, ChildSpec) (Child, error) {
		t.Fatal("spawn after listener close failure")
		return Child{}, nil
	}})
	owner.system.listenerFile = func(*net.TCPListener) (*os.File, error) {
		return os.CreateTemp(root, "listener-file")
	}
	owner.system.closeListener = func(listener net.Listener) error {
		if err := listener.Close(); err != nil {
			t.Fatalf("close test listener: %v", err)
		}
		return listenerCloseErr
	}
	owner.system.closeFile = func(file *os.File) error {
		if err := file.Close(); err != nil {
			t.Fatalf("close test file: %v", err)
		}
		return fileCloseErr
	}
	err := owner.Run(context.Background())
	for _, want := range []error{listenerCloseErr, fileCloseErr} {
		if !errors.Is(err, want) {
			t.Fatalf("Run error %v does not preserve %v", err, want)
		}
	}
}

func TestOwnerJoinsListenerFileCloseFailureWhenDaemonStartFails(t *testing.T) {
	root := t.TempDir()
	spawnErr := errors.New("daemon start")
	fileCloseErr := errors.New("close listener file")
	owner := NewOwner(OwnerConfig{RepositoryRoot: cleanRepository(t, root), TempParent: root, ForegroundOptIn: true, Spawn: func(_ context.Context, spec ChildSpec) (Child, error) {
		if spec.Kind != "daemon" {
			t.Fatalf("unexpected child %q", spec.Kind)
		}
		return Child{}, spawnErr
	}})
	owner.system.listenerFile = func(*net.TCPListener) (*os.File, error) {
		return os.CreateTemp(root, "listener-file")
	}
	owner.system.closeFile = func(file *os.File) error {
		if err := file.Close(); err != nil {
			t.Fatalf("close test file: %v", err)
		}
		return fileCloseErr
	}
	err := owner.Run(context.Background())
	for _, want := range []error{spawnErr, fileCloseErr} {
		if !errors.Is(err, want) {
			t.Fatalf("Run error %v does not preserve %v", err, want)
		}
	}
}

func TestOwnerRejectsSymlinkedOrDirtyRepositoryBeforeEffects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, root string) string
	}{
		{"symlink", func(t *testing.T, root string) string {
			outside := cleanRepository(t, t.TempDir())
			link := filepath.Join(root, "source")
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			return link
		}},
		{"dirty", func(t *testing.T, root string) string {
			source := cleanRepository(t, root)
			if err := os.WriteFile(filepath.Join(source, "dirty"), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
			return source
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			spawned := 0
			owner := NewOwner(OwnerConfig{RepositoryRoot: tc.prepare(t, root), TempParent: root, ForegroundOptIn: true, Spawn: func(context.Context, ChildSpec) (Child, error) { spawned++; return Child{}, nil }})
			if err := owner.Run(context.Background()); err == nil {
				t.Fatal("expected trust rejection")
			}
			if spawned != 0 {
				t.Fatalf("untrusted preflight spawned %d children", spawned)
			}
		})
	}
}

func TestOwnedProbeMustBeRegularPrivateFileAndIsAttested(t *testing.T) {
	root := t.TempDir()
	source := cleanRepository(t, root)
	probeSeen := false
	owner := NewOwner(OwnerConfig{
		RepositoryRoot: source, TempParent: root, ForegroundOptIn: true,
		Prepare: func(_ context.Context, privateRoot string) (string, error) {
			probe := filepath.Join(privateRoot, "probe")
			return probe, os.WriteFile(probe, []byte("fixed"), 0700)
		},
		Spawn: sequenceSpawn(&fakeChild{pid: 1001}, &fakeChild{pid: 1002}),
		Probe: func(_ context.Context, attestation Attestation) error {
			probeSeen = attestation.ProbePath != "" && strings.HasPrefix(attestation.ProbeDigest, "sha256:")
			return nil
		},
	})
	if err := owner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !probeSeen {
		t.Fatal("owner did not attest fixed private probe")
	}
	if _, err := canonicalOwnedFile(root, filepath.Join(root, "outside")); err == nil {
		t.Fatal("probe outside root must fail")
	}
}
