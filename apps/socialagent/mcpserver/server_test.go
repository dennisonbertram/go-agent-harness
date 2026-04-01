package mcpserver_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"go-agent-harness/apps/socialagent/db"
	"go-agent-harness/apps/socialagent/mcpserver"
)

// mockStore implements UserStore for testing.
type mockStore struct {
	profiles            []db.UserProfile
	user                *db.User
	profile             *db.UserProfile
	activity            []db.ActivityEntry
	insights            []db.UserInsight
	communityStats      *db.CommunityStats
	saveInsightErr      error
	searchErr           error
	getUserErr          error
	getProfileErr       error
	getActivityErr      error
	getInsightsErr      error
	getCommunityStatsErr error
}

func (m *mockStore) SearchProfiles(_ context.Context, _ string, _ int) ([]db.UserProfile, error) {
	return m.profiles, m.searchErr
}

func (m *mockStore) GetProfile(_ context.Context, _ string) (*db.UserProfile, error) {
	return m.profile, m.getProfileErr
}

func (m *mockStore) GetUserByDisplayName(_ context.Context, _ string) (*db.User, error) {
	return m.user, m.getUserErr
}

func (m *mockStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return m.user, m.getUserErr
}

func (m *mockStore) GetRecentActivity(_ context.Context, _ int, _ string) ([]db.ActivityEntry, error) {
	return m.activity, m.getActivityErr
}

func (m *mockStore) SaveInsight(_ context.Context, _, _, _ string) error {
	return m.saveInsightErr
}

func (m *mockStore) GetInsights(_ context.Context, _ string) ([]db.UserInsight, error) {
	return m.insights, m.getInsightsErr
}

func (m *mockStore) GetAllProfiles(_ context.Context, _ string, _ int) ([]db.UserProfile, error) {
	return m.profiles, m.searchErr
}

func (m *mockStore) GetCommunityStats(_ context.Context) (*db.CommunityStats, error) {
	return m.communityStats, m.getCommunityStatsErr
}

// callTool invokes a named tool on the server and returns the text result.
func callTool(t *testing.T, s *mcpserver.Server, toolName string, args map[string]any) string {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	result, err := s.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool(%s) returned error: %v", toolName, err)
	}
	if result == nil {
		t.Fatalf("CallTool(%s) returned nil result", toolName)
	}
	// Extract text from first content item.
	for _, c := range result.Content {
		if tc, ok := mcp.AsTextContent(c); ok {
			return tc.Text
		}
	}
	return ""
}

// --- search_users ---

func TestSearchUsers_ReturnsResults(t *testing.T) {
	store := &mockStore{
		profiles: []db.UserProfile{
			{
				UserID:    "uid-1",
				Summary:   "Loves hiking and photography",
				Interests: []string{"hiking", "photography"},
				LookingFor: "adventure buddies",
			},
			{
				UserID:    "uid-2",
				Summary:   "Software engineer interested in open source",
				Interests: []string{"programming", "open source"},
				LookingFor: "collaborators",
			},
		},
	}
	s := mcpserver.New(store)

	out := callTool(t, s, "search_users", map[string]any{"query": "hiking", "limit": float64(10)})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "uid-1") && !contains(out, "Loves hiking") {
		t.Errorf("expected output to contain first profile info, got: %s", out)
	}
	if !contains(out, "uid-2") && !contains(out, "Software engineer") {
		t.Errorf("expected output to contain second profile info, got: %s", out)
	}
}

func TestSearchUsers_NoResults(t *testing.T) {
	store := &mockStore{profiles: []db.UserProfile{}}
	s := mcpserver.New(store)

	out := callTool(t, s, "search_users", map[string]any{"query": "nobody", "limit": float64(10)})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "No users found") && !contains(out, "no users") && !contains(out, "0 users") {
		t.Errorf("expected helpful no-results message, got: %s", out)
	}
}

// --- get_user_profile ---

