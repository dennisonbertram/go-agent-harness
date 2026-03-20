package deferred

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeleteProfileTool_DeletesUserProfile verifies the tool deletes an existing user profile.
func TestDeleteProfileTool_DeletesUserProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a user profile to delete.
	content := `[meta]
name = "to-delete"
description = "Delete me"
version = 1
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "to-delete.toml"), []byte(content), 0644))

	tool := DeleteProfileTool(dir)

	args := map[string]any{
		"name": "to-delete",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)
	assert.Contains(t, result, "deleted")

	// Verify the file is gone.
	_, statErr := os.Stat(filepath.Join(dir, "to-delete.toml"))
	require.True(t, os.IsNotExist(statErr), "profile file should be deleted")
}

// TestDeleteProfileTool_ProtectsBuiltinProfile verifies built-in profiles cannot be deleted.
func TestDeleteProfileTool_ProtectsBuiltinProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := DeleteProfileTool(dir)

	// "github" is a built-in; no file in the user dir.
	args := map[string]any{
		"name": "github",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
}

// TestDeleteProfileTool_NotFound verifies that deleting a nonexistent profile returns an error.
func TestDeleteProfileTool_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := DeleteProfileTool(dir)

	args := map[string]any{
		"name": "does-not-exist",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestDeleteProfileTool_RequiresName verifies that an empty name returns an error.
func TestDeleteProfileTool_RequiresName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := DeleteProfileTool(dir)

	args := map[string]any{
		"name": "",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}
