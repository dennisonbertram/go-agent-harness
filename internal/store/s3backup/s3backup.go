// Package s3backup provides JSONL event backup streaming to S3 on run completion.
//
// When configured (via environment variables), the uploader fetches all events
// for a completed run from the store and PUTs a single JSONL file to S3 at:
//
//	{prefix}/{conversation_id}/{run_id}.jsonl
//
// Each line of the JSONL file is a JSON object. The first line is a "run"
// header record containing the run metadata; subsequent lines are event records
// in ascending sequence order.
//
// Configuration is read from environment variables:
//
//	AWS_ACCESS_KEY_ID      — required
//	AWS_SECRET_ACCESS_KEY  — required
//	AWS_REGION             — required
//	S3_BUCKET              — required
//	S3_KEY_PREFIX          — optional (default: empty)
//
// If any required variable is absent, ConfigFromEnv returns ok=false and the
// caller should use NewNoOpUploader() so the harness continues without backup.
package s3backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-agent-harness/internal/store"
)

// Config holds the S3 backup configuration.
type Config struct {
	// Bucket is the target S3 bucket name. Required.
	Bucket string
	// KeyPrefix is the optional path prefix prepended to every object key.
	// If empty, keys are formatted as "{conversation_id}/{run_id}.jsonl".
	KeyPrefix string
	// Region is the AWS region (e.g. "us-east-1"). Required.
	Region string
	// AccessKeyID is the AWS access key. Required.
	AccessKeyID string
	// SecretAccessKey is the AWS secret key. Required.
	SecretAccessKey string
	// EndpointURL overrides the default AWS S3 endpoint. Used in tests to
	// point at a local httptest.Server instead of real AWS. Empty means
	// use the standard regional endpoint.
	EndpointURL string
}

// ObjectKey returns the S3 object key for the given conversation and run IDs.
// Format: "{prefix}/{conversation_id}/{run_id}.jsonl" when prefix is set,
// or "{conversation_id}/{run_id}.jsonl" when prefix is empty.
func (c Config) ObjectKey(conversationID, runID string) string {
	if c.KeyPrefix != "" {
		return c.KeyPrefix + "/" + conversationID + "/" + runID + ".jsonl"
	}
	return conversationID + "/" + runID + ".jsonl"
}

