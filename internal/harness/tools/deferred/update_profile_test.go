package deferred_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/profiles"
)

// TestUpdateProfileTool_UpdatesExistingProfile verifies the tool can update fields of an existing profile.
func TestUpdateProfileTool_UpdatesExistingProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create an existing profile to update.
	p := &profiles.Profile{
		Meta: profiles.ProfileMeta{
			Name:        "update-target",
			Description: "Original description",
			Version:     1,
			CreatedBy:   "user",
		},
		Runner: profiles.ProfileRunner{
			Model:    "gpt-4.1-mini",
			MaxSteps: 5,
		},
	}
	// Write directly using the internal save function via a temp file.
	data := `[meta]
name = "update-target"
description = "Original description"
version = 1
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "update-target.toml"), []byte(data), 0644))
	_ = p // used implicitly above

	tool := UpdateProfileTool(dir)

	args := map[string]any{
		"name":        "update-target",
		"description": "Updated description",
		"model":       "gpt-4.1",
		"max_steps":   20,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)
	assert.Contains(t, result, "updated")

	// Reload and verify.
	loaded, err := profiles.LoadProfileFromUserDir("update-target", dir)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", loaded.Meta.Description)
	assert.Equal(t, "gpt-4.1", loaded.Runner.Model)
	assert.Equal(t, 20, loaded.Runner.MaxSteps)
}

// TestUpdateProfileTool_RejectsBuiltinProfile verifies that updating a built-in profile fails.
func TestUpdateProfileTool_RejectsBuiltinProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := UpdateProfileTool(dir)

	// "full" is a built-in; no user-dir file exists.
	args := map[string]any{
		"name":        "full",
		"description": "Attempt to modify built-in",
		"model":       "gpt-4.1",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
}

// TestUpdateProfileTool_RejectsNonExistentProfile verifies that updating a nonexistent profile fails.
func TestUpdateProfileTool_RejectsNonExistentProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := UpdateProfileTool(dir)

	args := map[string]any{
		"name":        "does-not-exist-xyz",
		"description": "Will fail",
		"model":       "gpt-4.1-mini",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
