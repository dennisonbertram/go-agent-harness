package deferred_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateProfileTool_CreatesProfileFile verifies that the tool creates a new profile file.
func TestCreateProfileTool_CreatesProfileFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := CreateProfileTool(dir)

	args := map[string]any{
		"name":        "my-new-profile",
		"description": "A test profile",
		"model":       "gpt-4.1-mini",
		"max_steps":   10,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)
	assert.Contains(t, result, "created")

	// Verify file was written.
	path := filepath.Join(dir, "my-new-profile.toml")
	_, statErr := os.Stat(path)
	require.NoError(t, statErr, "expected profile file to exist at %s", path)
}

// TestCreateProfileTool_RejectsBuiltinName verifies that creating a profile with a built-in name is refused.
func TestCreateProfileTool_RejectsBuiltinName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := CreateProfileTool(dir)

	// "github" is a built-in profile name.
	args := map[string]any{
		"name":        "github",
		"description": "Attempt to shadow built-in",
		"model":       "gpt-4.1-mini",
		"max_steps":   5,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
}

// TestCreateProfileTool_RequiresName verifies that an empty name returns an error.
func TestCreateProfileTool_RequiresName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := CreateProfileTool(dir)

	args := map[string]any{
		"name":        "",
		"description": "Missing name",
		"model":       "gpt-4.1-mini",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// TestCreateProfileTool_RequiresDescription verifies that a missing description returns an error.
func TestCreateProfileTool_RequiresDescription(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := CreateProfileTool(dir)

	args := map[string]any{
		"name":  "valid-name",
		"model": "gpt-4.1-mini",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

// TestCreateProfileTool_RejectsExistingProfile verifies that re-creating an existing profile fails.
func TestCreateProfileTool_RejectsExistingProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := CreateProfileTool(dir)

	args := map[string]any{
		"name":        "my-profile",
		"description": "First creation",
		"model":       "gpt-4.1-mini",
		"max_steps":   5,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	// First creation should succeed.
	_, err = tool.Handler(context.Background(), raw)
	require.NoError(t, err)

	// Second creation of same name should fail.
	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
