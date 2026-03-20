package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-agent-harness/internal/config"
)

// ValidateProfile checks a Profile for required fields and valid values.
// It does NOT write any files.
func ValidateProfile(p *Profile) error {
	if strings.TrimSpace(p.Meta.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	if err := config.ValidateProfileName(p.Meta.Name); err != nil {
		return err
	}
	if strings.TrimSpace(p.Meta.Description) == "" {
		return fmt.Errorf("profile description is required")
	}
	return nil
}

// SaveProfileToDir writes a profile TOML to the given directory atomically.
// Exported for use by tools in the deferred package.
func SaveProfileToDir(p *Profile, dir string) error {
	return saveProfileToDir(p, dir)
}

// IsBuiltinProfile reports whether name corresponds to an embedded built-in profile.
func IsBuiltinProfile(name string) bool {
	names, err := listBuiltinNames()
	if err != nil {
		return false
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// DeleteProfile removes a user profile from the default user profiles directory.
// Returns an error if the profile is a built-in (embedded) profile or if the
// file does not exist in the user directory.
func DeleteProfile(name string) error {
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}
	dir := defaultUserProfilesDir()
	if dir == "" {
		return fmt.Errorf("cannot determine user home directory")
	}
	return DeleteProfileFromDir(name, dir)
}

// DeleteProfileFromDir removes a profile TOML from the given directory.
// It refuses to delete if the name only exists as a built-in (no user file present).
// Exported for use by tools in the deferred package.
func DeleteProfileFromDir(name, dir string) error {
	return deleteProfileFromDir(name, dir)
}

// deleteProfileFromDir is the internal implementation.
func deleteProfileFromDir(name, dir string) error {
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".toml")

	// Check if the user file exists.
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// No user file. If it's a built-in, return a helpful error.
			if IsBuiltinProfile(name) {
				return fmt.Errorf("profile %q is a built-in profile and cannot be deleted", name)
			}
			return fmt.Errorf("profile %q not found", name)
		}
		return fmt.Errorf("stat profile %q: %w", name, err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete profile %q: %w", name, err)
	}
	return nil
}
