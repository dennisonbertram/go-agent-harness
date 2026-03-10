package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file at path/name with empty contents.
func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestDetectCloudflare verifies wrangler.toml → cloudflare.
func TestDetectCloudflare(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "wrangler.toml")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cloudflare" {
		t.Errorf("expected 'cloudflare', got %q", got)
	}
}

// TestDetectCloudflareJsonc verifies wrangler.jsonc is also detected.
func TestDetectCloudflareJsonc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "wrangler.jsonc")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cloudflare" {
		t.Errorf("expected 'cloudflare', got %q", got)
	}
}

// TestDetectVercel verifies vercel.json → vercel.
func TestDetectVercel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "vercel.json")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "vercel" {
		t.Errorf("expected 'vercel', got %q", got)
	}
}

// TestDetectFlyio verifies fly.toml → flyio.
func TestDetectFlyio(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "fly.toml")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "flyio" {
		t.Errorf("expected 'flyio', got %q", got)
	}
}

// TestDetectRailway verifies railway.json → railway.
func TestDetectRailway(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "railway.json")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "railway" {
		t.Errorf("expected 'railway', got %q", got)
	}
}

// TestDetectRailwayToml verifies railway.toml is also detected.
func TestDetectRailwayToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "railway.toml")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "railway" {
		t.Errorf("expected 'railway', got %q", got)
	}
}

// TestDetectDockerFallback verifies Dockerfile → docker when no other config present.
func TestDetectDockerFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile")
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "docker" {
		t.Errorf("expected 'docker', got %q", got)
	}
}

// TestDetectNone verifies an error is returned when no config is found.
func TestDetectNone(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectPlatform(dir)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

// TestDetectMultiple verifies priority order when multiple configs exist.
// Cloudflare should win over Vercel, which wins over Fly.io, etc.
func TestDetectMultiple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "wrangler.toml") // cloudflare (priority 1)
	writeFile(t, dir, "vercel.json")   // vercel (priority 2)
	writeFile(t, dir, "fly.toml")      // flyio (priority 3)
	got, err := DetectPlatform(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cloudflare" {
		t.Errorf("expected 'cloudflare' (highest priority), got %q", got)
	}
}

// TestDetectAll returns all detected platforms.
func TestDetectAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "fly.toml")
	writeFile(t, dir, "railway.json")
	all := DetectAll(dir)
	if len(all) != 2 {
		t.Fatalf("expected 2 platforms, got %d: %v", len(all), all)
	}
	// flyio should come before railway in priority order.
	if all[0] != "flyio" {
		t.Errorf("expected first to be 'flyio', got %q", all[0])
	}
	if all[1] != "railway" {
		t.Errorf("expected second to be 'railway', got %q", all[1])
	}
}

// TestDetectAllEmpty returns empty slice when no configs present.
func TestDetectAllEmpty(t *testing.T) {
	dir := t.TempDir()
	all := DetectAll(dir)
	if len(all) != 0 {
		t.Errorf("expected empty slice, got %v", all)
	}
}

// TestConcurrentDetection verifies detection is safe under concurrent access.
func TestConcurrentDetection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "fly.toml")
	const goroutines = 20
	done := make(chan string, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			name, err := DetectPlatform(dir)
			if err != nil {
				done <- "error:" + err.Error()
				return
			}
			done <- name
		}()
	}
	for i := 0; i < goroutines; i++ {
		got := <-done
		if got != "flyio" {
			t.Errorf("concurrent detect: got %q, want 'flyio'", got)
		}
	}
}
