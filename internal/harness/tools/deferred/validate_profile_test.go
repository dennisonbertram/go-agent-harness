package deferred_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateProfileTool_ValidProfile verifies a well-formed profile passes validation.
func TestValidateProfileTool_ValidProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := ValidateProfileTool(dir)

	validToml := `[meta]
name = "my-profile"
description = "A valid test profile"
version = 1
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 10
max_cost_usd = 1.0

[tools]
allow = ["read", "write"]
`
	args := map[string]any{
		"toml": validToml,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := tool.Handler(context.Background(), raw)
	require.NoError(t, err)
	assert.Contains(t, result, "valid")
}

// TestValidateProfileTool_InvalidToml verifies that invalid TOML returns an error.
func TestValidateProfileTool_InvalidToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := ValidateProfileTool(dir)

	invalidToml := `[meta]
name = "broken
this is invalid TOML
`
	args := map[string]any{
		"toml": invalidToml,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

// TestValidateProfileTool_MissingName verifies validation fails for a profile without a name.
func TestValidateProfileTool_MissingName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := ValidateProfileTool(dir)

	noName := `[meta]
description = "No name here"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
`
	args := map[string]any{
		"toml": noName,
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// TestValidateProfileTool_RequiresToml verifies that empty TOML input returns an error.
func TestValidateProfileTool_RequiresToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := ValidateProfileTool(dir)

	args := map[string]any{
		"toml": "",
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = tool.Handler(context.Background(), raw)
	require.Error(t, err)
}
