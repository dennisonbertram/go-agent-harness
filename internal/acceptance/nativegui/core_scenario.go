package nativegui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type CoreScenarioContract struct {
	Nonce        string
	FirstPrompt  string
	SecondPrompt string
	FirstReply   string
	SecondReply  string
}

func NewCoreScenarioContract(nonce string) (CoreScenarioContract, error) {
	if len(strings.TrimSpace(nonce)) < 32 {
		return CoreScenarioContract{}, fmt.Errorf("core rendered scenario requires a launcher nonce")
	}
	return CoreScenarioContract{
		Nonce:        nonce,
		FirstPrompt:  "Native rendered first prompt " + nonce,
		SecondPrompt: "Native rendered second prompt " + nonce,
		FirstReply:   "Native core tool result recorded for " + nonce + ".",
		SecondReply:  "Native second message continues the same conversation for " + nonce + ".",
	}, nil
}

// CorePlatform is the only rendered control/capture boundary. Implementations
// must target the attested PID and must not discover or attach to another app.
type CorePlatform interface {
	SubmitPrompt(context.Context, int, string) error
	AccessibilitySnapshot(context.Context, int) ([]byte, error)
	CaptureScreenshot(context.Context, int, string) error
}

type CoreScenarioRunner struct {
	Platform     CorePlatform
	HTTPClient   *http.Client
	PollInterval time.Duration
	Timeout      time.Duration
}

type coreRun struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Prompt         string    `json:"prompt"`
	Status         string    `json:"status"`
	Output         string    `json:"output"`
	CreatedAt      time.Time `json:"created_at"`
}

func (r CoreScenarioRunner) Run(ctx context.Context, attestation Attestation, contract CoreScenarioContract) (CoreProof, error) {
	if r.Platform == nil {
		return CoreProof{}, fmt.Errorf("core rendered scenario lacks platform adapter")
	}
	if attestation.AppPID <= 0 || attestation.DaemonPID <= 0 || attestation.AppPID == attestation.DaemonPID || attestation.ArtifactRoot == "" || attestation.Nonce != contract.Nonce {
		return CoreProof{}, fmt.Errorf("core rendered scenario lacks owner attestation")
	}
	root, err := canonicalDirectory(attestation.ArtifactRoot, "core rendered artifact root")
	if err != nil {
		return CoreProof{}, err
	}
	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	poll := r.PollInterval
	if poll <= 0 {
		poll = 200 * time.Millisecond
	}
	baseURL := "http://" + attestation.Endpoint
	first, err := waitForCoreRuns(ctx, client, baseURL, contract, 1, timeout, poll)
	if err != nil {
		return CoreProof{}, fmt.Errorf("observe first rendered run: %w", err)
	}
	if err := r.Platform.SubmitPrompt(ctx, attestation.AppPID, contract.SecondPrompt); err != nil {
		return CoreProof{}, fmt.Errorf("submit second rendered prompt: %w", err)
	}
	runs, err := waitForCoreRuns(ctx, client, baseURL, contract, 2, timeout, poll)
	if err != nil {
		return CoreProof{}, fmt.Errorf("observe second rendered run: %w", err)
	}
	if runs[0].ID != first[0].ID {
		return CoreProof{}, fmt.Errorf("core rendered first-run identity changed during continuation")
	}
	proof := CoreProof{
		SchemaVersion: "native-core-rendered-v1", Nonce: contract.Nonce,
		ArtifactRoot: root, ConversationID: runs[0].ConversationID,
		RunIDs:      []string{runs[0].ID, runs[1].ID},
		FirstPrompt: contract.FirstPrompt, SecondPrompt: contract.SecondPrompt,
		FirstReply: contract.FirstReply, SecondReply: contract.SecondReply,
		DaemonPID: attestation.DaemonPID, AppPID: attestation.AppPID,
	}
	paths := map[CoreArtifactKind]string{
		CoreArtifactScreenshot: filepath.Join(root, "screen.png"),
		CoreArtifactAX:         filepath.Join(root, "accessibility.json"),
		CoreArtifactRawSSE:     filepath.Join(root, "events.sse"),
		CoreArtifactAPIStore:   filepath.Join(root, "api-store.json"),
		CoreArtifactDaemonLog:  filepath.Join(root, "daemon.log"),
		CoreArtifactAppLog:     filepath.Join(root, "app.log"),
	}
	if err := r.Platform.CaptureScreenshot(ctx, attestation.AppPID, paths[CoreArtifactScreenshot]); err != nil {
		return proof, fmt.Errorf("capture rendered screenshot: %w", err)
	}
	ax, err := r.Platform.AccessibilitySnapshot(ctx, attestation.AppPID)
	if err != nil {
		return proof, fmt.Errorf("capture accessibility tree: %w", err)
	}
	for _, visible := range []string{contract.FirstPrompt, contract.SecondPrompt, contract.FirstReply, contract.SecondReply} {
		if !bytes.Contains(ax, []byte(visible)) {
			return proof, fmt.Errorf("accessibility tree lacks rendered transcript marker %q", visible)
		}
	}
	axArtifact, err := json.MarshalIndent(map[string]any{
		"nonce": contract.Nonce, "conversation_id": proof.ConversationID,
		"run_ids": proof.RunIDs, "tree": json.RawMessage(ax),
		"first_prompt": contract.FirstPrompt, "second_prompt": contract.SecondPrompt,
		"first_reply": contract.FirstReply, "second_reply": contract.SecondReply,
	}, "", "  ")
	if err != nil {
		return proof, fmt.Errorf("encode accessibility evidence: %w", err)
	}
	if err := os.WriteFile(paths[CoreArtifactAX], append(axArtifact, '\n'), 0600); err != nil {
		return proof, err
	}
	var rawSSE bytes.Buffer
	for _, runID := range proof.RunIDs {
		data, err := getBody(ctx, client, baseURL+"/v1/runs/"+runID+"/events")
		if err != nil {
			return proof, fmt.Errorf("collect raw SSE for %s: %w", runID, err)
		}
		rawSSE.WriteString("# run_id=" + runID + " conversation_id=" + proof.ConversationID + " nonce=" + contract.Nonce + "\n")
		rawSSE.Write(data)
		rawSSE.WriteByte('\n')
	}
	if err := os.WriteFile(paths[CoreArtifactRawSSE], rawSSE.Bytes(), 0600); err != nil {
		return proof, err
	}
	messages, err := getBody(ctx, client, baseURL+"/v1/conversations/"+proof.ConversationID+"/messages")
	if err != nil {
		return proof, err
	}
	runsJSON, err := getBody(ctx, client, baseURL+"/v1/conversations/"+proof.ConversationID+"/runs")
	if err != nil {
		return proof, err
	}
	apiArtifact, err := json.MarshalIndent(map[string]any{
		"nonce": contract.Nonce, "conversation_id": proof.ConversationID,
		"run_ids": proof.RunIDs, "messages": json.RawMessage(messages), "runs": json.RawMessage(runsJSON),
		"first_prompt": contract.FirstPrompt, "second_prompt": contract.SecondPrompt,
		"first_reply": contract.FirstReply, "second_reply": contract.SecondReply,
	}, "", "  ")
	if err != nil {
		return proof, err
	}
	if err := os.WriteFile(paths[CoreArtifactAPIStore], append(apiArtifact, '\n'), 0600); err != nil {
		return proof, err
	}
	for _, kind := range []CoreArtifactKind{CoreArtifactScreenshot, CoreArtifactAX, CoreArtifactRawSSE, CoreArtifactAPIStore, CoreArtifactDaemonLog, CoreArtifactAppLog} {
		proof.Artifacts = append(proof.Artifacts, CoreArtifact{Kind: kind, Path: paths[kind]})
	}
	return proof, nil
}

