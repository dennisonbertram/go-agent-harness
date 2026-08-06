// native-gui-acceptance owns a private native-app lifecycle. It deliberately
// accepts no URL, driver, manifest, bundle, or cleanup selector from callers.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	permissionState              = nativegui.PlatformPermissionState
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
	artifactRoot, err := runLifecycle(*foregroundOptIn)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "owner-created core rendered scenario completed; correlated artifacts: %s\n", artifactRoot)
	return nil
}

func ownedLifecycle(foregroundOptIn bool) (string, error) {
	repoRoot, err := workingDirectory()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	manifest, err := defaultScenarioManifest()
	if err != nil {
		return "", err
	}
	contract, err := nativegui.NewCoreScenarioContract(manifest.Nonce)
	if err != nil {
		return "", err
	}
	var proof nativegui.CoreProof
	var retainedRoot string
	config := nativegui.OwnerConfig{
		RepositoryRoot:  repoRoot,
		TempParent:      temporaryDirectory(),
		ArtifactParent:  temporaryDirectory(),
		Nonce:           manifest.Nonce,
		ForegroundOptIn: foregroundOptIn,
		Prepare:         prepareOwnedProbe(repoRoot),
		Spawn:           spawnOwnedChild(repoRoot, manifest),
		Probe: func(ctx context.Context, attestation nativegui.Attestation) error {
			retainedRoot = attestation.ArtifactRoot
			if err := probeOwnedDaemon(ctx, attestation); err != nil {
				return err
			}
			proof, err = (nativegui.CoreScenarioRunner{Platform: nativegui.DarwinCorePlatform{}}).Run(ctx, attestation, contract)
			return err
		},
		Complete: func(_ context.Context, attestation nativegui.Attestation, cleanup nativegui.CoreCleanup, scenarioErr error) error {
			retainedRoot = attestation.ArtifactRoot
			if scenarioErr != nil || !cleanup.Verified {
				return writeFailureDiagnostic(attestation, proof, cleanup, scenarioErr)
			}
			proof.Cleanup = cleanup
			if err := proof.SealArtifacts(); err != nil {
				return errors.Join(err, writeFailureDiagnostic(attestation, proof, cleanup, err))
			}
			if err := nativegui.WriteCoreProof(filepath.Join(attestation.ArtifactRoot, "proof.json"), proof); err != nil {
				return errors.Join(err, writeFailureDiagnostic(attestation, proof, cleanup, err))
			}
			return nil
		},
	}
	err = (nativegui.RenderedDriver{
		Permissions: permissionState,
		Start:       func(ctx context.Context) error { return runOwnedOwner(config) },
	}).Run(context.Background())
	if err != nil && retainedRoot != "" {
		return retainedRoot, fmt.Errorf("%w (diagnostics retained at %s)", err, retainedRoot)
	}
	return retainedRoot, err
}

func writeFailureDiagnostic(attestation nativegui.Attestation, proof nativegui.CoreProof, cleanup nativegui.CoreCleanup, primary error) error {
	detail := "cleanup was not verified"
	if primary != nil {
		detail = primary.Error()
	}
	data, err := json.MarshalIndent(map[string]any{
		"schema_version": "native-core-rendered-failure-v1", "nonce": attestation.Nonce,
		"conversation_id": proof.ConversationID, "run_ids": proof.RunIDs,
		"error": detail, "cleanup": cleanup,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode native failure diagnostic: %w", err)
	}
	if err := os.WriteFile(filepath.Join(attestation.ArtifactRoot, "failure.json"), append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("retain native failure diagnostic: %w", err)
	}
	return nil
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
		var logName string
		switch spec.Kind {
		case "daemon":
			logName = "daemon.log"
			turns := filepath.Join(spec.Root, "fake-turns.json")
			if err := nativegui.WriteFakeProviderTurns(spec.Root, "fake-turns.json", manifest); err != nil {
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
			command.Env = append(os.Environ(), "HARNESS_PROVIDER=fake", "HARNESS_FAKE_TURNS="+turns, "HARNESS_MODEL=fake-model", "HARNESS_WORKSPACE="+workspace, "HARNESS_ADDR="+spec.Endpoint, "HARNESS_LISTEN_FD=3", "HARNESS_AUTH_DISABLED=true", "HARNESS_MEMORY_MODE=off", "HARNESS_ENABLE_CALLBACKS=false", "HARNESS_GLOBAL_DIR="+filepath.Join(spec.Root, "global"), "HARNESS_RUN_DB="+filepath.Join(spec.Root, "runs.db"), "HARNESS_CONVERSATION_DB="+filepath.Join(spec.Root, "conversations.db"))
		case "app":
			logName = "app.log"
			buildDir := filepath.Join(spec.Root, "swift-build")
			if output, err := exec.CommandContext(ctx, "swift", "build", "--package-path", filepath.Join(repoRoot, "macapp"), "--build-path", buildDir, "--product", "GoCode").CombinedOutput(); err != nil {
				return nativegui.Child{}, fmt.Errorf("build owned native app: %w: %s", err, output)
			}
			appBinary := filepath.Join(buildDir, "debug", "GoCode")
			command = exec.Command(appBinary)
			command.Dir = spec.Root
			contract, err := nativegui.NewCoreScenarioContract(manifest.Nonce)
			if err != nil {
				return nativegui.Child{}, err
			}
			command.Env = append(os.Environ(), "HARNESS_BASE_URL=http://"+spec.Endpoint, "HARNESS_WORKSPACE="+filepath.Join(spec.Root, "workspace"), "GOCODE_INITIAL_PROMPT="+contract.FirstPrompt)
		default:
			return nativegui.Child{}, fmt.Errorf("unknown owned child kind %q", spec.Kind)
		}
		if spec.ArtifactRoot == "" {
			return nativegui.Child{}, fmt.Errorf("owned %s lacks artifact root", spec.Kind)
		}
		logFile, err := os.OpenFile(filepath.Join(spec.ArtifactRoot, logName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nativegui.Child{}, fmt.Errorf("open owned %s log: %w", spec.Kind, err)
		}
		if _, err := fmt.Fprintf(logFile, "owner-created %s starting\n", spec.Kind); err != nil {
			_ = logFile.Close()
			return nativegui.Child{}, err
		}
		command.Stdout, command.Stderr = logFile, logFile
		if err := command.Start(); err != nil {
			_ = logFile.Close()
			return nativegui.Child{}, err
		}
		return nativegui.Child{PID: command.Process.Pid, Stop: func(stopCtx context.Context) error {
			_ = command.Process.Signal(os.Interrupt)
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			select {
			case err := <-done:
				closeErr := logFile.Close()
				if err != nil {
					return closeErr
				}
				return closeErr
			case <-stopCtx.Done():
				_ = command.Process.Kill()
				<-done
				return errors.Join(stopCtx.Err(), logFile.Close())
			case <-time.After(10 * time.Second):
				_ = command.Process.Kill()
				<-done
				return errors.Join(fmt.Errorf("owned %s did not stop", spec.Kind), logFile.Close())
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
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("owned daemon PID %d never became healthy at its recorded endpoint", attestation.DaemonPID)
}
