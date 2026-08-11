// Package apisserunner executes hash-bound acceptance cases exclusively
// through harnessd's public HTTP and SSE boundaries.
package apisserunner

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/acceptance/inventory"
)

type Probe func(context.Context, string, string) ([]inventory.ProbeObservation, error)
type Cleanup func(context.Context) (string, error)

// Plan is an intent-specific safe fixture. Its item identity and completeness
// are always validated against the live registry-derived inventory.
type Plan struct {
	Case           inventory.Case
	Prompt         string
	StartFields    map[string]any // additive JSON fields for real run admission
	ContinuePrompt string         // optional ordered same-conversation second message
	Probe          Probe
	Cleanup        Cleanup
}

type Runner struct {
	BaseURL, ArtifactRoot string
	Client                *http.Client
}

// LoadLiveInventory compiles the exact condition-resolved catalog returned by
// harnessd. It refuses an absent resolver observation array, matching the
// inventory command's fail-closed boundary.
func (r Runner) LoadLiveInventory(ctx context.Context) (inventory.Compiled, error) {
	base := strings.TrimRight(strings.TrimSpace(r.BaseURL), "/")
	if base == "" {
		return inventory.Compiled{}, fmt.Errorf("API runner requires base URL")
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/tools", nil)
	if err != nil {
		return inventory.Compiled{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return inventory.Compiled{}, fmt.Errorf("GET /v1/tools: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return inventory.Compiled{}, fmt.Errorf("GET /v1/tools: status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Tools                         []inventory.HTTPTool             `json:"tools"`
		ConfiguredUnavailableToolsets *[]inventory.ConfiguredToolset   `json:"configured_unavailable_toolsets"`
		Unavailable                   *[]inventory.ResolverObservation `json:"unavailable"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return inventory.Compiled{}, fmt.Errorf("decode /v1/tools: %w", err)
	}
	if payload.ConfiguredUnavailableToolsets == nil || payload.Unavailable == nil {
		return inventory.Compiled{}, fmt.Errorf("decode /v1/tools: authoritative resolver evidence is absent or null")
	}
	return inventory.Compile(inventory.InputFromHTTPBoundary(payload.Tools, tui.NewCommandRegistry().All(), *payload.Unavailable, *payload.ConfiguredUnavailableToolsets))
}

// RunLive binds a suite to the current daemon inventory before it can issue a
// run request. A stale caller-supplied hash or an incomplete plan fails closed.
func (r Runner) RunLive(ctx context.Context, plans []Plan) ([]inventory.Evidence, inventory.Compiled, error) {
	compiled, err := r.LoadLiveInventory(ctx)
	if err != nil {
		return nil, inventory.Compiled{}, err
	}
	evidence, err := r.Run(ctx, compiled, plans)
	return evidence, compiled, err
}

func (r Runner) Run(ctx context.Context, compiled inventory.Compiled, plans []Plan) ([]inventory.Evidence, error) {
	if strings.TrimSpace(r.BaseURL) == "" || strings.TrimSpace(r.ArtifactRoot) == "" {
		return nil, fmt.Errorf("API runner requires base URL and artifact root")
	}
	cases := make([]inventory.Case, len(plans))
	for i := range plans {
		cases[i] = plans[i].Case
		if plans[i].Cleanup == nil {
			return nil, fmt.Errorf("%s: case requires cleanup before accepted run can proceed", plans[i].Case.ItemID)
		}
	}
	if err := inventory.ValidateCasesForSurface(compiled, cases, inventory.SurfaceAPI); err != nil {
		return nil, fmt.Errorf("validate registry-derived API cases: %w", err)
	}
	if err := os.MkdirAll(r.ArtifactRoot, 0o700); err != nil {
		return nil, err
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	results := make([]inventory.Evidence, 0, len(plans))
	for _, p := range plans {
		e, err := r.runOne(ctx, client, strings.TrimRight(r.BaseURL, "/"), compiled, p)
		if err != nil {
			return results, fmt.Errorf("%s: %w", p.Case.ItemID, err)
		}
		results = append(results, e)
	}
	return results, nil
}

func (r Runner) runOne(ctx context.Context, client *http.Client, base string, compiled inventory.Compiled, plan Plan) (evidence inventory.Evidence, err error) {
	started := time.Now().UTC()
	payload := make(map[string]any, len(plan.StartFields)+1)
	for key, value := range plan.StartFields {
		if key == "prompt" {
			return inventory.Evidence{}, fmt.Errorf("start fields must not override prompt")
		}
		payload[key] = value
	}
	payload["prompt"] = plan.Prompt
	body, err := json.Marshal(payload)
	if err != nil {
		return inventory.Evidence{}, fmt.Errorf("encode start request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/runs", strings.NewReader(string(body)))
	if err != nil {
		return inventory.Evidence{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return inventory.Evidence{}, fmt.Errorf("start run: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return inventory.Evidence{}, fmt.Errorf("start run status %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	// The daemon has accepted work. Install the cleanup guard before decoding
	// its response so malformed JSON or a missing identity cannot leak the
	// fixture that made the accepted run possible.
	cleanupCompleted := false
	defer func() {
		if cleanupCompleted {
			return
		}
		_, cleanupErr := plan.Cleanup(ctx)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup after accepted run: %w", cleanupErr))
		}
	}()
	var start struct {
		RunID          string `json:"run_id"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&start); err != nil {
		return inventory.Evidence{}, err
	}
	if start.RunID == "" {
		return inventory.Evidence{}, fmt.Errorf("started run omitted run identity")
	}
	raw, ids, err := stream(ctx, client, base, start.RunID)
	if err != nil {
		return inventory.Evidence{}, err
	}
	statusRaw, status, conversation, err := terminal(ctx, client, base, start.RunID)
	if err != nil {
		return inventory.Evidence{}, err
	}
	if status != "completed" {
		return inventory.Evidence{}, fmt.Errorf("terminal status = %q", status)
	}
	if start.ConversationID != "" && conversation != "" && conversation != start.ConversationID {
		return inventory.Evidence{}, fmt.Errorf("terminal conversation identity differs from start")
	}
	if start.ConversationID == "" {
		start.ConversationID = conversation
	}
	if start.ConversationID == "" {
		return inventory.Evidence{}, fmt.Errorf("run and terminal status omitted conversation identity")
	}
	if plan.ContinuePrompt != "" {
		continued, err := continueRun(ctx, client, base, start.RunID, plan.ContinuePrompt)
		if err != nil {
			return inventory.Evidence{}, err
		}
		continuedRaw, continuedIDs, err := stream(ctx, client, base, continued)
		if err != nil {
			return inventory.Evidence{}, err
		}
		continuedStatusRaw, continuedStatus, continuedConversation, err := terminal(ctx, client, base, continued)
		if err != nil {
			return inventory.Evidence{}, err
		}
		if continuedStatus != "completed" {
			return inventory.Evidence{}, fmt.Errorf("continued terminal status = %q", continuedStatus)
		}
		if continuedConversation != "" && continuedConversation != start.ConversationID {
			return inventory.Evidence{}, fmt.Errorf("continued conversation identity differs from source")
		}
		raw = append(append(raw, '\n'), continuedRaw...)
		ids = append(ids, continuedIDs...)
		start.RunID = continued
		statusRaw = continuedStatusRaw
	}
	rawArtifact, err := r.artifact(plan.Case.ItemID, "raw-sse.txt", inventory.ArtifactRawSSEEvent, raw)
	if err != nil {
		return inventory.Evidence{}, err
	}
	statusArtifact, err := r.artifact(plan.Case.ItemID, "terminal.json", inventory.ArtifactAPIStoreProbe, statusRaw)
	if err != nil {
		return inventory.Evidence{}, err
	}
	if plan.Probe == nil {
		return inventory.Evidence{}, fmt.Errorf("case requires independent probe")
	}
	observed, err := plan.Probe(ctx, base, start.RunID)
	if err != nil {
		return inventory.Evidence{}, fmt.Errorf("independent probe: %w", err)
	}
	cleanupCompleted = true
	cleanup, err := plan.Cleanup(ctx)
	if err != nil {
		return inventory.Evidence{}, fmt.Errorf("cleanup: %w", err)
	}
	evidence = inventory.Evidence{SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, ItemID: plan.Case.ItemID, InvocationID: plan.Case.InvocationID, Surface: inventory.SurfaceAPI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass, OrderedActions: plan.Case.OrderedActions, RunID: start.RunID, ConversationID: start.ConversationID, EventIDs: ids, ExpectedPostconditions: plan.Case.ExpectedPostconditions, ObservedPostconditions: observed, Artifacts: []inventory.ArtifactRef{rawArtifact, statusArtifact}, Cleanup: inventory.CleanupEvidence{Verified: true, Detail: cleanup}, Timing: inventory.Timing{StartedAt: started, FinishedAt: time.Now().UTC()}}
	if err := inventory.ValidateEvidence(compiled, plan.Case, evidence); err != nil {
		return inventory.Evidence{}, fmt.Errorf("validate evidence: %w", err)
	}
	return evidence, nil
}

func stream(ctx context.Context, client *http.Client, base, runID string) ([]byte, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs/"+runID+"/events", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return nil, nil, fmt.Errorf("SSE status/content-type = %s/%q", resp.Status, resp.Header.Get("Content-Type"))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	var ids []string
	for scanner.Scan() {
		if id, ok := strings.CutPrefix(scanner.Text(), "id: "); ok && strings.TrimSpace(id) != "" {
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return nil, nil, fmt.Errorf("SSE contained no event identities")
	}
	return raw, ids, nil
}

func terminal(ctx context.Context, client *http.Client, base, runID string) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs/"+runID, nil)
	if err != nil {
		return nil, "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("terminal status = %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}
	var value struct {
		Status         string `json:"status"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, "", "", err
	}
	return raw, value.Status, value.ConversationID, nil
}

func continueRun(ctx context.Context, client *http.Client, base, runID, prompt string) (string, error) {
	body, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/runs/"+runID+"/continue", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("continue status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var result struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.RunID == "" {
		return "", fmt.Errorf("continue omitted run identity")
	}
	return result.RunID, nil
}

func (r Runner) artifact(itemID, suffix string, kind inventory.ArtifactKind, data []byte) (inventory.ArtifactRef, error) {
	name := strings.NewReplacer(":", "_", "/", "_").Replace(itemID) + "-" + suffix
	if err := os.WriteFile(filepath.Join(r.ArtifactRoot, name), data, 0o600); err != nil {
		return inventory.ArtifactRef{}, err
	}
	sum := sha256.Sum256(data)
	redacted := true
	return inventory.ArtifactRef{Kind: kind, Path: name, Digest: "sha256:" + hex.EncodeToString(sum[:]), Redacted: &redacted}, nil
}
