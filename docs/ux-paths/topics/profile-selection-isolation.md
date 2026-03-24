# UX Stories: Profile Selection & Isolation

Generated: 2026-03-23
Topic: Profile Selection & Isolation

---

## STORY-PR-001: Opening the Profile Picker for the First Time

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer using the TUI for the first time who wants to understand what profiles are available
**Goal**: Open the profile picker and see the full list of available profiles
**Preconditions**: TUI is running (`harnesscli --tui`), no profile is currently selected, server is reachable at the configured base URL

### Steps

1. User types `/` in the input area → Slash command autocomplete dropdown appears above the input, listing all known commands
2. User types `prof` → Dropdown filters to show only `/profiles`
3. User presses Enter (or Tab) → Autocomplete closes; `/profiles` command executes
4. TUI sets `activeOverlay = "profiles"` and fires `loadProfilesCmd` against `GET /v1/profiles` → Status bar flashes "Loading profiles..." for the duration of the fetch
5. Server returns a JSON response containing all profiles across all three tiers → `ProfilesLoadedMsg` is received with a non-nil `Entries` slice
6. Profile picker opens as a rounded-border overlay centered in the terminal → Picker shows "Profiles" as the bold title, followed by up to 10 profile rows; footer shows `↑/↓ navigate  enter select  esc cancel`
7. User reads the first row: `full` profile — dim metadata columns show name (`full`), model (`gpt-4.1-mini`), source tier (`built-in`), description (`Default — all tools available`) → All built-in profiles are visible in the list

### Variations

- Typed `/profiles` in full without using autocomplete: Command still executes identically; autocomplete is optional
- Used keyboard shortcut instead: No dedicated shortcut exists for `/profiles`; the slash command is the only entry point from the TUI

### Edge Cases

- Server takes more than a moment to respond: Status bar shows "Loading profiles..." for the entire duration; overlay region is blank until `ProfilesLoadedMsg` arrives; user cannot interact with the picker yet
- Profile name column exceeds 20 characters: `truncateStr` clips to 19 runes and appends `…`; full name visible only if user selects the entry and reads the status bar confirmation

---

## STORY-PR-002: Selecting a Built-in Profile and Sending a Run

**Type**: medium
**Topic**: Profile Selection & Isolation
**Persona**: Developer who wants to run a read-only analysis task without risking file writes
**Goal**: Select the built-in `researcher` profile and send a prompt that will use it
**Preconditions**: TUI is running, server is reachable, `researcher` profile exists as a built-in with tools: `read`, `grep`, `glob`, `ls`, `web_search`, `web_fetch`

### Steps

1. User types `/profiles` and presses Enter → Picker opens; status bar shows "Loading profiles..."
2. `ProfilesLoadedMsg` arrives with built-in profiles listed → Picker renders; `researcher` entry shows: name=`researcher`, model=`gpt-4.1-mini`, source tier=`built-in`, description=`Read-only analysis, no writes`, tool count=6
3. User presses `j` or Down arrow twice to move the highlight to `researcher` → Selected row renders with reversed-video highlight (purple background, white text); unselected rows render with dim metadata and normal-color description
4. User presses Enter → `ProfileSelectedMsg` is emitted with `Entry.Name = "researcher"`; overlay closes; `selectedProfile` is set to `"researcher"`; status bar flashes "Profile: researcher"
5. User types a prompt: `Summarize all Go files in internal/harness/` and presses Enter → `startRunCmd` fires against `POST /v1/runs` with `profile: "researcher"` in the request body
6. Run starts; server enforces `researcher` profile's tool allowlist — only `read`, `grep`, `glob`, `ls`, `web_search`, `web_fetch` are available to the agent → Agent cannot call `bash` or write files even if prompted to
7. Agent streams results; viewport renders assistant response normally

### Variations

- User navigates with `k` / Up arrow: Moves selection up; wraps from the first entry to the last entry in the list
- User navigates with arrow keys instead of vim keys: `tea.KeyUp` and `tea.KeyDown` produce identical behavior to `k` and `j`

### Edge Cases

- User selects a profile and then sends multiple prompts: `selectedProfile` persists in memory for the rest of the session; all subsequent runs in this TUI session use `researcher` unless the user picks a different profile
- User closes TUI and reopens it: `selectedProfile` is not persisted to disk; the new session starts with no profile selected

---

## STORY-PR-003: Browsing a Long Profile List with Scroll

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Operator who has installed many custom profiles at the project and user level
**Goal**: Navigate a list longer than the visible window and find a specific profile
**Preconditions**: 15 or more profiles exist across all tiers; the picker window is open and the terminal is 80 columns wide

