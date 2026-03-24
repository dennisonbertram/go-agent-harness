# Implementation Plan: Issue #391 — feat(cli): add profile selection and inspection to harness CLI and TUI

**Date:** 2026-03-20  
**Issue:** feat(cli): add profile selection and inspection to harness CLI and TUI  
**Status:** Planning  
**Estimated Effort:** 3-5 days  

---

## Executive Summary

Issue #391 adds read-only profile discovery and selection UX to both the CLI and TUI. The CLI gains a `-list-profiles` flag to list available profiles from the server. The TUI gains a `/profiles` slash command that opens a profile picker overlay (modeled on `sessionpicker`) allowing users to select a profile for the next run without persisting it across sessions.

This work depends on issue #377 (profile listing HTTP endpoint) which is already implemented (`GET /v1/profiles`).

---

## Changes Required

### 1. CLI: Add `-list-profiles` Flag

**File:** `cmd/harnesscli/main.go`

**Changes:**

| Line Range | Change | Details |
|-----------|--------|---------|
| 132 | Add new flag | `listProfiles := flags.Bool("list-profiles", false, "list available profiles and exit")` (after `-tui` flag) |
| 138-150 | Add early exit logic | After `flags.Parse(args)`, before TUI/prompt checks: `if *listProfiles { return listProfilesCmd(requestHTTPClient, *baseURL) }` |
| 379+ | Add new function | `func listProfilesCmd(client *http.Client, baseURL string) int { ... }` |

**New Function Signature:**
```go
func listProfilesCmd(client *http.Client, baseURL string) int {
    // GET /v1/profiles
    // Parse response: {"profiles": [...], "count": int}
    // For each profile: print "Name: name, Description: desc, Model: model"
    // Return 0 on success, 1 on error
}
```

**Implementation Details:**
- Make HTTP GET request to `baseURL + "/v1/profiles"`
- Parse JSON response into `map[string]interface{}` or custom struct with `Profiles []map[string]string`
- Print each profile: `Name: <name> | Description: <description> | Model: <model>`
- Handle errors (network, parse, server error responses)
- Return 0 on success, 1 on failure

**Test Cases:**
- `-list-profiles` against live/mock server returns profile list (JSON parsing, print format)
- `-list-profiles` with no profiles returns empty list gracefully
- `-list-profiles` with server error (500, 404) returns error message and exit code 1
- `-list-profiles` output is deterministic (sorted by name for consistent testing)

---

### 2. TUI: Create Profile Picker Component

**New File:** `cmd/harnesscli/tui/components/profilepicker/model.go`

**Component Structure:**

```go
package profilepicker

// ProfileEntry holds display data for a single profile.
type ProfileEntry struct {
    Name        string  // profile name
    Description string  // short description
    Model       string  // LLM model used
    ToolCount   int     // number of allowed tools
    SourceTier  string  // "project" | "user" | "built-in"
}

// Model is the profile picker state machine (value semantics).
type Model struct {
    entries      []ProfileEntry
    selected     int
    scrollOffset int
    open         bool
    Width        int
    Height       int
    filter       string  // optional search query
}

// Methods:
// New(entries []ProfileEntry) Model
// Open() Model
// Close() Model
// IsOpen() bool
// SetEntries(entries []ProfileEntry) Model
// SelectUp() Model
// SelectDown() Model
// Selected() (ProfileEntry, bool)
// SetFilter(q string) Model  // future: search by name/description
// Update(msg tea.Msg) (Model, tea.Cmd)
```

**Files to Create:**

1. `cmd/harnesscli/tui/components/profilepicker/model.go` (~200 lines)
   - ProfileEntry struct
   - Model struct (similar to sessionpicker)
   - New, Open, Close, IsOpen, SetEntries, SelectUp, SelectDown, Selected, Update methods
   - adjustScroll helper

2. `cmd/harnesscli/tui/components/profilepicker/messages.go` (~30 lines)
   - ProfileSelectedMsg{Entry ProfileEntry}

3. `cmd/harnesscli/tui/components/profilepicker/view.go` (~150 lines)
   - View(m Model) string
   - Render list with selection highlight, scrolling, title, instructions
   - Follow sessionpicker styling/layout

