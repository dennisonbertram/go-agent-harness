// native-gui-acceptance owns a private native-app lifecycle. It deliberately
// accepts no URL, driver, manifest, bundle, or cleanup selector from callers.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go-agent-harness/internal/acceptance/nativegui"
)

var (
	commandArgs                  = os.Args[1:]
	stdout             io.Writer = os.Stdout
	stderr             io.Writer = os.Stderr
	exitFunc                     = os.Exit
	runMain                      = run
	runLifecycle                 = ownedLifecycle
	workingDirectory             = os.Getwd
	temporaryDirectory           = os.TempDir
	runOwnedOwner                = func(config nativegui.OwnerConfig) error { return nativegui.NewOwner(config).Run(context.Background()) }
)

func main() {
	if err := runMain(commandArgs, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "native-gui-acceptance:", err)
		exitFunc(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("native-gui-acceptance", flag.ContinueOnError)
	flags.SetOutput(errOut)
	foregroundOptIn := flags.Bool("foreground-opt-in", false, "explicitly allow the owner to launch its isolated native app")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("native acceptance accepts no positional inputs")
	}
	if !*foregroundOptIn {
		return fmt.Errorf("-foreground-opt-in is required; no native app was launched")
	}
	if err := runLifecycle(*foregroundOptIn); err != nil {
		return err
	}
	fmt.Fprintln(out, "owned native acceptance lifecycle completed; no rendered scenario was executed")
	return nil
}

func ownedLifecycle(foregroundOptIn bool) error {
	repoRoot, err := workingDirectory()
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	manifest, err := defaultScenarioManifest()
	if err != nil {
		return err
	}
	config := nativegui.OwnerConfig{
		RepositoryRoot:  repoRoot,
		TempParent:      temporaryDirectory(),
		ForegroundOptIn: foregroundOptIn,
		Prepare:         prepareOwnedProbe(repoRoot),
		Spawn:           spawnOwnedChild(repoRoot, manifest),
		Probe:           probeOwnedDaemon,
	}
	return runOwnedOwner(config)
}

func defaultScenarioManifest() (nativegui.FakeProviderScenarioManifest, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nativegui.FakeProviderScenarioManifest{}, fmt.Errorf("generate native scenario nonce: %w", err)
	}
	manifest := nativegui.DefaultFakeProviderScenarioManifest(hex.EncodeToString(bytes))
	if err := nativegui.ValidateFakeProviderScenarioManifest(manifest); err != nil {
		return nativegui.FakeProviderScenarioManifest{}, fmt.Errorf("preflight owned native scenarios: %w", err)
	}
	return manifest, nil
}

func prepareOwnedProbe(repoRoot string) func(context.Context, string) (string, error) {
	return func(ctx context.Context, root string) (string, error) {
		binDir := filepath.Join(root, "bin")
		if err := os.MkdirAll(binDir, 0700); err != nil {
			return "", err
		}
		binary := filepath.Join(binDir, "harnessd")
		build := exec.CommandContext(ctx, "go", "build", "-buildvcs=true", "-o", binary, "./cmd/harnessd")
		build.Dir = repoRoot
		if output, err := build.CombinedOutput(); err != nil {
			return "", fmt.Errorf("build fixed harnessd probe: %w: %s", err, output)
		}
		return binary, nil
	}
}

func spawnOwnedChild(repoRoot string, manifest nativegui.FakeProviderScenarioManifest) func(context.Context, nativegui.ChildSpec) (nativegui.Child, error) {
	return func(ctx context.Context, spec nativegui.ChildSpec) (nativegui.Child, error) {
		var command *exec.Cmd
		switch spec.Kind {
		case "daemon":
			turns := filepath.Join(spec.Root, "fake-turns.json")
			if err := nativegui.WriteFakeProviderTurns(turns, manifest); err != nil {
				return nativegui.Child{}, err
			}
			workspace := filepath.Join(spec.Root, "workspace")
			if err := os.MkdirAll(workspace, 0700); err != nil {
				return nativegui.Child{}, err
			}
			if spec.ListenerFile == nil {
				return nativegui.Child{}, fmt.Errorf("owned daemon lacks inherited listener")
			}
			command = exec.Command(spec.ProbePath)
			command.Dir = workspace
			command.ExtraFiles = []*os.File{spec.ListenerFile}
			command.Env = append(os.Environ(), "HARNESS_PROVIDER=fake", "HARNESS_FAKE_TURNS="+turns, "HARNESS_MODEL=fake-model", "HARNESS_WORKSPACE="+workspace, "HARNESS_ADDR="+spec.Endpoint, "HARNESS_LISTEN_FD=3", "HARNESS_AUTH_DISABLED=true", "HARNESS_MEMORY_MODE=off", "HARNESS_GLOBAL_DIR="+filepath.Join(spec.Root, "global"))
		case "app":
			buildDir := filepath.Join(spec.Root, "swift-build")
			if output, err := exec.CommandContext(ctx, "swift", "build", "--package-path", filepath.Join(repoRoot, "macapp"), "--build-path", buildDir, "--product", "GoCode").CombinedOutput(); err != nil {
				return nativegui.Child{}, fmt.Errorf("build owned native app: %w: %s", err, output)
			}
			appBinary := filepath.Join(buildDir, "debug", "GoCode")
			command = exec.Command(appBinary)
			command.Dir = spec.Root
			command.Env = append(os.Environ(), "HARNESS_BASE_URL=http://"+spec.Endpoint, "HARNESS_WORKSPACE="+filepath.Join(spec.Root, "workspace"))
		default:
			return nativegui.Child{}, fmt.Errorf("unknown owned child kind %q", spec.Kind)
		}
		if err := command.Start(); err != nil {
			return nativegui.Child{}, err
		}
		return nativegui.Child{PID: command.Process.Pid, Stop: func(stopCtx context.Context) error {
			_ = command.Process.Signal(os.Interrupt)
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			select {
			case err := <-done:
				if err != nil {
					return nil
				}
				return nil
			case <-stopCtx.Done():
				_ = command.Process.Kill()
				<-done
				return stopCtx.Err()
			case <-time.After(10 * time.Second):
				_ = command.Process.Kill()
				<-done
				return fmt.Errorf("owned %s did not stop", spec.Kind)
			}
		}}, nil
	}
}

func probeOwnedDaemon(ctx context.Context, attestation nativegui.Attestation) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+attestation.Endpoint+"/healthz", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("owned daemon PID %d never became healthy at its recorded endpoint", attestation.DaemonPID)
}