func waitForCoreRuns(ctx context.Context, client *http.Client, baseURL string, contract CoreScenarioContract, count int, timeout, poll time.Duration) ([]coreRun, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var lastErr error
	for {
		conversations, err := getBody(ctx, client, baseURL+"/v1/conversations/?limit=10")
		if err == nil {
			var listed struct {
				Conversations []struct {
					ID string `json:"id"`
				} `json:"conversations"`
			}
			if err = json.Unmarshal(conversations, &listed); err == nil {
				if len(listed.Conversations) != 1 {
					err = fmt.Errorf("expected exactly one isolated conversation, got %d", len(listed.Conversations))
				} else {
					rawRuns, getErr := getBody(ctx, client, baseURL+"/v1/conversations/"+listed.Conversations[0].ID+"/runs")
					if getErr == nil {
						var response struct {
							Runs []coreRun `json:"runs"`
						}
						if decodeErr := json.Unmarshal(rawRuns, &response); decodeErr == nil {
							completed := make([]coreRun, 0, len(response.Runs))
							for _, run := range response.Runs {
								if run.Status == "completed" {
									completed = append(completed, run)
								}
							}
							sort.Slice(completed, func(i, j int) bool { return completed[i].CreatedAt.Before(completed[j].CreatedAt) })
							if len(completed) == count {
								if validationErr := validateCoreRuns(completed, listed.Conversations[0].ID, contract, count); validationErr == nil {
									return completed, nil
								} else {
									err = validationErr
								}
							} else {
								err = fmt.Errorf("completed run count=%d want=%d", len(completed), count)
							}
						} else {
							err = decodeErr
						}
					} else {
						err = getErr
					}
				}
			}
		}
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return nil, errors.Join(ctx.Err(), lastErr)
		case <-time.After(poll):
		}
	}
}

func validateCoreRuns(runs []coreRun, conversationID string, contract CoreScenarioContract, count int) error {
	expectedPrompts := []string{contract.FirstPrompt, contract.SecondPrompt}
	expectedReplies := []string{contract.FirstReply, contract.SecondReply}
	for i := 0; i < count; i++ {
		if runs[i].ConversationID != conversationID || runs[i].Prompt != expectedPrompts[i] || runs[i].Output != expectedReplies[i] {
			return fmt.Errorf("run %d does not match deterministic conversation contract", i+1)
		}
	}
	return nil
}

func getBody(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %s: %s", url, response.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}