4. `cmd/harnesscli/tui/components/profilepicker/model_test.go` (~150 lines)
   - TestNew, TestOpen, TestClose, TestSelectUp, TestSelectDown, TestSelected
   - TestUpdate with key events (Up, Down, Enter, Escape)

**Modeling on sessionpicker:**
- `sessionpicker/model.go` lines 1-150 for structure
- `sessionpicker/messages.go` for message types
- `sessionpicker/view.go` for rendering pattern
- Max visible rows: 10 (similar to sessionpicker)

---

### 3. TUI: Wire Profile Picker into Main Model

**File:** `cmd/harnesscli/tui/model.go`

**Changes:**

| Line | Change | Details |
|------|--------|---------|
| 222 | Add field after `statsPanel statspanel.Model` | `profilePicker profilepicker.Model` |
| 163+ | Add fields for profile state | `selectedProfile string` (name of currently selected profile), `profilePickerOpen bool` (redundant with profilePicker.IsOpen but useful for overlay logic) |

**New Fields in Model struct:**
```go
// Profile-related state
profilePicker       profilepicker.Model
selectedProfile     string  // name of profile selected for next run (not persisted)
```

---

### 4. TUI: Add `/profiles` Slash Command

**File:** `cmd/harnesscli/tui/cmd_parser.go`

**Changes:**

| Line Range | Change | Details |
|-----------|--------|---------|
| 79-153 | Add command entry in `builtinCommandEntries()` | After `subagents` entry (line 150), add new `profiles` command |

**New Command Entry:**
```go
{
    Name:        "profiles",
    Description: "View and select a profile for next run",
    Handler: func(cmd Command) CommandResult {
        return CommandResult{Status: CmdOK}
    },
    Execute: executeProfilesCommand,
}
```

**New Function:** `cmd/harnesscli/tui/model.go` (after `executeSubagentsCommand`, ~line 890)

```go
func executeProfilesCommand(m *Model, _ Command) ([]tea.Cmd, bool) {
    m.overlayActive = true
    m.activeOverlay = "profiles"
    return []tea.Cmd{
        loadProfilesCmd(m.config.BaseURL),
    }, false
}
```

**New Command Function:** `cmd/harnesscli/tui/model.go`

```go
func loadProfilesCmd(baseURL string) tea.Cmd {
    return func() tea.Msg {
        // GET /v1/profiles, parse response
        // Return ProfilesLoadedMsg{entries []profilepicker.ProfileEntry, err error}
    }
}
```

**New Message Type:** `cmd/harnesscli/tui/model.go` or separate file

```go
type ProfilesLoadedMsg struct {
    Entries []profilepicker.ProfileEntry
    Err     error
}
```

---

### 5. TUI: Handle Profile Selection Messages

**File:** `cmd/harnesscli/tui/model.go`

**Changes in Update() method** (lines 914+):

Add case in the message switch (after existing cases):

```go
case profilepicker.ProfileSelectedMsg:
    m.selectedProfile = msg.Entry.Name
    m.profilePicker = m.profilePicker.Close()
    m.overlayActive = false
    m.activeOverlay = ""
    return m, m.setStatusMsg("Profile: " + msg.Entry.Name)

case ProfilesLoadedMsg:
    entries := make([]profilepicker.ProfileEntry, len(msg.Entries))
    for i, p := range msg.Entries {
        entries[i] = profilepicker.ProfileEntry{
            Name:        p.Name,
            Description: p.Description,
            Model:       p.Model,
            ToolCount:   p.ToolCount,
            SourceTier:  p.SourceTier,
        }
    }
    m.profilePicker = m.profilePicker.SetEntries(entries).Open()
    return m, nil
```

---

### 6. TUI: Pass Selected Profile to Run Request

**File:** `cmd/harnesscli/tui/bridge.go` (or wherever run requests are submitted)

**Find:** The `startRunRequest` or equivalent function that builds the POST /v1/runs request body

**Change:** Add profile to request if selected:

```go
if m.selectedProfile != "" {
    req.PromptProfile = m.selectedProfile
}
```

Note: `runCreateRequest` struct already has `PromptProfile` field (main.go line 69).

---

### 7. TUI: Display Selected Profile in Status/Config

**File:** `cmd/harnesscli/tui/components/configpanel/model.go`

**Changes:**

Add a read-only entry showing the selected profile:

