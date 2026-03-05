package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type toolArgsPath struct {
	Path     string `json:"path"`
	FilePath string `json:"file_path"`
}

func applyPolicy(def Definition, mode ApprovalMode, policy Policy, handler Handler) Handler {
	if mode == "" {
		mode = ApprovalModeFullAuto
	}
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		if mode == ApprovalModeFullAuto {
			return handler(ctx, args)
		}

		path := ""
		var parsed toolArgsPath
		_ = json.Unmarshal(args, &parsed)
		if parsed.Path != "" {
			path = parsed.Path
		} else if parsed.FilePath != "" {
			path = parsed.FilePath
		}

		if def.Action == ActionRead || def.Action == ActionList {
			return handler(ctx, args)
		}

		in := PolicyInput{
			ToolName:  def.Name,
			Action:    def.Action,
			Path:      path,
			Arguments: args,
			Mutating:  def.Mutating,
		}

		if policy == nil {
			return marshalToolResult(map[string]any{
				"error": map[string]any{
					"code":    "permission_denied",
					"tool":    def.Name,
					"action":  def.Action,
					"reason":  "no policy configured for permissions mode",
					"allowed": false,
				},
			})
		}

		decision, err := policy.Allow(ctx, in)
		if err != nil {
			return marshalToolResult(map[string]any{
				"error": map[string]any{
					"code":    "permission_error",
					"tool":    def.Name,
					"action":  def.Action,
					"reason":  fmt.Sprintf("policy error: %v", err),
					"allowed": false,
				},
			})
		}
		if !decision.Allow {
			reason := decision.Reason
			if reason == "" {
				reason = "policy denied"
			}
			return marshalToolResult(map[string]any{
				"error": map[string]any{
					"code":    "permission_denied",
					"tool":    def.Name,
					"action":  def.Action,
					"reason":  reason,
					"allowed": false,
				},
			})
		}

		return handler(ctx, args)
	}
}
