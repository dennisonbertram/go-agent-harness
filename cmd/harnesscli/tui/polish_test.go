package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPolishIsTooSmall(t *testing.T) {
	cases := []struct {
		width, height int
		want          bool
	}{
		{0, 0, true},
		{39, 10, true},
		{40, 10, false},
		{40, 9, true},
		{80, 24, false},
		{200, 50, false},
	}
	for _, tc := range cases {
		got := IsTooSmall(tc.width, tc.height)
		if got != tc.want {
			t.Errorf("IsTooSmall(%d, %d) = %v, want %v", tc.width, tc.height, got, tc.want)
		}
	}
}

func TestPolishTooSmallView(t *testing.T) {
	// Truncation: width=5 should return first 5 chars of the message.
	msg := TooSmallView(5, 5)
	if len(msg) != 5 {
		t.Errorf("TooSmallView(5,5) length = %d, want 5", len(msg))
	}
	expected5 := "Termi"
	if msg != expected5 {
		t.Errorf("TooSmallView(5,5) = %q, want %q", msg, expected5)
	}

	// Normal: width=80 should return full message.
	full := "Terminal too small. Please resize."
	msg80 := TooSmallView(80, 24)
	if msg80 != full {
		t.Errorf("TooSmallView(80,24) = %q, want %q", msg80, full)
	}
}

func TestPolishReducedMotion(t *testing.T) {
	// Neither env var set — should be false.
	t.Setenv("NO_MOTION", "")
	t.Setenv("PREFERS_REDUCED_MOTION", "")
	if ReducedMotion() {
		t.Error("ReducedMotion() = true with no env vars set, want false")
	}

	// NO_MOTION set to non-empty — should be true.
	t.Setenv("NO_MOTION", "1")
	if !ReducedMotion() {
		t.Error("ReducedMotion() = false with NO_MOTION=1, want true")
	}
	t.Setenv("NO_MOTION", "")

	// PREFERS_REDUCED_MOTION=1 — should be true.
	t.Setenv("PREFERS_REDUCED_MOTION", "1")
	if !ReducedMotion() {
		t.Error("ReducedMotion() = false with PREFERS_REDUCED_MOTION=1, want true")
	}
	t.Setenv("PREFERS_REDUCED_MOTION", "")

	// PREFERS_REDUCED_MOTION set to something other than "1" — should be false.
	t.Setenv("PREFERS_REDUCED_MOTION", "0")
	if ReducedMotion() {
		t.Error("ReducedMotion() = true with PREFERS_REDUCED_MOTION=0, want false")
	}
}

func TestPolishSpinnerFrames(t *testing.T) {
	// Full animation: 6 frames.
	frames := SpinnerFrames(false)
	if len(frames) != 6 {
		t.Errorf("SpinnerFrames(false) len = %d, want 6", len(frames))
	}

	// Reduced motion: single "…" frame.
	reduced := SpinnerFrames(true)
	if len(reduced) != 1 {
		t.Errorf("SpinnerFrames(true) len = %d, want 1", len(reduced))
	}
	if reduced[0] != "…" {
		t.Errorf("SpinnerFrames(true)[0] = %q, want %q", reduced[0], "…")
	}
}

func TestDefaultExportDir_NonEmpty(t *testing.T) {
	dir := defaultExportDir()
	if dir == "" {
		t.Fatal("defaultExportDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("defaultExportDir() returned relative path: %q", dir)
	}
	// Must end in harness/transcripts
	if !strings.HasSuffix(filepath.ToSlash(dir), "harness/transcripts") {
		t.Errorf("defaultExportDir() should end with harness/transcripts, got: %q", dir)
	}
}

func TestDefaultExportDir_NotRepoRoot(t *testing.T) {
	// The export dir must not be "." or the current working directory directly.
	dir := defaultExportDir()
	if dir == "." {
		t.Errorf("defaultExportDir() must not return '.': %q", dir)
	}
}

func TestPolishClampWidth(t *testing.T) {
	th := DefaultTheme()

	cases := []struct {
		w, min, max, want int
	}{
		{50, 0, 100, 50},   // within range
		{-5, 0, 100, 0},    // below min
		{150, 0, 100, 100}, // above max
		{0, 0, 100, 0},     // exactly min
		{100, 0, 100, 100}, // exactly max
		{10, 10, 10, 10},   // min == max == w
		{5, 10, 10, 10},    // below min where min==max
		{15, 10, 10, 10},   // above max where min==max
	}
	for _, tc := range cases {
		got := th.ClampWidth(tc.w, tc.min, tc.max)
		if got != tc.want {
			t.Errorf("ClampWidth(%d, %d, %d) = %d, want %d", tc.w, tc.min, tc.max, got, tc.want)
		}
	}
}

// TestAPIKeyCursor_DefaultZero verifies the initial cursor position is zero.
func TestAPIKeyCursor_DefaultZero(t *testing.T) {
	m := New(DefaultTUIConfig())
	if got := m.APIKeyCursor(); got != 0 {
		t.Errorf("APIKeyCursor() = %d, want 0", got)
	}
}

// TestProviderIndexInAPIKeyList_FoundAndNotFound exercises the search helper.
func TestProviderIndexInAPIKeyList_FoundAndNotFound(t *testing.T) {
	m := New(DefaultTUIConfig())
	m.apiKeyProviders = []apiKeyProvider{
		{Name: "openai"},
		{Name: "anthropic"},
	}

	if idx := m.providerIndexInAPIKeyList("anthropic"); idx != 1 {
		t.Errorf("providerIndexInAPIKeyList(anthropic) = %d, want 1", idx)
	}
	if idx := m.providerIndexInAPIKeyList("missing"); idx != -1 {
		t.Errorf("providerIndexInAPIKeyList(missing) = %d, want -1", idx)
	}
}
