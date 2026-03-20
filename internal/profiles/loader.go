package profiles

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/config"
)

//go:embed builtins/*.toml
var builtinFS embed.FS

// LoadProfile loads a named profile using the three-tier resolution:
//  1. Project-level:  <projectProfilesDir>/<name>.toml
//  2. User-global:    <userProfilesDir>/<name>.toml
//  3. Built-in:       embedded in binary
//
// Pass empty strings for dirs you want to skip.
// Returns ErrProfileNotFound if the profile cannot be resolved from any tier.
func LoadProfile(name string) (*Profile, error) {
	return loadProfileWithDirs(name, defaultProjectProfilesDir(), defaultUserProfilesDir())
}

// LoadProfileFromUserDir loads a profile using an explicit user profiles directory.
// Falls back to built-ins if not found in userDir.
func LoadProfileFromUserDir(name, userDir string) (*Profile, error) {
	return loadProfileWithDirs(name, "", userDir)
}

// LoadProfileWithDirs loads a profile using explicit project and user profile
// directories, then falls back to embedded built-ins.
func LoadProfileWithDirs(name, projectDir, userDir string) (*Profile, error) {
	return loadProfileWithDirs(name, projectDir, userDir)
}

// loadProfileWithDirs is the internal implementation that accepts explicit dirs for testing.
func loadProfileWithDirs(name, projectDir, userDir string) (*Profile, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return nil, err
	}

	// Tier 1: project-level.
	if projectDir != "" {
		p, err := loadProfileFile(filepath.Join(projectDir, name+".toml"))
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("project profile %q: %w", name, err)
		}
		if p != nil {
			return p, nil
		}
	}

	// Tier 2: user-global.
	if userDir != "" {
		p, err := loadProfileFile(filepath.Join(userDir, name+".toml"))
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("user profile %q: %w", name, err)
		}
		if p != nil {
			return p, nil
		}
	}

	// Tier 3: built-in embedded profiles.
	p, err := loadBuiltinProfile(name)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}

	return nil, fmt.Errorf("profile %q not found", name)
}

// ListProfiles returns the names of all available profiles across all three tiers.
// Duplicates (same name in multiple tiers) are deduplicated; project-level wins.
func ListProfiles() ([]string, error) {
	return listProfilesWithDirs(defaultProjectProfilesDir(), defaultUserProfilesDir())
}

// listProfilesWithDirs is the internal implementation for testing.
func listProfilesWithDirs(projectDir, userDir string) ([]string, error) {
	seen := make(map[string]bool)
	var names []string

	addDir := func(dir string) error {
		if dir == "" {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".toml") {
				continue
			}
			n := strings.TrimSuffix(e.Name(), ".toml")
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
		return nil
	}

	if err := addDir(projectDir); err != nil {
		return nil, err
	}
	if err := addDir(userDir); err != nil {
		return nil, err
	}

	// Add built-ins not already seen.
	builtins, err := listBuiltinNames()
	if err != nil {
		return nil, err
	}
	for _, n := range builtins {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	return names, nil
}

// SaveProfile writes a profile to the user-global profiles directory.
// Creates the directory if it does not exist.
func SaveProfile(p *Profile) error {
	dir := defaultUserProfilesDir()
	if dir == "" {
		return fmt.Errorf("cannot determine user home directory")
	}
	return saveProfileToDir(p, dir)
}

// saveProfileToDir writes a profile TOML to the given directory.
// Creates the directory if it does not exist.
func saveProfileToDir(p *Profile, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create profiles dir: %w", err)
	}
	if err := config.ValidateProfileName(p.Meta.Name); err != nil {
		return err
	}
	path := filepath.Join(dir, p.Meta.Name+".toml")
	// Write atomically: write to temp file, then rename.
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp profile file: %w", err)
	}
	if err := toml.NewEncoder(f).Encode(p); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode profile: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// loadProfileFile reads and parses a single TOML profile file.
// Returns (nil, nil) when the file does not exist.
func loadProfileFile(path string) (*Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var p Profile
	if _, err := toml.NewDecoder(f).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// loadBuiltinProfile loads a named built-in profile from the embedded FS.
func loadBuiltinProfile(name string) (*Profile, error) {
	data, err := builtinFS.ReadFile("builtins/" + name + ".toml")
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read built-in profile %q: %w", name, err)
	}
	var p Profile
	if _, err := toml.Decode(string(data), &p); err != nil {
		return nil, fmt.Errorf("parse built-in profile %q: %w", name, err)
	}
	return &p, nil
}

// listBuiltinNames returns the names of all embedded built-in profiles.
func listBuiltinNames() ([]string, error) {
	entries, err := fs.ReadDir(builtinFS, "builtins")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
		}
	}
	return names, nil
}

// isNotExist checks if an embed.FS error is "file not found".
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "file does not exist") ||
		strings.Contains(err.Error(), "open builtins/") ||
		os.IsNotExist(err)
}

// defaultUserProfilesDir returns ~/.harness/profiles/.
func defaultUserProfilesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harness", "profiles")
}

// defaultProjectProfilesDir returns .harness/profiles/ relative to cwd.
func defaultProjectProfilesDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".harness", "profiles")
}