### Steps

1. User opens `/profiles` → Picker fetches all 15+ profiles from `GET /v1/profiles`; server returns summaries sorted by source tier (project first, then user, then built-in)
2. Picker renders with the first 10 entries visible and a dim footer line: `  ... 5 more` below the visible window → User sees that the list extends beyond the viewport
3. User holds `j` (Down) and navigates through all 10 visible rows → Highlight advances one row per keypress; scroll offset stays at 0
4. User presses `j` once more (row 11) → `adjustScroll` shifts the window: `scrollOffset` becomes 1; picker re-renders showing rows 2–11; "... N more" footer updates to reflect remaining hidden entries
5. User continues pressing `j` until reaching the desired profile near the bottom → Scroll window tracks selection; footer disappears when selection is within the last 10 entries
6. User presses Enter → Profile is selected; overlay closes; status bar shows the profile name

### Variations

- User navigates upward from the top: Wraps to the last entry; scroll window jumps to show the last 10 entries
- User navigates downward from the bottom: Wraps to the first entry; scroll offset resets to 0

### Edge Cases

- Exactly 10 profiles exist: No footer appears; no scrolling needed; all rows visible simultaneously
- Exactly 11 profiles exist: `scrollOffset = 0` initially; footer shows "... 1 more"; one `j` press reveals row 11 and hides row 1

---

## STORY-PR-004: Dismissing the Picker Without Selecting

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer who opened the picker by mistake or changed their mind
**Goal**: Close the profile picker without applying any profile
**Preconditions**: Profile picker is open and populated with entries

### Steps

1. User has opened `/profiles` and the picker shows several profiles → `selectedProfile` is currently `""` (no prior selection)
2. User presses `Esc` → `profilePicker.Close()` is called; `overlayActive` is set to false; `activeOverlay` is cleared to `""`
3. Picker overlay disappears; main viewport is visible again → `selectedProfile` remains `""` — no profile was applied
4. User types a new prompt and submits → Run fires without any profile override; server uses its default configuration

### Variations

- User had a profile already selected from an earlier `/profiles` invocation: Pressing Esc on a new picker session does not clear the previously selected profile; `selectedProfile` retains its former value
- User navigates to a row but presses Esc instead of Enter: Selection highlight position is irrelevant; Esc always cancels without applying anything

### Edge Cases

- User presses Esc while the picker is loading (before `ProfilesLoadedMsg` arrives): Because the overlay is open but entries are empty, Esc still closes it cleanly; no crash or stuck state

---

## STORY-PR-005: Handling a Profile Load Failure

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer working with a harnessd server that is temporarily down or misconfigured
**Goal**: Understand what happens when the profile list cannot be fetched
**Preconditions**: TUI is running; harnessd is not reachable or returns a non-200 response on `GET /v1/profiles`

### Steps

1. User types `/profiles` and presses Enter → Overlay opens; status bar shows "Loading profiles..."; `loadProfilesCmd` fires against `GET /v1/profiles`
2. Server is unreachable → HTTP request fails with a connection error; `ProfilesLoadedMsg{Err: <error>}` is returned
3. TUI receives `ProfilesLoadedMsg` with non-nil `Err` → `overlayActive` is set to false; `activeOverlay` is cleared; overlay closes immediately
4. Status bar shows `"Load profiles failed: <error message>"` for 3 seconds → User sees the error without any modal blocking the UI
5. Main chat viewport is visible again → User can continue typing prompts normally without any profile applied

### Variations

- Server returns HTTP 500: `ProfilesLoadedMsg{Err: fmt.Errorf("server returned 500")}` is returned; same flow as a network error
- Server returns malformed JSON: JSON decode fails; same `ProfilesLoadedMsg.Err` path; same status bar message

### Edge Cases

- User immediately retries `/profiles` after a failure: Another `loadProfilesCmd` is dispatched; if the server has recovered, it succeeds and the picker opens normally
- Error message is very long: Status bar truncates or wraps according to its own layout; no overflow into the main viewport

---

## STORY-PR-006: No Profiles Configured — Empty State

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Operator who has just deployed harnessd without any project or user profiles defined
**Goal**: Understand the picker's behavior when no profiles are available
**Preconditions**: No `.harness/profiles/` directory exists; no `~/.harness/profiles/` directory exists; no built-in profiles are embedded in the binary (or built-ins are removed from the build)

### Steps

