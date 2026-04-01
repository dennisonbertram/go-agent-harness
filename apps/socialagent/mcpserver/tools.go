package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"go-agent-harness/apps/socialagent/db"
)

// registerTools registers all social tools on the MCPServer.
func (s *Server) registerTools() {
	s.mcpServer.AddTool(buildSearchUsersTool(), s.handleSearchUsers)
	s.mcpServer.AddTool(buildGetUserProfileTool(), s.handleGetUserProfile)
	s.mcpServer.AddTool(buildGetUpdatesTool(), s.handleGetUpdates)
	s.mcpServer.AddTool(buildSaveInsightTool(), s.handleSaveInsight)
	s.mcpServer.AddTool(buildGetMyProfileTool(), s.handleGetMyProfile)
	s.mcpServer.AddTool(buildGetCommunityStatsTool(), s.handleGetCommunityStats)
}

// --- Tool definitions ---

func buildSearchUsersTool() mcp.Tool {
	return mcp.NewTool("search_users",
		mcp.WithDescription("Search for users by interests, traits, or keywords. Returns matching user profiles with display names and summaries."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query — keywords, interests, or traits to look for."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return. Defaults to 10."),
			mcp.DefaultNumber(10),
		),
	)
}

func buildGetUserProfileTool() mcp.Tool {
	return mcp.NewTool("get_user_profile",
		mcp.WithDescription("Get detailed profile information about a specific user by their display name."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The display name of the user to look up."),
		),
	)
}

func buildGetUpdatesTool() mcp.Tool {
	return mcp.NewTool("get_updates",
		mcp.WithDescription("Get recent activity and updates from the community. Shows what other users have been up to."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of activity entries to return. Defaults to 10."),
			mcp.DefaultNumber(10),
		),
		mcp.WithString("exclude_user_id",
			mcp.Description("User ID to exclude from the activity feed (typically the current user)."),
		),
	)
}

func buildSaveInsightTool() mcp.Tool {
	return mcp.NewTool("save_insight",
		mcp.WithDescription("Save an observation or insight about the current user for future reference. Use this to remember important details like interests, preferences, and what they're looking for."),
		mcp.WithString("user_id",
			mcp.Required(),
			mcp.Description("The internal user ID (UUID) of the user this insight is about."),
		),
		mcp.WithString("insight",
			mcp.Required(),
			mcp.Description("The observation or insight to save about the user."),
		),
	)
}

func buildGetMyProfileTool() mcp.Tool {
	return mcp.NewTool("get_my_profile",
		mcp.WithDescription("Get the current user's profile, including what you know about them. Use this when the user asks what you know about them."),
		mcp.WithString("user_id",
			mcp.Required(),
			mcp.Description("The internal user ID (UUID) of the current user."),
		),
	)
}

func buildGetCommunityStatsTool() mcp.Tool {
	return mcp.NewTool("get_community_stats",
		mcp.WithDescription("Get community statistics including total number of users, users with profiles, and total activity count"),
	)
}

// --- Handlers ---

func (s *Server) handleSearchUsers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	limit := int(req.GetFloat("limit", 10))
	if limit <= 0 {
		limit = 10
	}

	profiles, err := s.store.SearchProfiles(ctx, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(profiles) == 0 {
		return mcp.NewToolResultText("No users found matching your search. Try different keywords or browse all users."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d user(s):\n\n", len(profiles))
	for i, p := range profiles {
		name := p.DisplayName
		if name == "" {
			name = p.UserID
		}
		fmt.Fprintf(&sb, "%d. **%s**", i+1, name)
		if p.Summary != "" {
			fmt.Fprintf(&sb, " - %s", p.Summary)
		}
		if len(p.Interests) > 0 {
			fmt.Fprintf(&sb, "\n   Interests: %s", strings.Join(p.Interests, ", "))
		}
		if p.LookingFor != "" {
			fmt.Fprintf(&sb, "\n   Looking for: %s", p.LookingFor)
		}
		sb.WriteString("\n\n")
	}

	return mcp.NewToolResultText(strings.TrimRight(sb.String(), "\n")), nil
}

func (s *Server) handleGetUserProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	user, err := s.store.GetUserByDisplayName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("lookup failed: %v", err)), nil
	}
	if user == nil {
		return mcp.NewToolResultText(fmt.Sprintf("No user found with the name %q. They may not have joined yet.", name)), nil
	}

	profile, err := s.store.GetProfile(ctx, user.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("profile fetch failed: %v", err)), nil
	}

	insights, err := s.store.GetInsights(ctx, user.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("insights fetch failed: %v", err)), nil
	}

	return mcp.NewToolResultText(formatProfile(user.DisplayName, profile, insights)), nil
}

func (s *Server) handleGetUpdates(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := int(req.GetFloat("limit", 10))
	if limit <= 0 {
		limit = 10
	}
	excludeUserID := req.GetString("exclude_user_id", "")

	entries, err := s.store.GetRecentActivity(ctx, limit, excludeUserID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("activity fetch failed: %v", err)), nil
	}

	if len(entries) == 0 {
		return mcp.NewToolResultText("No recent activity from other users."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Recent community updates (%d entries):\n\n", len(entries))
	for _, e := range entries {
		fmt.Fprintf(&sb, "• **%s** [%s]: %s\n", e.DisplayName, e.ActivityType, e.Content)
	}

	return mcp.NewToolResultText(strings.TrimRight(sb.String(), "\n")), nil
}

func (s *Server) handleSaveInsight(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	userID := req.GetString("user_id", "")
	insight := req.GetString("insight", "")

	if userID == "" {
		return mcp.NewToolResultError("user_id parameter is required"), nil
	}
	if insight == "" {
		return mcp.NewToolResultError("insight parameter is required"), nil
	}

	if err := s.store.SaveInsight(ctx, userID, insight, "agent"); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save insight: %v", err)), nil
	}

	return mcp.NewToolResultText("Insight saved successfully. I'll remember that for future conversations."), nil
}

func (s *Server) handleGetMyProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	userID := req.GetString("user_id", "")
	if userID == "" {
		return mcp.NewToolResultError("user_id parameter is required"), nil
	}

	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("profile fetch failed: %v", err)), nil
	}

	insights, err := s.store.GetInsights(ctx, userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("insights fetch failed: %v", err)), nil
	}

	if profile == nil && len(insights) == 0 {
		return mcp.NewToolResultText("I don't know much about you yet! As we chat, I'll learn more about your interests and preferences."), nil
	}

	return mcp.NewToolResultText(formatProfile("", profile, insights)), nil
}

func (s *Server) handleGetCommunityStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats, err := s.store.GetCommunityStats(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get community stats: %v", err)), nil
	}
	b, err := json.Marshal(stats)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to encode stats: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// formatProfile renders a UserProfile and its insights as human-readable text.
func formatProfile(displayName string, profile *db.UserProfile, insights []db.UserInsight) string {
	var sb strings.Builder

	if displayName != "" {
		fmt.Fprintf(&sb, "**%s**\n\n", displayName)
	}

	if profile != nil {
		if profile.Summary != "" {
			fmt.Fprintf(&sb, "Summary: %s\n", profile.Summary)
		}
		if len(profile.Interests) > 0 {
			fmt.Fprintf(&sb, "Interests: %s\n", strings.Join(profile.Interests, ", "))
		}
		if profile.LookingFor != "" {
			fmt.Fprintf(&sb, "Looking for: %s\n", profile.LookingFor)
		}
	}

	if len(insights) > 0 {
		sb.WriteString("\nWhat I know:\n")
		for _, ins := range insights {
			fmt.Fprintf(&sb, "• %s\n", ins.Insight)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