// ConfigFromEnv reads S3 configuration from environment variables.
// Returns (cfg, true) when all required variables are present.
// Returns (Config{}, false) when any required variable is absent — the
// caller should then use a no-op uploader and skip backup silently.
func ConfigFromEnv(getenv func(string) string) (Config, bool) {
	accessKey := strings.TrimSpace(getenv("AWS_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(getenv("AWS_SECRET_ACCESS_KEY"))
	region := strings.TrimSpace(getenv("AWS_REGION"))
	bucket := strings.TrimSpace(getenv("S3_BUCKET"))
	prefix := strings.TrimSpace(getenv("S3_KEY_PREFIX"))

	if accessKey == "" || secretKey == "" || region == "" || bucket == "" {
		return Config{}, false
	}
	return Config{
		Bucket:          bucket,
		KeyPrefix:       prefix,
		Region:          region,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
	}, true
}

// RunUploader is the interface for uploading a run's JSONL backup to S3.
// Both Uploader and NoOpUploader implement this interface.
type RunUploader interface {
	// UploadRun fetches run events from st and uploads them as JSONL to S3.
	// It is safe to call concurrently from multiple goroutines.
	// If S3 config is absent, the no-op implementation returns nil.
	UploadRun(ctx context.Context, st store.Store, conversationID, runID string) error
}

// --- NoOpUploader ---

// NoOpUploader is a RunUploader that does nothing. It is used when S3 config
// is absent so the harness continues without backup (no error, no log noise).
type NoOpUploader struct{}

// NewNoOpUploader returns a no-op uploader that silently ignores all calls.
func NewNoOpUploader() *NoOpUploader { return &NoOpUploader{} }

// UploadRun is a no-op; always returns nil.
func (*NoOpUploader) UploadRun(_ context.Context, _ store.Store, _, _ string) error { return nil }

// --- Uploader ---

// Uploader uploads run events as JSONL to S3 using AWS Signature Version 4.
// It is safe for concurrent use.
type Uploader struct {
	cfg    Config
	client *http.Client
}

// NewUploader creates an Uploader with the given config.
func NewUploader(cfg Config) *Uploader {
	return &Uploader{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadRun builds a JSONL payload from the store and PUTs it to S3.
// The S3 object key is: {prefix}/{conversationID}/{runID}.jsonl
func (u *Uploader) UploadRun(ctx context.Context, st store.Store, conversationID, runID string) error {
	body, err := BuildJSONL(ctx, st, runID)
	if err != nil {
		return fmt.Errorf("s3backup: build JSONL for run %s: %w", runID, err)
	}

	key := u.cfg.ObjectKey(conversationID, runID)
	if err := u.put(ctx, key, body); err != nil {
		return fmt.Errorf("s3backup: PUT s3://%s/%s: %w", u.cfg.Bucket, key, err)
	}
	return nil
}

// put performs an S3 PutObject using AWS Signature Version 4.
func (u *Uploader) put(ctx context.Context, key string, body []byte) error {
	const contentType = "application/x-ndjson"

	endpoint := u.s3Endpoint()
	url := endpoint + "/" + u.cfg.Bucket + "/" + key

	now := time.Now().UTC()
	payloadHash := hashSHA256(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-date", now.Format("20060102T150405Z"))
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.ContentLength = int64(len(body))

	// Build and attach Authorization header.
	authHeader := u.sigV4AuthHeader(req, now, payloadHash, body)
	req.Header.Set("Authorization", authHeader)

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain to allow connection reuse

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected S3 status %d", resp.StatusCode)
	}
	return nil
}

// s3Endpoint returns the S3 endpoint URL to use.
// When EndpointURL is set (typically in tests), it is returned verbatim.
// Otherwise, the standard regional endpoint is returned.
func (u *Uploader) s3Endpoint() string {
	if u.cfg.EndpointURL != "" {
		return strings.TrimRight(u.cfg.EndpointURL, "/")
	}
	return "https://s3." + u.cfg.Region + ".amazonaws.com"
}

// sigV4AuthHeader computes the AWS Signature Version 4 Authorization header
// for a PutObject request. This is a minimal implementation that covers the
// single-chunk upload path required for JSONL backup.
func (u *Uploader) sigV4AuthHeader(req *http.Request, now time.Time, payloadHash string, _ []byte) string {
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	// Canonical headers (must be lowercase, sorted, and include host).
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}

	canonicalHeaders := "content-type:" + req.Header.Get("Content-Type") + "\n" +
		"host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	// Canonical request.
	canonicalURI := req.URL.Path
	canonicalQueryString := ""
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// String to sign.
	credentialScope := dateStamp + "/" + u.cfg.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" +
		amzDate + "\n" +
		credentialScope + "\n" +
		hashSHA256([]byte(canonicalRequest))

	// Signing key.
	signingKey := deriveSigningKey(u.cfg.SecretAccessKey, dateStamp, u.cfg.Region, "s3")

	// Signature.
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	return fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		u.cfg.AccessKeyID, credentialScope, signedHeaders, signature,
	)
}

// --- JSONL builder ---

// runRecord is the first JSONL line in a backup file.
// It contains the full run metadata for quick lookups without scanning events.
type runRecord struct {
	Type           string    `json:"type"`
	RunID          string    `json:"run_id"`
	ConversationID string    `json:"conversation_id"`
	TenantID       string    `json:"tenant_id"`
	AgentID        string    `json:"agent_id"`
	Model          string    `json:"model"`
	ProviderName   string    `json:"provider_name,omitempty"`
	Status         string    `json:"status"`
	Output         string    `json:"output,omitempty"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// eventRecord is one JSONL line per run event.
type eventRecord struct {
	Type      string         `json:"type"`
	RunID     string         `json:"run_id"`
	Seq       int            `json:"seq"`
	EventID   string         `json:"event_id"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// BuildJSONL fetches the run and its events from st and returns a JSONL byte
// slice. The first line is a run header record; subsequent lines are event
// records in ascending seq order.
//
// Returns an error if the run is not found or if store reads fail.
func BuildJSONL(ctx context.Context, st store.Store, runID string) ([]byte, error) {
	run, err := st.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}

	events, err := st.GetEvents(ctx, runID, -1)
	if err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	// First line: run header.
	hdr := runRecord{
		Type:           "run",
		RunID:          run.ID,
		ConversationID: run.ConversationID,
		TenantID:       run.TenantID,
		AgentID:        run.AgentID,
		Model:          run.Model,
		ProviderName:   run.ProviderName,
		Status:         string(run.Status),
		Output:         run.Output,
		Error:          run.Error,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
	}
	if err := enc.Encode(hdr); err != nil {
		return nil, fmt.Errorf("encode run header: %w", err)
	}

	// Subsequent lines: events in seq order.
	for _, e := range events {
		var payload map[string]any
		if e.Payload != "" {
			if jerr := json.Unmarshal([]byte(e.Payload), &payload); jerr != nil {
				// If payload is not valid JSON, store it as a raw string field.
				payload = map[string]any{"raw": e.Payload}
			}
		}
		rec := eventRecord{
			Type:      "event",
			RunID:     e.RunID,
			Seq:       e.Seq,
			EventID:   e.EventID,
			EventType: e.EventType,
			Payload:   payload,
			Timestamp: e.Timestamp,
		}
		if err := enc.Encode(rec); err != nil {
			return nil, fmt.Errorf("encode event seq=%d: %w", e.Seq, err)
		}
	}

	return buf.Bytes(), nil
}

// --- crypto helpers ---

func hashSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