1. User types `/profiles` → Overlay opens; fetch fires against `GET /v1/profiles`
2. Server returns `{"profiles": [], "count": 0}` → `ProfilesLoadedMsg{Entries: []}` is received with an empty slice
3. Picker opens with zero entries → View renders the "Profiles" title, then a center-padded dim line: `No profiles available`, then the footer instructions `↑/↓ navigate  enter select  esc cancel`
4. User presses `j`, `k`, Up, or Down → All navigation keys are no-ops because `len(entries) == 0`; `SelectUp()` and `SelectDown()` return unchanged models
5. User presses Enter → No `ProfileSelectedMsg` is emitted because `Selected()` returns `(ProfileEntry{}, false)` when the list is empty; nothing happens
6. User presses Esc → Picker closes cleanly; no profile is applied

### Variations

- Built-in profiles exist but project/user profiles do not: Picker shows only the built-in tier entries; this is the typical initial state before any custom profiles are created
- Profiles directory exists but is empty (no `.toml` files): Server returns zero profiles from that tier; built-ins still appear

### Edge Cases

- Picker opens with entries after previously showing empty state: User opens `/profiles` again after operator adds profile files; server now returns profiles; `SetEntries` resets selection to index 0 and scroll to 0

---

## STORY-PR-007: Selecting a Project-Level Profile That Overrides a Built-in

**Type**: medium
**Topic**: Profile Selection & Isolation
**Persona**: Operator who has customized the `researcher` profile for their project's specific toolchain
**Goal**: Verify that the project-level `researcher` profile appears in the picker and takes priority over the built-in
**Preconditions**: `.harness/profiles/researcher.toml` exists in the project root with a custom tool set including `bash`; the built-in `researcher.toml` allows only read-only tools

### Steps

1. User types `/profiles` → Picker fetches `GET /v1/profiles`
2. Server resolves profiles using the three-tier priority order: project-level first, then user-global, then built-in; because a project-level `researcher` exists, the built-in `researcher` is suppressed → Only one `researcher` entry appears in the list
3. Picker renders the `researcher` entry with `SourceTier = "project"` visibly in the dim metadata column → User can distinguish this from the built-in by reading the source tier column
4. User selects `researcher` → `selectedProfile = "researcher"`; status bar shows "Profile: researcher"
5. User sends a prompt → Server loads the project-level profile from `.harness/profiles/researcher.toml`; the expanded tool list (including `bash`) is available to the agent
6. Agent can call `bash` within the project's researcher profile → The project override is transparently applied; the TUI does not need to know which tier won

### Variations

- User-global profile with same name as built-in: Same deduplication logic applies; `SourceTier = "user"` appears in the picker; the user-level TOML is loaded by the server when the run starts
- All three tiers define the same profile name: Project-level always wins; only one entry is shown with `SourceTier = "project"`

### Edge Cases

- Operator deletes the project-level file between the picker fetch and the run start: Server loads the user or built-in tier on run start; the picker display becomes stale but no error is shown at selection time; a run error may surface if the profile resolution fails at run time

---

## STORY-PR-008: Selecting a Container-Isolated Profile

**Type**: long
**Topic**: Profile Selection & Isolation
**Persona**: Developer who wants to run an untrusted code-analysis task in a fully isolated container workspace
**Goal**: Select a profile with `isolation_mode = "container"` and observe that the run is executed in a container workspace transparently
**Preconditions**: A profile named `secure-sandbox` exists at user level (`~/.harness/profiles/secure-sandbox.toml`) with `isolation_mode = "container"`, `cleanup_policy = "delete_on_success"`, and a restricted tool list; harnessd is configured with a Docker daemon accessible

### Steps

1. User types `/profiles` → Picker fetches `GET /v1/profiles`; `secure-sandbox` appears with `SourceTier = "user"` in the dim metadata column and its description indicates it is a sandboxed profile
2. User navigates to `secure-sandbox` and presses Enter → `selectedProfile = "secure-sandbox"`; status bar shows "Profile: secure-sandbox"; overlay closes
3. User types a prompt: `Analyze this untrusted Go module for security issues` and presses Enter → `startRunCmd` fires with `profile: "secure-sandbox"` in the request body
4. Server receives the run request and resolves the `secure-sandbox` profile → Profile's `IsolationMode = "container"` is read from `ProfileValues`; harnessd provisions a new Docker container workspace via the workspace factory
5. Container spins up and `harnessd` polls `/healthz` inside it until the harness is ready to serve → This provisioning latency appears in the TUI as a longer-than-usual delay before the first streaming event; the thinking bar or a "run started" event may not appear immediately
6. Agent run begins inside the container → Tool calls stream normally via SSE; tool use blocks appear in the viewport as usual; the user does not see any container-specific UI
7. Run completes successfully → `cleanup_policy = "delete_on_success"` causes the container and its filesystem to be destroyed after the run; server sends the final `run.completed` event
8. Cost and token usage update in the status bar → User can see the total cost for the isolated run; no indication in the TUI that a container was used (isolation is transparent)

