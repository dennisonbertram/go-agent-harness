package tui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

func TestSkillsCommand_OpensOverlay(t *testing.T) {
	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/skills")

	if !m.OverlayActive() {
		t.Fatal("overlay must be active after /skills")
	}
	if m.ActiveOverlay() != "skills" {
		t.Fatalf("active overlay: got %q want %q", m.ActiveOverlay(), "skills")
	}
}

func TestSkillsOverlay_RendersInstalledSkills(t *testing.T) {
	m := initModel(t, 100, 30)
	m = sendSlashCommand(m, "/skills")

	m2, _ := m.Update(tui.SkillsLoadedMsg{Skills: []tui.SkillEntry{
		{Name: "github-actions", Description: "Manage GitHub Actions", Verified: true},
		{Name: "docker-debug", Description: "Debug Docker issues", Verified: false},
	}})
	m = m2.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Install from GitHub repo") {
		t.Fatalf("skills overlay should show install row; got:\n%s", view)
	}
	if !strings.Contains(view, "github-actions") {
		t.Fatalf("skills overlay should show installed skill; got:\n%s", view)
	}
	if !strings.Contains(view, "docker-debug [unverified]") {
		t.Fatalf("skills overlay should show unverified marker; got:\n%s", view)
	}
}

func TestSkillsOverlay_SelectSkillPrefillsInput(t *testing.T) {
	m := initModel(t, 100, 30)
	m = sendSlashCommand(m, "/skills")

	m2, _ := m.Update(tui.SkillsLoadedMsg{Skills: []tui.SkillEntry{
		{Name: "github-actions", Description: "Manage GitHub Actions", Verified: true},
	}})
	m = m2.(tui.Model)

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m3.(tui.Model)
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m4.(tui.Model)

	if m.OverlayActive() {
		t.Fatal("overlay should close after choosing a skill")
	}
	if got := m.Input(); got != "/github-actions " {
		t.Fatalf("input: got %q want %q", got, "/github-actions ")
	}
}

func TestUnknownSlashInstalledSkill_SubmitsRunPrompt(t *testing.T) {
	var capturedPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/runs":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method %s", r.Method)
			}
			var req struct {
				Prompt string `json:"prompt"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode run request: %v", err)
			}
			capturedPrompt = req.Prompt
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "run-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL = srv.URL
	m := tui.New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	m3, _ := m.Update(tui.SkillsLoadedMsg{Skills: []tui.SkillEntry{
		{Name: "github-actions", Description: "Manage GitHub Actions", Verified: true},
	}})
	m = m3.(tui.Model)

	m4, cmd := m.Update(inputarea.CommandSubmittedMsg{Value: "/github-actions"})
	m = m4.(tui.Model)
	if cmd == nil {
		t.Fatal("expected run command for installed skill slash input")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected non-nil message from run command")
	}
	if capturedPrompt != "/github-actions" {
		t.Fatalf("captured prompt: got %q want %q", capturedPrompt, "/github-actions")
	}
	if _, ok := msg.(tui.RunStartedMsg); !ok {
		t.Fatalf("expected RunStartedMsg, got %T", msg)
	}
}

func TestSkillsOverlay_EnterOnInstallRowStartsInputMode(t *testing.T) {
	m := initModel(t, 100, 30)
	m = sendSlashCommand(m, "/skills")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Install from GitHub") {
		t.Fatalf("skills overlay should enter install mode; got:\n%s", view)
	}
	if !strings.Contains(view, "Paste a GitHub repo URL") {
		t.Fatalf("install mode instructions missing; got:\n%s", view)
	}
}
