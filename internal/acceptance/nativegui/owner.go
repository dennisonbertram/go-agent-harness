package nativegui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Owner is the only component allowed to create native-acceptance resources.
// It intentionally accepts no caller URL, driver, manifest, app path, or
// cleanup selector.
type Owner struct {
	config OwnerConfig
	system ownerSystem
}

type ownerSystem struct {
	mkdirTemp     func(string, string) (string, error)
	chmod         func(string, os.FileMode) error
	removeAll     func(string) error
	listen        func(string, string) (net.Listener, error)
	listenerFile  func(*net.TCPListener) (*os.File, error)
	closeListener func(net.Listener) error
	closeFile     func(*os.File) error
}
type OwnerConfig struct {
	RepositoryRoot  string
	TempParent      string
	ArtifactParent  string
	Nonce           string
	ForegroundOptIn bool
	// Prepare builds the fixed owner probe inside Root. It is deliberately not
	// a public command input: callers cannot substitute an executable.
	Prepare func(context.Context, string) (string, error)
	Spawn   func(context.Context, ChildSpec) (Child, error)
	Probe   func(context.Context, Attestation) error
	// Complete runs after exact child shutdown and runtime-root removal. It may
	// finalize a retained proof, but cannot turn an earlier scenario or cleanup
	// failure into success.
	Complete func(context.Context, Attestation, CoreCleanup, error) error
	HTTPGet  func(string) error // test seam; never called before preflight.
}
type ChildSpec struct {
	Kind, Root, ArtifactRoot, Endpoint, ProbePath string
	// ListenerFile is supplied only to the daemon. It is an inherited duplicate
	// of the owner-reserved loopback listener, never a caller-selected socket.
	ListenerFile *os.File
}
type Child struct {
	PID  int
	Stop func(context.Context) error
}
type Attestation struct {
	Root, ArtifactRoot, Nonce        string
	Endpoint, ProbePath, ProbeDigest string
	DaemonPID, AppPID                int
	ParentPID                        int
	StartedAt                        time.Time
}

func NewOwner(config OwnerConfig) *Owner {
	return &Owner{config: config, system: ownerSystem{
		mkdirTemp: os.MkdirTemp, chmod: os.Chmod, removeAll: os.RemoveAll,
		listen:        net.Listen,
		listenerFile:  func(listener *net.TCPListener) (*os.File, error) { return listener.File() },
		closeListener: func(listener net.Listener) error { return listener.Close() },
		closeFile:     func(file *os.File) error { return file.Close() },
	}}
}