```go
// In the config panel initialization or entry update:
if selectedProfile != "" {
    entries = append(entries, ConfigEntry{
        Key:         "Profile",
        Value:       selectedProfile,
        Description: "Selected profile for next run",
        ReadOnly:    true,
    })
}
```

Or alternatively: add profile info to the status bar via `statusbar.Model`.

---

### 8. TUI: Handle Profile Picker Overlay in Key Routing

**File:** `cmd/harnesscli/tui/model.go` (Update method, key handling section, ~line 1162)

**Changes:**

In the main key handling switch, add case for profile picker being open:

```go
case m.overlayActive && m.activeOverlay == "profiles":
    pp, cmd := m.profilePicker.Update(msg)
    m.profilePicker = pp
    if cmd != nil {
        cmds = append(cmds, cmd)
    }
    return m, tea.Batch(cmds...)
```

Also ensure Escape key closes the profile picker:
```go
case key.Type == tea.KeyEsc:
    if m.activeOverlay == "profiles" {
        m.profilePicker = m.profilePicker.Close()
        m.overlayActive = false
        m.activeOverlay = ""
        return m, nil
    }
    // ... other overlay close logic
```

---

### 9. TUI: Render Profile Picker Overlay

**File:** `cmd/harnesscli/tui/view.go` (or the main View() method in model.go)

**Changes:**

In the View() method's overlay rendering logic (near line 1500+), add:

```go
if m.overlayActive && m.activeOverlay == "profiles" {
    overlayContent := m.profilePicker.View()
    return renderOverlay(overlayContent, m.width, m.height)
}
```

Ensure profile picker receives Width/Height updates on WindowSizeMsg (like modelSwitcher does).

---

## Files to Create/Modify Summary

### New Files
1. `cmd/harnesscli/tui/components/profilepicker/model.go` (~200 lines)
2. `cmd/harnesscli/tui/components/profilepicker/messages.go` (~30 lines)
3. `cmd/harnesscli/tui/components/profilepicker/view.go` (~150 lines)
4. `cmd/harnesscli/tui/components/profilepicker/model_test.go` (~150 lines)

### Modified Files
| File | Lines | Change |
|------|-------|--------|
| `cmd/harnesscli/main.go` | 132, 138-150, 379+ | Add `-list-profiles` flag and `listProfilesCmd()` |
| `cmd/harnesscli/tui/cmd_parser.go` | 79-153 | Add `/profiles` command entry in `builtinCommandEntries()` |
| `cmd/harnesscli/tui/model.go` | 222, 163+, 890+, 914+, 1162+ | Add profile picker field, command executor, message handlers, key routing, overlay logic |
| `cmd/harnesscli/tui/bridge.go` | (find startRunRequest) | Add `PromptProfile` to run request if selected |
| `cmd/harnesscli/tui/components/configpanel/model.go` | (entry construction) | Optional: add profile display entry |
| `cmd/harnesscli/tui/view.go` | (overlay rendering) | Add profile picker overlay rendering |

---

## Testing Strategy

### CLI Tests

**File:** `cmd/harnesscli/main_test.go` (or new `cmd/harnesscli/list_profiles_test.go`)

```go
// Test cases:
func TestListProfiles_Success(t *testing.T) {
    // Mock HTTP server returning valid profile list
    // Call listProfilesCmd()
    // Verify exit code 0 and output contains expected profiles
}

func TestListProfiles_EmptyList(t *testing.T) {
    // Mock server returning empty profile list
    // Verify output indicates no profiles
}

func TestListProfiles_ServerError(t *testing.T) {
    // Mock server returning 500 error
    // Verify exit code 1 and error message
}

func TestListProfiles_NetworkError(t *testing.T) {
    // Mock network failure
    // Verify exit code 1
}

func TestListProfiles_OutputFormat(t *testing.T) {
    // Verify output is deterministic (sorted by name)
    // Verify readable format (Name | Description | Model)
}
```

### TUI Tests

**File:** `cmd/harnesscli/tui/components/profilepicker/model_test.go` (new)

```go
func TestProfilePickerModel_New(t *testing.T) { ... }
func TestProfilePickerModel_SelectUp(t *testing.T) { ... }
func TestProfilePickerModel_SelectDown(t *testing.T) { ... }
func TestProfilePickerModel_Update_KeyUpDown(t *testing.T) { ... }
func TestProfilePickerModel_Update_KeyEnter(t *testing.T) { ... }
func TestProfilePickerModel_Update_KeyEsc(t *testing.T) { ... }
func TestProfilePickerModel_ScrollLogic(t *testing.T) { ... }
```

