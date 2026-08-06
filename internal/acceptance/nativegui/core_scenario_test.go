package nativegui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeCorePlatform struct {
	contract  CoreScenarioContract
	submitted atomic.Bool
}

func (f *fakeCorePlatform) SubmitPrompt(_ context.Context, pid int, prompt string) error {
	if pid != 102 || prompt != f.contract.SecondPrompt {
		return os.ErrInvalid
	}
	f.submitted.Store(true)
	return nil
}
func (f *fakeCorePlatform) AccessibilitySnapshot(context.Context, int) ([]byte, error) {
	return json.Marshal(map[string]any{"role": "AXApplication", "value": strings.Join([]string{f.contract.FirstPrompt, f.contract.SecondPrompt, f.contract.FirstReply, f.contract.SecondReply}, " ")})
}
func (f *fakeCorePlatform) CaptureScreenshot(_ context.Context, _ int, path string) error {
	return os.WriteFile(path, append([]byte("\x89PNG\r\n\x1a\n"), []byte("rendered pixels")...), 0600)
}

func TestCoreScenarioRunnerCorrelatesTwoRenderedRunsAndAllArtifacts(t *testing.T) {
	nonce := strings.Repeat("n", 32)
	contract, err := NewCoreScenarioContract(nonce)
	if err != nil {
		t.Fatal(err)
	}
	platform := &fakeCorePlatform{contract: contract}
	created := time.Now().UTC()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/conversations/":
			_, _ = w.Write([]byte(`{"conversations":[{"id":"conversation-1"}]}`))
		case "/v1/conversations/conversation-1/runs":
			runs := []coreRun{{ID: "run-1", ConversationID: "conversation-1", Prompt: contract.FirstPrompt, Status: "completed", Output: contract.FirstReply, CreatedAt: created}}
			if platform.submitted.Load() {
				runs = append([]coreRun{{ID: "run-2", ConversationID: "conversation-1", Prompt: contract.SecondPrompt, Status: "completed", Output: contract.SecondReply, CreatedAt: created.Add(time.Second)}}, runs...)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"runs": runs})
		case "/v1/conversations/conversation-1/messages":
			_ = json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]string{{"role": "user", "content": contract.FirstPrompt}, {"role": "assistant", "content": contract.FirstReply}, {"role": "user", "content": contract.SecondPrompt}, {"role": "assistant", "content": contract.SecondReply}}})
		case "/v1/runs/run-1/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: run.started\ndata: " + contract.FirstPrompt + "\nevent: assistant.message\ndata: " + contract.FirstReply + "\n"))
		case "/v1/runs/run-2/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: run.started\ndata: " + contract.SecondPrompt + "\nevent: assistant.message\ndata: " + contract.SecondReply + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	for _, name := range []string{"daemon.log", "app.log"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("owned "+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	endpoint := strings.TrimPrefix(server.URL, "http://")
	proof, err := (CoreScenarioRunner{Platform: platform, HTTPClient: server.Client(), PollInterval: time.Millisecond, Timeout: time.Second}).Run(context.Background(), Attestation{
		ArtifactRoot: root, Nonce: nonce, Endpoint: endpoint, DaemonPID: 101, AppPID: 102,
	}, contract)
	if err != nil {
		t.Fatal(err)
	}
	proof.Cleanup = CoreCleanup{Verified: true, Detail: "bounded owner cleanup"}
	if err := proof.SealArtifacts(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCoreProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestCoreScenarioRunnerRejectsMixedOrExtraConversationState(t *testing.T) {
	nonce := strings.Repeat("n", 32)
	contract, _ := NewCoreScenarioContract(nonce)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/conversations/" {
			_, _ = w.Write([]byte(`{"conversations":[{"id":"one"},{"id":"foreign"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	_, err := (CoreScenarioRunner{Platform: &fakeCorePlatform{contract: contract}, HTTPClient: server.Client(), PollInterval: time.Millisecond, Timeout: 20 * time.Millisecond}).Run(context.Background(), Attestation{
		ArtifactRoot: t.TempDir(), Nonce: nonce, Endpoint: strings.TrimPrefix(server.URL, "http://"), DaemonPID: 1, AppPID: 2,
	}, contract)
	if err == nil || !strings.Contains(err.Error(), "exactly one isolated conversation") {
		t.Fatalf("err=%v", err)
	}
}