### Variations

- Profile uses `isolation_mode = "worktree"` instead: Server creates a git worktree on the host filesystem; `base_ref` from the profile determines which branch the worktree is based on; TUI experience is identical
- Profile uses `isolation_mode = "none"`: No workspace isolation is applied; run executes in the server's default working directory; this matches the baseline TUI behavior for runs without a profile

### Edge Cases

- Docker daemon is unavailable when the run starts: Server fails to provision the container; `run.failed` SSE event is emitted; TUI appends an error to the viewport and clears run state; no container is left dangling
- Container takes longer than the provisioning timeout: Server times out waiting for harnessd inside the container to become healthy; same `run.failed` path; error message surfaces in the viewport
- User presses `Ctrl+C` during a container-backed run: Interrupt banner appears (Confirm → Waiting → Done states); server sends a cancel signal to the container; cleanup policy runs regardless of success or failure if set to `"delete"`

---

## STORY-PR-009: Switching Profiles Mid-Session

**Type**: medium
**Topic**: Profile Selection & Isolation
**Persona**: Developer who ran one task with the `reviewer` profile and now wants to run a follow-up with `bash-runner`
**Goal**: Switch the active profile between turns without restarting the TUI
**Preconditions**: TUI is running; a previous run completed with `selectedProfile = "reviewer"`; the user now wants to run a shell script execution task

### Steps

1. Previous run with `reviewer` profile is complete; status bar shows the model and cumulative cost → `selectedProfile` holds `"reviewer"`; the TUI did not persist this to disk
2. User types `/profiles` → Picker fetches fresh profile list from `GET /v1/profiles`; `reviewer` and `bash-runner` are both visible
3. User navigates to `bash-runner` → Entry shows: model=`gpt-4.1-mini`, source tier=`built-in`, description=`Script execution, pipeline tasks`, tool count=1 (`bash` only)
4. User presses Enter → `selectedProfile` is updated to `"bash-runner"`; status bar flashes "Profile: bash-runner"; picker closes
5. User types prompt: `Run the regression test suite and report failures` and presses Enter → New run fires with `profile: "bash-runner"`; agent has only `bash` in its tool allowlist
6. Agent runs bash commands to execute the tests; results stream into the viewport

### Variations

- User wants to clear the profile and return to default behavior: No explicit "clear profile" UI exists; user can work around this by creating a profile named `full` (which is the built-in with an empty tool allow list meaning all tools) and selecting it
- User opens `/profiles` while a run is active: Profile picker can be opened but selecting a profile only affects the next run; the active run is unaffected

### Edge Cases

- User switches profiles between turns of a multi-turn conversation: Each `startRunCmd` carries the current `selectedProfile` at the time of submission; different turns in the same conversation can run under different profiles; the conversation ID is preserved regardless of profile changes
- User selects the same profile they already have selected: `selectedProfile` is overwritten with the same value; status bar still flashes "Profile: reviewer"; no observable difference

---

## STORY-PR-010: Using `harnesscli --list-profiles` Without the TUI

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Operator writing CI scripts or automation that needs to enumerate available profiles
**Goal**: List all profiles from the command line and exit, without launching the TUI
**Preconditions**: harnessd is running and reachable; at least some profiles exist across tiers

### Steps

1. Operator runs: `harnesscli --list-profiles` (optionally with `--base-url http://myserver:8080`) → `listProfilesCmd` is called; program issues `GET /v1/profiles` against the base URL
2. Server returns profiles from all three tiers in resolution order → `listProfilesCmd` receives the JSON response, parses it, and sorts profiles alphabetically by name
3. Each profile is printed to stdout in a fixed-width tabular format: `Name: <name padded to 30>  | Description: <description padded to 40>  | Model: <model>` → One profile per line; output is machine-scannable
4. Program exits with code 0 → Shell can capture the output, grep for a specific profile, or pipe it into another tool

### Variations

- No profiles are configured anywhere: Server returns `{"profiles": [], "count": 0}`; `listProfilesCmd` prints `"No profiles available"` and exits 0
- Server is not reachable: `listProfilesCmd` prints an error to stderr (`harnesscli: list-profiles: request failed: ...`) and exits with code 1
- Server returns a non-200 status: Error message printed to stderr: `harnesscli: list-profiles: status 500: ...`; exits code 1

### Edge Cases

