package nativegui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOwnerPreflightRejectsBeforeSpawnOrHTTP(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0700); err != nil { t.Fatal(err) }
	spawned := 0
	hit := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit++ }))
	t.Cleanup(server.Close)
	owner := NewOwner(OwnerConfig{RepositoryRoot: source, TempParent: root, ForegroundOptIn: false, Spawn: func(context.Context, ChildSpec) (Child, error) { spawned++; return Child{}, nil }, HTTPGet: func(string) error { hit++; return nil }})
	if err := owner.Run(context.Background()); err == nil { t.Fatal("expected foreground preflight failure") }
	if spawned != 0 || hit != 0 { t.Fatalf("preflight caused effects: spawned=%d http=%d", spawned, hit) }
}

type fakeChild struct { pid int; stopped bool; healthy bool }
func (c *fakeChild) child() Child { return Child{PID: c.pid, Stop: func(context.Context) error { c.stopped = true; return nil }} }
func sequenceSpawn(children ...*fakeChild) func(context.Context, ChildSpec) (Child, error) { i := 0; return func(context.Context, ChildSpec) (Child, error) { c := children[i]; i++; return c.child(), nil } }
func cleanRepository(t *testing.T, parent string) string {
	t.Helper(); source := filepath.Join(parent, "source")
	if err := os.MkdirAll(source, 0700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(source, "tracked"), []byte("fixture"), 0600); err != nil { t.Fatal(err) }
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.invalid"}, {"config", "user.name", "test"}, {"add", "."}, {"commit", "-m", "initial"}} { cmd := exec.Command("git", args...); cmd.Dir = source; if output, err := cmd.CombinedOutput(); err != nil { t.Fatalf("git %v: %v: %s", args, err, output) } }
	return source
}

func TestOwnerFailureCleansOnlyRecordedChildrenAndPreservesSentinel(t *testing.T) {
	root := t.TempDir()
	source := cleanRepository(t, root)
	sentinel := fakeChild{pid: 9001, healthy: true}
	daemon := &fakeChild{pid: 1001}
	app := &fakeChild{pid: 1002}
	owner := NewOwner(OwnerConfig{RepositoryRoot: source, TempParent: root, ForegroundOptIn: true, Spawn: sequenceSpawn(daemon, app), Probe: func(context.Context, Attestation) error { return errors.New("probe failure") }})
	if err := owner.Run(context.Background()); err == nil { t.Fatal("expected probe failure") }
	if !daemon.stopped || !app.stopped { t.Fatalf("owned children not cleaned: daemon=%v app=%v", daemon.stopped, app.stopped) }
	if sentinel.stopped || !sentinel.healthy { t.Fatalf("sentinel was touched: %#v", sentinel) }
}

func TestOwnerRejectsSymlinkedOrDirtyRepositoryBeforeEffects(t *testing.T) {
	for _, tc := range []struct { name string; prepare func(t *testing.T, root string) string }{
		{"symlink", func(t *testing.T, root string) string { outside := cleanRepository(t, t.TempDir()); link := filepath.Join(root, "source"); if err := os.Symlink(outside, link); err != nil { t.Fatal(err) }; return link }},
		{"dirty", func(t *testing.T, root string) string { source := cleanRepository(t, root); if err := os.WriteFile(filepath.Join(source, "dirty"), []byte("x"), 0600); err != nil { t.Fatal(err) }; return source }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir(); spawned := 0
			owner := NewOwner(OwnerConfig{RepositoryRoot: tc.prepare(t, root), TempParent: root, ForegroundOptIn: true, Spawn: func(context.Context, ChildSpec) (Child, error) { spawned++; return Child{}, nil }})
			if err := owner.Run(context.Background()); err == nil { t.Fatal("expected trust rejection") }
			if spawned != 0 { t.Fatalf("untrusted preflight spawned %d children", spawned) }
		})
	}
}
