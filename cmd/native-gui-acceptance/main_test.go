package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/acceptance/nativegui"
)

func TestMainReportsEntrypointError(t *testing.T) {
	oldArgs, oldRun, oldOut, oldErr, oldExit := commandArgs, runMain, stdout, stderr, exitFunc
	t.Cleanup(func() { commandArgs, runMain, stdout, stderr, exitFunc = oldArgs, oldRun, oldOut, oldErr, oldExit })
	var out, errOut bytes.Buffer
	commandArgs, stdout, stderr = nil, &out, &errOut
	runMain = func([]string, io.Writer, io.Writer) error { return errors.New("fixture failure") }
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	main()
	if exitCode != 1 || !strings.Contains(errOut.String(), "fixture failure") {
		t.Fatalf("exit=%d stderr=%q", exitCode, errOut.String())
	}
}

func TestValidateLauncherBindingRejectsUnboundOrDifferentCollection(t *testing.T) {
	manifest := nativegui.Manifest{Collection: nativegui.CollectionProvenance{Launcher: "scripts/run-native-gui-acceptance.sh", Nonce: "nonce", TempRoot: "/tmp/root", ArtifactRoot: "/tmp/root/artifacts", RepositoryRoot: "/repo", DriverPath: "/repo/driver", DriverDigest: "sha256:digest", DaemonURL: "http://127.0.0.1:1234"}}
	if err := validateLauncherBinding("http://127.0.0.1:1234", manifest); err == nil {
		t.Fatal("expected absent launcher environment to fail")
	}
	for name, value := range map[string]string{
		"NATIVE_GUI_COLLECTION_LAUNCHER":        manifest.Collection.Launcher,
		"NATIVE_GUI_COLLECTION_NONCE":           manifest.Collection.Nonce,
		"NATIVE_GUI_COLLECTION_TEMP_ROOT":       manifest.Collection.TempRoot,
		"NATIVE_GUI_COLLECTION_ARTIFACT_ROOT":   manifest.Collection.ArtifactRoot,
		"NATIVE_GUI_COLLECTION_REPOSITORY_ROOT": manifest.Collection.RepositoryRoot,
		"NATIVE_GUI_COLLECTION_DRIVER_PATH":     manifest.Collection.DriverPath,
		"NATIVE_GUI_COLLECTION_DRIVER_DIGEST":   manifest.Collection.DriverDigest,
	} {
		t.Setenv(name, value)
	}
	if err := validateLauncherBinding("http://127.0.0.1:1234", manifest); err != nil {
		t.Fatal(err)
	}
	if err := validateLauncherBinding("http://127.0.0.1:4321", manifest); err == nil {
		t.Fatal("expected URL mismatch to fail")
	}
}

func TestRunRejectsMissingArguments(t *testing.T) {
	if err := run(nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err=%v", err)
	}
}

func TestLiveInventoryCompilesAndRejectsAbsentResolverEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{"complete", `{"tools":[{"name":"read","description":"read","tier":"core","owner":"harness.default.core","condition":"built-in runtime registry"}],"configured_unavailable_toolsets":[],"unavailable":[]}`, false},
		{"absent", `{"tools":[]}`, true},
		{"status", `bad`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.name == "status" {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			compiled, err := liveInventory(server.URL)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || compiled.Hash == "" {
				t.Fatalf("compiled=%#v err=%v", compiled, err)
			}
		})
	}
}
