package nativegui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
type Owner struct{ config OwnerConfig }
type OwnerConfig struct {
	RepositoryRoot string
	TempParent string
	ForegroundOptIn bool
	Spawn func(context.Context, ChildSpec) (Child, error)
	Probe func(context.Context, Attestation) error
	HTTPGet func(string) error // test seam; never called before preflight.
}
type ChildSpec struct { Kind, Root, Endpoint, ProbePath string }
type Child struct { PID int; Stop func(context.Context) error }
type Attestation struct { Root, Endpoint, ProbePath, ProbeDigest string; DaemonPID, AppPID int; ParentPID int; StartedAt time.Time }

func NewOwner(config OwnerConfig) *Owner { return &Owner{config: config} }

func (o *Owner) Run(ctx context.Context) (err error) {
	if err := o.preflight(); err != nil { return err }
	root, err := os.MkdirTemp(o.config.TempParent, "native-gui-owned-*")
	if err != nil { return fmt.Errorf("create private root: %w", err) }
	if err := os.Chmod(root, 0700); err != nil { _ = os.RemoveAll(root); return err }
	defer func() { if err == nil { err = os.RemoveAll(root) } }()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return fmt.Errorf("reserve loopback endpoint: %w", err) }
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil { return fmt.Errorf("release loopback reservation: %w", err) }
	daemon, err := o.spawn(ctx, ChildSpec{Kind: "daemon", Root: root, Endpoint: endpoint})
	if err != nil { return err }
	var app Child
	defer func() { err = joinCleanup(err, app, daemon) }()
	app, err = o.spawn(ctx, ChildSpec{Kind: "app", Root: root, Endpoint: endpoint})
	if err != nil { return err }
	attestation := Attestation{Root: root, Endpoint: endpoint, DaemonPID: daemon.PID, AppPID: app.PID, ParentPID: os.Getpid(), StartedAt: time.Now().UTC()}
	if daemon.PID <= 0 || app.PID <= 0 || daemon.PID == app.PID { return fmt.Errorf("invalid owned child identity") }
	if o.config.Probe != nil { return o.config.Probe(ctx, attestation) }
	return nil
}
func (o *Owner) preflight() error {
	if !o.config.ForegroundOptIn { return fmt.Errorf("foreground-control opt-in is required before native acceptance lifecycle start") }
	if o.config.Spawn == nil { return fmt.Errorf("owned child launcher is required") }
	root := strings.TrimSpace(o.config.RepositoryRoot)
	if root == "" || !filepath.IsAbs(root) { return fmt.Errorf("repository root must be absolute") }
	info, err := os.Lstat(root); if err != nil { return fmt.Errorf("stat repository root: %w", err) }
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() { return fmt.Errorf("repository root must be a non-symlink directory") }
	command := exec.Command("git", "-C", root, "status", "--porcelain")
	out, err := command.Output(); if err != nil { return fmt.Errorf("verify repository source: %w", err) }
	if strings.TrimSpace(string(out)) != "" { return fmt.Errorf("repository source must be clean") }
	parent := strings.TrimSpace(o.config.TempParent)
	if parent == "" || !filepath.IsAbs(parent) { return fmt.Errorf("temporary parent must be absolute") }
	return nil
}
func (o *Owner) spawn(ctx context.Context, spec ChildSpec) (Child, error) { child, err := o.config.Spawn(ctx, spec); if err != nil { return Child{}, fmt.Errorf("start owned %s: %w", spec.Kind, err) }; if child.PID <= 0 || child.Stop == nil { return Child{}, fmt.Errorf("owned %s lacks cleanup identity", spec.Kind) }; return child, nil }
func joinCleanup(primary error, children ...Child) error { for _, child := range children { if child.Stop != nil { if cleanupErr := child.Stop(context.Background()); cleanupErr != nil && primary == nil { primary = fmt.Errorf("owned child cleanup: %w", cleanupErr) } } }; return primary }
func digestFile(path string) (string, error) { data, err := os.ReadFile(path); if err != nil { return "", err }; sum := sha256.Sum256(data); return "sha256:" + hex.EncodeToString(sum[:]), nil }