**File:** `cmd/harnesscli/tui/cmd_parser_test.go` (modify)

```go
func TestBuiltinCommandEntries_Profiles(t *testing.T) {
    // Verify /profiles command is registered
    reg := NewCommandRegistry()
    cmd, ok := reg.Lookup("profiles")
    if !ok { t.Fatal("profiles command not found") }
    // Verify description, handler, executor
}
```

**File:** `cmd/harnesscli/tui/model_test.go` or new `cmd/harnesscli/tui/overlay_test.go` (modify)

```go
func TestProfilesCommand_Opens_Overlay(t *testing.T) {
    m := New(testConfig)
    m, cmd := executeProfilesCommand(&m, Command{})
    if !m.overlayActive || m.activeOverlay != "profiles" {
        t.Fatal("profiles overlay not opened")
    }
}

func TestProfileSelectedMsg_Updates_Model(t *testing.T) {
    m := New(testConfig)
    m.overlayActive = true
    m.activeOverlay = "profiles"
    
    msg := profilepicker.ProfileSelectedMsg{
        Entry: profilepicker.ProfileEntry{Name: "test-profile"},
    }
    m.Update(msg)
    
    if m.selectedProfile != "test-profile" {
        t.Fatal("profile not selected")
    }
    if m.overlayActive {
        t.Fatal("overlay should close")
    }
}

func TestRunRequest_Includes_SelectedProfile(t *testing.T) {
    // After selecting a profile, verify startRun request includes it in JSON body
}
```

**Visual Regression Tests:**

```go
func TestProfilePicker_View_Rendering(t *testing.T) {
    entries := []profilepicker.ProfileEntry{
        {Name: "test1", Description: "Test profile 1", Model: "gpt-4", ToolCount: 5, SourceTier: "project"},
        {Name: "test2", Description: "Test profile 2", Model: "claude-opus", ToolCount: 10, SourceTier: "built-in"},
    }
    m := profilepicker.New(entries).Open()
    m.Width = 80
    m.Height = 24
    
    view := m.View()
    // Verify view contains expected elements
    // Use snapshots for regression testing
}
```

---

## Risk Areas & Edge Cases

### Edge Cases

1. **Empty Profile List:** Server returns `{"profiles": [], "count": 0}`
   - CLI: Print "No profiles available"
   - TUI: Show "No profiles" in overlay, don't crash

2. **Profile Not Found After Selection:** User selects profile, but it's deleted before run starts
   - Server will return 404 when run request includes invalid profile name
   - TUI should show error in status bar; don't auto-clear selection

3. **Long Profile Names/Descriptions:** Names > 50 chars, descriptions > 100 chars
   - TUI should truncate with ellipsis or wrap text
   - CLI should handle gracefully (no truncation needed, text display is flexible)

4. **Network Failure During Profile List Load:** User types `/profiles` but network is down
   - TUI should show error in status bar
   - Don't crash; allow retry by typing `/profiles` again

5. **Concurrent Profile Selection & Run Start:** User selects profile, immediately hits Enter to send message
   - Ensure selected profile is captured in the run request before submission
   - Race condition unlikely but testable with goroutine sequencing

6. **Profile Selection Across Sessions:** User selects profile in TUI, quits, reopens
   - Profile selection should NOT persist (as specified: "not persisted")
   - Each new TUI session should start with no selected profile
   - Verify `selectedProfile` is not serialized to config

### Risk Areas

1. **HTTP Client Configuration:** `-list-profiles` uses `requestHTTPClient` (60s timeout) — appropriate for list operation
2. **Async Loading:** `loadProfilesCmd()` must handle network delays; use channel-based message pattern
3. **Overlay State Management:** Multiple overlays (model, keys, profiles) — ensure they don't collide
   - Verify only one overlay is active at a time
   - Escape key behavior is consistent across all overlays
4. **Key Binding Conflicts:** `/profiles` command shares key namespace with other commands
   - Verify fuzzy completion doesn't prioritize wrong command
5. **Component Lifecycle:** Profile picker must respect window resize events like modelSwitcher
   - Test with multiple WindowSizeMsg updates

---

