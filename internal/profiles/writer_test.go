package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateProfile_ValidProfile verifies that a complete, valid profile passes.
func TestValidateProfile_ValidProfile(t *testing.T) {
	p := &Profile{
		Meta: ProfileMeta{
			Name:        "my-profile",
			Description: "A test profile",
			Version:     1,
			CreatedBy:   "user",
		},
		Runner: ProfileRunner{
			Model:    "gpt-4.1-mini",
			MaxSteps: 10,
		},
	}
	err := ValidateProfile(p)
	require.NoError(t, err)
}

// TestValidateProfile_MissingName verifies that a missing name returns an error.
func TestValidateProfile_MissingName(t *testing.T) {
	p := &Profile{
		Meta: ProfileMeta{
			Description: "No name profile",
		},
	}
	err := ValidateProfile(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// TestValidateProfile_InvalidName verifies path-traversal names are rejected.
func TestValidateProfile_InvalidName(t *testing.T) {
	p := &Profile{
		Meta: ProfileMeta{
			Name:        "../evil",
			Description: "traversal attempt",
		},
	}
	err := ValidateProfile(p)
	require.Error(t, err)
}

// TestValidateProfile_MissingDescription verifies that a missing description returns an error.
func TestValidateProfile_MissingDescription(t *testing.T) {
	p := &Profile{
		Meta: ProfileMeta{
			Name: "no-desc",
		},
	}
	err := ValidateProfile(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

// TestDeleteProfile_DeletesUserProfile verifies that a user profile can be deleted.
func TestDeleteProfile_DeletesUserProfile(t *testing.T) {
	dir := t.TempDir()

	// Write a user profile to delete.
	p := &Profile{
		Meta: ProfileMeta{
			Name:        "to-delete",
			Description: "A profile to delete",
			Version:     1,
			CreatedBy:   "user",
		},
		Runner: ProfileRunner{Model: "gpt-4.1-mini", MaxSteps: 5},
	}
	require.NoError(t, saveProfileToDir(p, dir))

	path := filepath.Join(dir, "to-delete.toml")
	_, err := os.Stat(path)
	require.NoError(t, err, "profile file should exist before deletion")

	// Delete using the explicit-dir internal function.
	require.NoError(t, deleteProfileFromDir("to-delete", dir))

	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err), "profile file should be gone after deletion")
}

// TestDeleteProfile_NotFound verifies that deleting a nonexistent profile returns an error.
func TestDeleteProfile_NotFound(t *testing.T) {
	dir := t.TempDir()
	err := deleteProfileFromDir("nonexistent-profile-xyz", dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestDeleteProfile_ProtectsBuiltinProfile verifies built-in profile names
// are rejected when attempting to delete from the builtin-only path.
// (If a user dir contains a profile with the same name as a builtin, that IS deletable —
// the restriction applies to the embedded built-ins themselves.)
func TestDeleteProfile_ProtectsBuiltinProfile(t *testing.T) {
	// Attempt to delete a built-in by name using the exported function
	// (which uses defaultUserProfilesDir — in a fresh temp HOME it won't exist).
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// The built-in "github" lives only in the embedded FS; there is no user file.
	// DeleteProfile must refuse if the profile only exists as a built-in.
	err := DeleteProfile("github")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
}

// TestDeleteProfileFromDir_ExportedPath verifies the exported DeleteProfile
// can delete a user-created profile in the default user dir.
func TestDeleteProfileFromDir_ExportedPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a user profile to the default user dir.
	p := &Profile{
		Meta: ProfileMeta{
			Name:        "user-deletable",
			Description: "A deletable profile",
			Version:     1,
			CreatedBy:   "user",
		},
		Runner: ProfileRunner{Model: "gpt-4.1-mini", MaxSteps: 3},
	}
	require.NoError(t, SaveProfile(p))

	// Now delete it.
	require.NoError(t, DeleteProfile("user-deletable"))

	// Confirm it's gone.
	userDir := filepath.Join(tmpHome, ".harness", "profiles")
	_, err := os.Stat(filepath.Join(userDir, "user-deletable.toml"))
	require.True(t, os.IsNotExist(err), "deleted profile file should not exist")
}

// TestIsBuiltinProfile verifies the builtin detection function.
func TestIsBuiltinProfile(t *testing.T) {
	assert.True(t, IsBuiltinProfile("github"))
	assert.True(t, IsBuiltinProfile("full"))
	assert.True(t, IsBuiltinProfile("researcher"))
	assert.False(t, IsBuiltinProfile("my-custom-profile"))
	assert.False(t, IsBuiltinProfile("nonexistent-xyz"))
}
