package deploy

import (
	"fmt"
	"os"
	"path/filepath"
)

// platformConfig maps platform names to their config file indicators.
// Order defines priority when multiple configs exist.
var platformConfig = []struct {
	name  string
	files []string
}{
	{"cloudflare", []string{"wrangler.toml", "wrangler.jsonc", "wrangler.json"}},
	{"vercel", []string{"vercel.json", ".vercel"}},
	{"flyio", []string{"fly.toml"}},
	{"railway", []string{"railway.json", "railway.toml"}},
	{"kamal", []string{"config/deploy.yml"}},
}

// DetectPlatform scans the workspace directory for platform config files and
// returns the name of the first matching platform in priority order.
// Returns an error if no platform is detected.
func DetectPlatform(workspaceDir string) (string, error) {
	for _, pc := range platformConfig {
		for _, file := range pc.files {
			path := filepath.Join(workspaceDir, file)
			if _, err := os.Stat(path); err == nil {
				return pc.name, nil
			}
		}
	}
	// Check for Docker fallback.
	if _, err := os.Stat(filepath.Join(workspaceDir, "Dockerfile")); err == nil {
		return "docker", nil
	}
	return "", fmt.Errorf("no platform config found in %s", workspaceDir)
}

// DetectAll returns all platform names detected in the workspace directory,
// in priority order. Returns an empty slice if none are found.
func DetectAll(workspaceDir string) []string {
	var found []string
	for _, pc := range platformConfig {
		for _, file := range pc.files {
			path := filepath.Join(workspaceDir, file)
			if _, err := os.Stat(path); err == nil {
				found = append(found, pc.name)
				break // only add the platform once
			}
		}
	}
	return found
}