## Commit Strategy

### Commit 1: Profile Picker Component
- Create `cmd/harnesscli/tui/components/profilepicker/model.go`, `messages.go`, `view.go`, `model_test.go`
- Commit message: `feat: add profilepicker TUI component for profile selection overlay`

### Commit 2: CLI `-list-profiles` Flag
- Modify `cmd/harnesscli/main.go` to add flag and function
- Add tests in `cmd/harnesscli/main_test.go`
- Commit message: `feat(cli): add -list-profiles flag to discover available profiles`

### Commit 3: TUI `/profiles` Command & Integration
- Modify `cmd/harnesscli/tui/cmd_parser.go` to add command entry
- Modify `cmd/harnesscli/tui/model.go` to add state fields, command executor, message handlers, key routing
- Modify `cmd/harnesscli/tui/view.go` to add overlay rendering
- Modify `cmd/harnesscli/tui/bridge.go` to pass selected profile to run request
- Add tests in cmd_parser_test.go, model_test.go
- Commit message: `feat(tui): add /profiles command and profile picker overlay for session-scoped profile selection`

### Commit 4: Config Panel Display (Optional)
- Modify `cmd/harnesscli/tui/components/configpanel/model.go` to add profile entry
- Update configpanel snapshots if needed
- Commit message: `feat(tui/configpanel): show selected profile in config panel`

---

## Documentation Updates

### README or CLI Help
- Document `-list-profiles` flag in `cmd/harnesscli/README.md` (if exists)
- Clarify that `-prompt-profile` expects a profile name from the list
- Example: `harnesscli -list-profiles` → shows available profiles, `harnesscli -prompt-profile=test-profile -prompt "..."` → uses that profile

### TUI Help
- Add `/profiles` to the help dialog (likely auto-populated from command registry)
- Document that selected profile applies to next run only (not persisted)

### CLAUDE.md Update
- Note that profile picker is now available via `/profiles` slash command
- Note that CLI has `-list-profiles` flag

---

## Dependencies & Blockers

**Blocked On:**
- Issue #377 (profile listing HTTP endpoint): Already implemented in current codebase ✓

**Depends On:**
- No external dependencies beyond existing go-agent-harness libraries (tea, lipgloss)

**Related Issues:**
- #375: Profile creation/editing (out of scope for #391)
- #381: Tool manifest introspection (optional for profile details)

---

## Success Criteria

1. ✓ CLI: `harnesscli -list-profiles` returns formatted list of available profiles (name, description, model)
2. ✓ CLI: Flag is documented and tested
3. ✓ TUI: `/profiles` slash command opens a profile picker overlay
4. ✓ TUI: Profile picker shows all available profiles with descriptions
5. ✓ TUI: User can navigate with arrow keys and select with Enter
6. ✓ TUI: Selected profile is stored in session state (not persisted)
7. ✓ TUI: Selected profile name appears in run request
8. ✓ TUI: Selected profile is displayed in config panel or status bar
9. ✓ TUI: Escape key closes overlay; status bar shows "Profile: <name>" after selection
10. ✓ All tests pass; no regressions in existing CLI/TUI functionality

---

## Open Questions / Clarifications

1. **CLI Output Format:** Should `-list-profiles` output JSON (for scripting) or human-readable text?
   - Recommended: Human-readable text (Name | Description | Model) with optional `--json` flag for future
2. **Profile Details View:** Should selecting a profile show details (tools, max_steps, etc.) before running?
   - Recommended: Just show name/description in picker; full details in help or on-demand modal
3. **Persistent Selection:** Confirmed: selected profile does NOT persist across TUI sessions (session-scoped only)
4. **Search/Filter:** Should profile picker support search/filter by name or description?
   - Recommended for future; not in initial scope

---

## References

- **Main CLI:** `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/main.go`
- **TUI Model:** `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go`
- **Command Parser:** `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/cmd_parser.go`
- **Session Picker Reference:** `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/components/sessionpicker/`
- **Profile Types:** `/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/profile.go`
- **Profile Loader:** `/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/loader.go`
- **HTTP Profiles Handler:** `/Users/dennisonbertram/Develop/go-agent-harness/internal/server/http_profiles.go`
- **Grooming Notes:** `/Users/dennisonbertram/Develop/go-agent-harness/docs/investigations/issue-391-grooming.md`
