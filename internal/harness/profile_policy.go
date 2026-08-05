package harness

import (
	"strings"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/profiles"
)

// applySelectedProfilePolicy composes an explicitly selected capability
// profile into an ordinary run request. Request values win for non-safety
// policy (model, budgets, prompt and reasoning), while a profile allowlist and
// explicit capability denials are upper bounds that a request cannot widen.
//
// Startup and subagent paths intentionally own their existing composition and
// do not call this function.
func applySelectedProfilePolicy(req RunRequest, profilesDir string) RunRequest {
	if strings.TrimSpace(req.ProfileName) == "" {
		return req
	}
	if strings.TrimSpace(profilesDir) == "" {
		profilesDir = defaultProfilesDir()
	}
	profile, err := profiles.LoadProfileFromUserDir(req.ProfileName, profilesDir)
	if err != nil || profile == nil {
		// Preserve existing ordinary profile behavior: isolation/MCP profile
		// loading is non-fatal, so an unavailable profile cannot turn a run
		// that previously started into a synchronous rejection.
		return req
	}
	values := profile.ApplyValues()
	if req.Model == "" {
		req.Model = values.Model
	}
	if req.MaxSteps == 0 {
		req.MaxSteps = values.MaxSteps
	}
	if req.MaxTurns == 0 {
		req.MaxTurns = values.MaxTurns
	}
	if req.MaxCostUSD == 0 {
		req.MaxCostUSD = values.MaxCostUSD
	}
	if req.SystemPrompt == "" {
		req.SystemPrompt = values.SystemPrompt
	}
	if req.ReasoningEffort == "" {
		req.ReasoningEffort = values.ReasoningEffort
	}
	if req.WorkspaceType == "" && values.IsolationMode != "none" {
		req.WorkspaceType = values.IsolationMode
	}
	req.AllowedTools = intersectSelectedProfileTools(values.AllowedTools, req.AllowedTools)
	req.profileAllowedTools = copyStringSlice(values.AllowedTools)
	req.profileToolsRestricted = len(values.AllowedTools) > 0
	req.profileToolsDenyAll = len(values.AllowedTools) > 0 && req.AllowedTools != nil && len(req.AllowedTools) == 0
	req.profileDeniedActions = selectedProfileDeniedActions(values.Permissions)
	if values.Permissions.BashSpecified() && !values.Permissions.AllowBash {
		req.DeniedTools = appendUniqueTool(req.DeniedTools, "bash")
	}
	return req
}

func appendUniqueTool(tools []string, name string) []string {
	for _, existing := range tools {
		if existing == name {
			return copyStringSlice(tools)
		}
	}
	return append(copyStringSlice(tools), name)
}

// intersectSelectedProfileTools makes a non-empty profile allowlist an upper
// bound. A nil request uses the profile list; a non-nil request can narrow it
// but cannot add a capability absent from the profile.
func intersectSelectedProfileTools(profileAllowed, requested []string) []string {
	if len(profileAllowed) == 0 {
		return copyStringSlice(requested)
	}
	if requested == nil {
		return copyStringSlice(profileAllowed)
	}
	profileSet := make(map[string]struct{}, len(profileAllowed))
	for _, name := range profileAllowed {
		profileSet[name] = struct{}{}
	}
	result := make([]string, 0, len(requested))
	for _, name := range requested {
		if _, ok := profileSet[name]; ok {
			result = append(result, name)
		}
	}
	return result
}

func selectedProfileDeniedActions(permissions profiles.ProfilePermissions) map[htools.Action]struct{} {
	denied := make(map[htools.Action]struct{})
	if permissions.FileWriteSpecified() && !permissions.AllowFileWrite {
		denied[htools.ActionWrite] = struct{}{}
		denied[htools.ActionDownload] = struct{}{}
	}
	if permissions.NetAccessSpecified() && !permissions.AllowNetAccess {
		denied[htools.ActionFetch] = struct{}{}
		denied[htools.ActionDownload] = struct{}{}
	}
	return denied
}
