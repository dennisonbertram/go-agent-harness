package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// ProfileRunStoreIface is the read-only interface required by GetEfficiencyReportTool
// to query aggregate profile run history. When the store is not available (nil),
// the tool returns a no-history report instead of an error.
//
// This interface is intentionally minimal and forward-compatible with the
// ProfileRunStore planned in #387. When that store lands, callers pass a real
// implementation; until then they pass nil.
type ProfileRunStoreIface interface {
	// AggregateProfileStats returns aggregate statistics for the named profile.
	// Returns (zero, false) when no history exists for the profile.
	AggregateProfileStats(ctx context.Context, profileName string) (profiles.ProfileStats, bool, error)
}

// GetEfficiencyReportTool returns a deferred tool that retrieves a suggest-only
// efficiency report for a named profile.
//
// When store is nil (no storage backend available), the tool returns a report
// with has_history=false and a single not-enough-history suggestion.
//
// Suggestions in the report are NEVER auto-applied.
func GetEfficiencyReportTool(store ProfileRunStoreIface) tools.Tool {
	def := tools.Definition{
		Name:         "get_efficiency_report",
		Description:  descriptions.Load("get_efficiency_report"),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profile", "efficiency", "report", "suggest"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile_name": map[string]any{
					"type":        "string",
					"description": "The name of the profile to retrieve an efficiency report for (e.g. 'researcher', 'github', 'full').",
				},
			},
			"required": []string{"profile_name"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ProfileName string `json:"profile_name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("get_efficiency_report: parse args: %w", err)
		}
		profileName := strings.TrimSpace(args.ProfileName)
		if profileName == "" {
			return "", fmt.Errorf("get_efficiency_report: profile_name is required")
		}

		// If no store is wired, return a no-history report immediately.
		if store == nil {
			report := profiles.BuildAggregateReport(profileName, profiles.ProfileStats{
				ProfileName: profileName,
				RunCount:    0,
			})
			return marshalReport(report)
		}

		// Query aggregate stats from the store.
		stats, found, err := store.AggregateProfileStats(ctx, profileName)
		if err != nil {
			return "", fmt.Errorf("get_efficiency_report: query store: %w", err)
		}
		if !found {
			report := profiles.BuildAggregateReport(profileName, profiles.ProfileStats{
				ProfileName: profileName,
				RunCount:    0,
			})
			return marshalReport(report)
		}

		report := profiles.BuildAggregateReport(profileName, stats)
		return marshalReport(report)
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// marshalReport serialises an AggregateReport to a JSON string.
func marshalReport(report profiles.AggregateReport) (string, error) {
	b, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("get_efficiency_report: marshal report: %w", err)
	}
	return string(b), nil
}