func TestGetUserProfile_Found(t *testing.T) {
	store := &mockStore{
		user: &db.User{
			ID:          "uid-alice",
			DisplayName: "Alice",
		},
		profile: &db.UserProfile{
			UserID:    "uid-alice",
			Summary:   "Alice loves baking and hiking",
			Interests: []string{"baking", "hiking"},
			LookingFor: "friends",
		},
		insights: []db.UserInsight{
			{ID: "ins-1", UserID: "uid-alice", Insight: "Prefers morning chats", Source: "agent", CreatedAt: time.Now()},
		},
	}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_user_profile", map[string]any{"name": "Alice"})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "Alice") {
		t.Errorf("expected output to contain user name, got: %s", out)
	}
	if !contains(out, "baking") && !contains(out, "hiking") {
		t.Errorf("expected output to contain interests, got: %s", out)
	}
}

func TestGetUserProfile_NotFound(t *testing.T) {
	store := &mockStore{user: nil}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_user_profile", map[string]any{"name": "Ghost"})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "not found") && !contains(out, "No user") && !contains(out, "couldn't find") {
		t.Errorf("expected error message about user not found, got: %s", out)
	}
}

// --- get_updates ---

func TestGetUpdates_ReturnsActivity(t *testing.T) {
	now := time.Now()
	store := &mockStore{
		activity: []db.ActivityEntry{
			{ID: "act-1", UserID: "uid-bob", DisplayName: "Bob", ActivityType: "message", Content: "Hello world!", CreatedAt: now},
			{ID: "act-2", UserID: "uid-carol", DisplayName: "Carol", ActivityType: "profile_update", Content: "Updated interests", CreatedAt: now},
		},
	}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_updates", map[string]any{"limit": float64(10), "exclude_user_id": "uid-current"})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "Bob") {
		t.Errorf("expected output to contain Bob, got: %s", out)
	}
	if !contains(out, "Carol") {
		t.Errorf("expected output to contain Carol, got: %s", out)
	}
}

// --- save_insight ---

func TestSaveInsight_Success(t *testing.T) {
	store := &mockStore{}
	s := mcpserver.New(store)

	out := callTool(t, s, "save_insight", map[string]any{
		"user_id": "uid-123",
		"insight": "Loves hiking on weekends",
	})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "saved") && !contains(out, "Saved") && !contains(out, "noted") && !contains(out, "Noted") {
		t.Errorf("expected confirmation message, got: %s", out)
	}
}

// --- get_my_profile ---

func TestGetMyProfile_Found(t *testing.T) {
	store := &mockStore{
		profile: &db.UserProfile{
			UserID:    "uid-me",
			Summary:   "I enjoy coding and coffee",
			Interests: []string{"coding", "coffee"},
			LookingFor: "fellow hackers",
		},
		insights: []db.UserInsight{
			{ID: "ins-1", UserID: "uid-me", Insight: "Night owl", Source: "agent", CreatedAt: time.Now()},
		},
	}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_my_profile", map[string]any{"user_id": "uid-me"})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "coding") && !contains(out, "coffee") {
		t.Errorf("expected profile content, got: %s", out)
	}
}

func TestGetMyProfile_NewUser(t *testing.T) {
	store := &mockStore{profile: nil, insights: nil}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_my_profile", map[string]any{"user_id": "uid-new"})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "don't know") && !contains(out, "Don't know") && !contains(out, "not much") && !contains(out, "new") {
		t.Errorf("expected new-user message, got: %s", out)
	}
}

// --- get_community_stats ---

func TestGetCommunityStats_ReturnsStats(t *testing.T) {
	store := &mockStore{
		communityStats: &db.CommunityStats{
			TotalUsers:        42,
			UsersWithProfiles: 30,
			TotalActivities:   150,
		},
	}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_community_stats", map[string]any{})

	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(out, "42") {
		t.Errorf("expected output to contain total_users=42, got: %s", out)
	}
	if !contains(out, "30") {
		t.Errorf("expected output to contain users_with_profiles=30, got: %s", out)
	}
	if !contains(out, "150") {
		t.Errorf("expected output to contain total_activities=150, got: %s", out)
	}
}

func TestGetCommunityStats_StoreError(t *testing.T) {
	store := &mockStore{
		getCommunityStatsErr: fmt.Errorf("db connection lost"),
	}
	s := mcpserver.New(store)

	out := callTool(t, s, "get_community_stats", map[string]any{})

	if out == "" {
		t.Fatal("expected non-empty error output")
	}
	if !contains(out, "failed") && !contains(out, "error") && !contains(out, "db connection lost") {
		t.Errorf("expected error message in output, got: %s", out)
	}
}

// contains is a helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
