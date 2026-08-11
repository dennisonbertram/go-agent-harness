package harnessmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestConversationToolsRestoreParity guards a regression I introduced.
//
// Converging /mcp onto the stdio dispatcher (#1317) replaced a surface that had
// list_conversations, get_conversation, search_conversations, and
// compact_conversation with one that had none of them. subscribe_run was also
// dropped, but tail_run_events supersedes it with a cursor. These four had no
// replacement, so HTTP callers lost capability in a change meant to add it.
func TestConversationToolsRestoreParity(t *testing.T) {
	advertised := map[string]bool{}
	for _, tool := range toolDefs() {
		advertised[tool.Name] = true
	}
	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})

	for _, name := range []string{
		"list_conversations", "get_conversation", "search_conversations", "compact_conversation",
	} {
		if !advertised[name] {
			t.Errorf("%q is not advertised", name)
		}
		if _, ok := d.tools[name]; !ok {
			t.Errorf("%q has no handler", name)
		}
	}
}

func TestConversationToolsHitTheirEndpoints(t *testing.T) {
	for _, tc := range []struct {
		tool     string
		args     string
		wantPath string
		wantQ    string
		build    func(*HarnessClient) ToolHandler
	}{
		{"list_conversations", `{}`, "/v1/conversations/", "", newListConversationsHandler},
		{"get_conversation", `{"conversation_id":"c1"}`, "/v1/conversations/c1", "", newGetConversationHandler},
		{"search_conversations", `{"query":"needle"}`, "/v1/conversations/search", "needle", newSearchConversationsHandler},
		{"compact_conversation", `{"conversation_id":"c1"}`, "/v1/conversations/c1/compact", "", newCompactConversationHandler},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			var gotPath, gotQ string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQ = r.URL.Query().Get("q")
				_, _ = w.Write([]byte(`{"conversations":[],"ok":true}`))
			}))
			defer srv.Close()

			res, err := tc.build(NewHarnessClient(srv.URL))(context.Background(), json.RawMessage(tc.args))
			if err != nil {
				t.Fatalf("%s: %v", tc.tool, err)
			}
			if res.IsError {
				t.Fatalf("%s errored: %s", tc.tool, res.Content[0].Text)
			}
			if gotPath != tc.wantPath {
				t.Errorf("%s hit %q, want %q", tc.tool, gotPath, tc.wantPath)
			}
			if tc.wantQ != "" && gotQ != tc.wantQ {
				t.Errorf("%s sent q=%q, want %q", tc.tool, gotQ, tc.wantQ)
			}
		})
	}
}

// TestConversationToolsValidateRequiredArgs — a missing id must be a clear error,
// not a request to a malformed path.
func TestConversationToolsValidateRequiredArgs(t *testing.T) {
	c := NewHarnessClient("http://127.0.0.1:1")
	for name, h := range map[string]ToolHandler{
		"get_conversation":     newGetConversationHandler(c),
		"compact_conversation": newCompactConversationHandler(c),
	} {
		res, _ := h(context.Background(), json.RawMessage(`{}`))
		if !res.IsError || !strings.Contains(res.Content[0].Text, "conversation_id") {
			t.Errorf("%s with no id must name the missing argument, got: %+v", name, res)
		}
	}
	res, _ := newSearchConversationsHandler(c)(context.Background(), json.RawMessage(`{}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "query") {
		t.Errorf("search_conversations with no query must name it, got: %+v", res)
	}
}
