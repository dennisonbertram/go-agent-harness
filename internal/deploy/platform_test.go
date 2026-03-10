package deploy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestDeployOptsDefaults verifies zero-value DeployOpts has expected defaults.
func TestDeployOptsDefaults(t *testing.T) {
	opts := DeployOpts{}
	if opts.DryRun {
		t.Error("DryRun should default to false")
	}
	if opts.Force {
		t.Error("Force should default to false")
	}
	if opts.Environment != "" {
		t.Error("Environment should default to empty")
	}
}

// TestDeployResultSerialization verifies JSON round-trip for DeployResult.
func TestDeployResultSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	r := DeployResult{
		URL:       "https://example.railway.app",
		Version:   "v42",
		Platform:  "railway",
		Timestamp: now,
		Logs:      "Deployment live",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 DeployResult
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r2.URL != r.URL {
		t.Errorf("URL: got %q, want %q", r2.URL, r.URL)
	}
	if r2.Platform != r.Platform {
		t.Errorf("Platform: got %q, want %q", r2.Platform, r.Platform)
	}
	if r2.Version != r.Version {
		t.Errorf("Version: got %q, want %q", r2.Version, r.Version)
	}
}

// TestDeployStatusStates verifies all valid state strings are accepted.
func TestDeployStatusStates(t *testing.T) {
	states := []string{"running", "building", "failed", "sleeping", "unknown"}
	for _, state := range states {
		s := DeployStatus{State: state}
		if s.State != state {
			t.Errorf("state %q not preserved", state)
		}
	}
}

// TestDeployStatusSerialization verifies JSON round-trip for DeployStatus.
func TestDeployStatusSerialization(t *testing.T) {
	s := DeployStatus{
		State:     "running",
		URL:       "https://myapp.fly.dev",
		Version:   "v3",
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var s2 DeployStatus
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s2.State != s.State {
		t.Errorf("State: got %q, want %q", s2.State, s.State)
	}
	if s2.URL != s.URL {
		t.Errorf("URL: got %q, want %q", s2.URL, s.URL)
	}
}

// TestErrNotImplemented verifies the sentinel error message.
func TestErrNotImplemented(t *testing.T) {
	err := ErrNotImplemented
	if err == nil {
		t.Fatal("ErrNotImplemented should not be nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("error message should contain 'not implemented', got %q", err.Error())
	}
}

// TestExtractURL verifies URL extraction from deployment output.
func TestExtractURL(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"Deployment live at https://myapp.railway.app success", "https://myapp.railway.app"},
		{"no url here", ""},
		{"visit http://example.com. for docs", "http://example.com"},
		{"https://a.fly.dev,extra", "https://a.fly.dev"},
		{"", ""},
	}
	for _, tc := range tests {
		got := extractURL(tc.text)
		if got != tc.want {
			t.Errorf("extractURL(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