func (o *Owner) Run(ctx context.Context) (err error) {
	if err := o.preflight(); err != nil {
		return err
	}
	root, err := o.system.mkdirTemp(o.config.TempParent, "native-gui-owned-*")
	if err != nil {
		return fmt.Errorf("create private root: %w", err)
	}
	if err := o.system.chmod(root, 0700); err != nil {
		return errors.Join(err, o.system.removeAll(root))
	}
	artifactParent := o.config.ArtifactParent
	if strings.TrimSpace(artifactParent) == "" {
		artifactParent = o.config.TempParent
	}
	artifactRoot, err := o.system.mkdirTemp(artifactParent, "native-gui-artifacts-*")
	if err != nil {
		return errors.Join(fmt.Errorf("create private artifact root: %w", err), o.system.removeAll(root))
	}
	if err := o.system.chmod(artifactRoot, 0700); err != nil {
		return errors.Join(err, o.system.removeAll(root), o.system.removeAll(artifactRoot))
	}
	var daemon, app Child
	attestation := Attestation{Root: root, ArtifactRoot: artifactRoot, Nonce: o.config.Nonce}
	defer func() {
		primary := err
		cleanupErr := joinCleanup(nil, app, daemon)
		removeErr := o.system.removeAll(root)
		cleanup := CoreCleanup{Verified: cleanupErr == nil && removeErr == nil, Detail: "stopped owner-created app and daemon; removed disposable runtime root"}
		if cleanupErr != nil || removeErr != nil {
			cleanup.Detail = errors.Join(cleanupErr, removeErr).Error()
		}
		var completeErr error
		if o.config.Complete != nil {
			completeErr = o.config.Complete(context.Background(), attestation, cleanup, primary)
		}
		err = errors.Join(primary, cleanupErr, removeErr, completeErr)
	}()
	probePath := ""
	if o.config.Prepare != nil {
		probePath, err = o.config.Prepare(ctx, root)
		if err != nil {
			return fmt.Errorf("prepare owned probe: %w", err)
		}
		probePath, err = canonicalOwnedFile(root, probePath)
		if err != nil {
			return err
		}
	}
	probeDigest := ""
	if probePath != "" {
		probeDigest, err = digestFile(probePath)
		if err != nil {
			return fmt.Errorf("digest owned probe: %w", err)
		}
	}
	listener, err := o.system.listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("reserve loopback endpoint: %w", err)
	}
	endpoint := listener.Addr().String()
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return errors.Join(fmt.Errorf("reserve loopback endpoint: expected TCP listener"), o.system.closeListener(listener))
	}
	listenerFile, err := o.system.listenerFile(tcpListener)
	if err != nil {
		return errors.Join(fmt.Errorf("duplicate loopback reservation: %w", err), o.system.closeListener(listener))
	}
	// The file descriptor remains open until exec has duplicated it into the
	// daemon. Closing the net.Listener never releases the duplicate, so no
	// foreign listener can claim this endpoint during startup or probing.
	if err := o.system.closeListener(listener); err != nil {
		return errors.Join(fmt.Errorf("close owner listener duplicate: %w", err), o.system.closeFile(listenerFile))
	}
	var spawnErr error
	daemon, spawnErr = o.spawn(ctx, ChildSpec{Kind: "daemon", Root: root, Endpoint: endpoint, ProbePath: probePath, ListenerFile: listenerFile, ArtifactRoot: artifactRoot})
	closeErr := o.system.closeFile(listenerFile)
	if spawnErr != nil {
		return errors.Join(spawnErr, closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close inherited listener duplicate: %w", closeErr)
	}
	app, err = o.spawn(ctx, ChildSpec{Kind: "app", Root: root, Endpoint: endpoint, ProbePath: probePath, ArtifactRoot: artifactRoot})
	if err != nil {
		return err
	}
	attestation.Endpoint, attestation.ProbePath, attestation.ProbeDigest = endpoint, probePath, probeDigest
	attestation.DaemonPID, attestation.AppPID = daemon.PID, app.PID
	attestation.ParentPID, attestation.StartedAt = os.Getpid(), time.Now().UTC()
	if daemon.PID <= 0 || app.PID <= 0 || daemon.PID == app.PID {
		return fmt.Errorf("invalid owned child identity")
	}
	if o.config.Probe != nil {
		return o.config.Probe(ctx, attestation)
	}
	return nil
}

func canonicalOwnedFile(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("owned probe must be absolute")
	}
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if rel, err := filepath.Rel(cleanRoot, cleanPath); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("owned probe escapes private root")
	}
	info, err := os.Lstat(cleanPath)
	if err != nil {
		return "", fmt.Errorf("stat owned probe: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("owned probe must be a regular non-symlink file")
	}
	return cleanPath, nil
}
func (o *Owner) preflight() error {
	if !o.config.ForegroundOptIn {
		return fmt.Errorf("foreground-control opt-in is required before native acceptance lifecycle start")
	}
	if o.config.Spawn == nil {
		return fmt.Errorf("owned child launcher is required")
	}
	root := strings.TrimSpace(o.config.RepositoryRoot)
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("repository root must be absolute")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("stat repository root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("repository root must be a non-symlink directory")
	}
	command := exec.Command("git", "-C", root, "status", "--porcelain")
	out, err := command.Output()
	if err != nil {
		return fmt.Errorf("verify repository source: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("repository source must be clean")
	}
	parent := strings.TrimSpace(o.config.TempParent)
	if parent == "" || !filepath.IsAbs(parent) {
		return fmt.Errorf("temporary parent must be absolute")
	}
	artifactParent := strings.TrimSpace(o.config.ArtifactParent)
	if artifactParent != "" && !filepath.IsAbs(artifactParent) {
		return fmt.Errorf("artifact parent must be absolute")
	}
	if o.config.Nonce != "" && len(strings.TrimSpace(o.config.Nonce)) < 32 {
		return fmt.Errorf("owner nonce must be at least 32 characters")
	}
	return nil
}
func (o *Owner) spawn(ctx context.Context, spec ChildSpec) (Child, error) {
	child, err := o.config.Spawn(ctx, spec)
	if err != nil {
		return Child{}, fmt.Errorf("start owned %s: %w", spec.Kind, err)
	}
	if child.PID <= 0 || child.Stop == nil {
		return Child{}, fmt.Errorf("owned %s lacks cleanup identity", spec.Kind)
	}
	return child, nil
}
func joinCleanup(primary error, children ...Child) error {
	for _, child := range children {
		if child.Stop != nil {
			if cleanupErr := child.Stop(context.Background()); cleanupErr != nil {
				primary = errors.Join(primary, fmt.Errorf("owned child cleanup: %w", cleanupErr))
			}
		}
	}
	return primary
}
func digestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