- Profile has no description: Output shows `(no description)` in the description column
- Profile has no model override: Output shows `(default)` in the model column
- `--list-profiles` and `--tui` are both specified: `--list-profiles` is checked first in `main.go` (line 145); TUI is never launched; profiles are listed and the program exits

---

## STORY-PR-011: Reading Profile Metadata Before Selecting

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer who is unfamiliar with the available profiles and wants to understand each before committing to one
**Goal**: Read and compare the description, model, tool count, and source tier of multiple profiles before selecting
**Preconditions**: Profile picker is open and populated with a mix of project, user, and built-in profiles

### Steps

1. Picker is open; user reads the first row of the list → Format: `  <name padded 20>  <model padded 20>  <source tier padded 10>  <description up to 40 chars>`; selected row shows all columns in reversed-video highlight; unselected rows show dim metadata with normal-color description text
2. User presses `j` to move down the list, reading each row → `bash-runner`: model=`gpt-4.1-mini`, source tier=`built-in`, description=`Script execution, pipeline tasks`, tool count=1; `file-writer`: model shown, source tier=`built-in`, description visible
3. User wants to see the full description of a profile whose text is truncated at 40 characters → No expand/drill-down exists in the picker; user must refer to the profile TOML file or the server docs for full details
4. User navigates to the profile with the right combination of model and tier → Highlight sits on the target row
5. User presses Enter → Profile is selected; status bar confirms the name; overlay closes

### Variations

- User wants to know the exact list of allowed tools: Tool count is shown in the picker metadata but individual tool names are not; user must inspect the profile TOML directly (at `.harness/profiles/`, `~/.harness/profiles/`, or the embedded binary)
- Model column is blank for a profile: Profile has an empty `runner.model`; the picker displays an empty string in that column position

### Edge Cases

- Source tier column shows an unexpected value: If the server returns an unrecognized tier string, the picker renders it verbatim; no validation or normalization occurs in the TUI

---

## STORY-PR-012: Profile Selection with a Worktree-Isolated Profile and BaseRef

**Type**: long
**Topic**: Profile Selection & Isolation
**Persona**: Developer who wants to run a code-review agent on a feature branch without touching their working tree
**Goal**: Select a worktree-isolated profile that targets a specific base branch, observe that the run starts on an isolated worktree, and return to a clean working directory afterward
**Preconditions**: Project has a profile at `.harness/profiles/branch-reviewer.toml` with `isolation_mode = "worktree"`, `base_ref = "main"`, `cleanup_policy = "delete"`, and tools `["read", "grep", "glob", "ls", "git_diff"]`; git repository is available to harnessd

### Steps

1. User types `/profiles` → Picker opens; `branch-reviewer` appears with `SourceTier = "project"` indicating it is a project-specific profile; description reads something like "Read-only review on worktree off main"
2. User selects `branch-reviewer` → `selectedProfile = "branch-reviewer"`; status bar shows "Profile: branch-reviewer"
3. User types: `Review changes on the current branch against main and list all issues` and submits → Run fires with `profile: "branch-reviewer"` in the request
4. Server reads the profile; `IsolationMode = "worktree"` and `BaseRef = "main"` are resolved from `ProfileValues` → harnessd calls `git worktree add` to create a new worktree based on `main`; the worktree is placed in a temporary directory
5. Agent run starts in the new worktree → Tool calls such as `grep`, `git_diff`, and `read` execute inside the worktree directory; the agent's view of the filesystem is isolated from the user's working tree
6. Agent completes the code review and streams findings into the TUI viewport → User reads the review output; cost updates in the status bar
7. Run completes → `cleanup_policy = "delete"` causes harnessd to call `git worktree remove` and delete the temporary directory; the user's working tree is untouched
8. User can immediately start a new run or switch profiles → No cleanup steps are needed in the TUI

### Variations

- User sets `base_ref = "develop"` in the profile: Worktree is created from `develop` instead of `main`; agent sees the state of `develop` rather than `main`
- `cleanup_policy = "keep"`: Worktree is preserved after the run for manual inspection; server log indicates its path; TUI has no visibility into this lifecycle detail

### Edge Cases

- Git worktree creation fails (e.g., conflicting branch name or dirty index): Server returns a run failure; `run.failed` SSE event triggers TUI error rendering in the viewport; the status bar may show the error if it is short enough
- User's repository has no `main` branch: Worktree creation fails at the server; same error path; user must update `base_ref` in the profile TOML and retry
- User opens `/profiles` while a worktree run is active: Profile picker loads fresh data; selecting a new profile only affects the next run; the in-flight worktree run continues uninterrupted
