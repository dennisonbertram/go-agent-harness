# UX Stories: Model & Provider Selection

**Topic**: Model & Provider Selection
**Generated**: 2026-03-23

---

## STORY-MP-001: Basic Model Switch via Provider Browser

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to try a different model mid-session
**Goal**: Switch from the current OpenAI model to Claude Sonnet without leaving the TUI
**Preconditions**: TUI is running with `gpt-4.1` as the active model; Anthropic API key is configured; no overlay is open

### Steps

1. User types `/model` in the input area and presses Enter → The slash autocomplete dropdown appears as soon as `/` is typed; typing `model` filters it to the single entry; pressing Enter (or Tab) executes the command immediately
2. The model overlay opens at Level 0, showing the provider list: Anthropic, DeepSeek, Google, Groq, Kimi, OpenAI, Qwen, xAI (sorted alphabetically) → The TUI fires `GET /v1/models` and `GET /v1/providers` in the background; the overlay shows "Loading models..." while the fetch is in progress; the provider cursor is pre-positioned on the OpenAI row (current model's provider) with `← current` label
3. Providers arrive; each row shows a model count `(N)` right-aligned and a `●` (configured) or `○` (unconfigured) indicator → Anthropic shows `●` because its API key is set; the cursor stays on OpenAI
4. User presses `k` (or Up) to move the cursor to Anthropic → The Anthropic row becomes highlighted in reverse-video
5. User presses Enter → The overlay transitions to Level 1, showing the Anthropic model list: Claude Haiku 4.5, Claude Opus 4.6, Claude Sonnet 4.6 (alphabetical order); a breadcrumb header reads `< Back  [Anthropic]`
6. User presses `j` (or Down) to navigate to Claude Sonnet 4.6 → The row highlights; the `← current` marker is absent (it belongs to gpt-4.1 on OpenAI)
7. User presses Enter → The config panel (Level 2) opens for Claude Sonnet 4.6, showing: model name + "Anthropic" provider line, Gateway section with `▶ Direct` selected, API Key section showing `● configured` in green, footer `↑/↓ sections  ←/→ gateway  enter confirm  esc back`
8. User presses Enter to confirm with the current gateway and key settings → The overlay closes; the status bar at the bottom now reads `claude-sonnet-4-6  $0.00`; the model has been switched for the next run

### Variations

- **Tab-complete shortcut**: User types `/mo` — the dropdown filters to `/model` as the only match and auto-executes on Tab, skipping the Enter press
- **Already on Anthropic**: If the user had claude-sonnet-4-6 as the current model, opening `/model` pre-positions the provider cursor on Anthropic and the model cursor on Claude Sonnet 4.6 when drilling in

### Edge Cases

- **Fetch fails**: If `GET /v1/models` returns an HTTP error, the overlay shows the error string in red with `esc cancel` as the only available footer action; the user must dismiss and retry
- **Provider list empty**: If the server returns no providers, the overlay shows "No providers available" in dim text

---

## STORY-MP-002: Starring a Model for Quick Access

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer who switches between two models frequently
**Goal**: Star Claude Opus 4.6 so it appears at the top of any future model list
**Preconditions**: TUI is running; `/model` overlay is open at Level 1 on the Anthropic model list; Claude Opus 4.6 is not yet starred

### Steps

1. User navigates to Claude Opus 4.6 with `j`/`k` → The row highlights, showing `> Claude Opus 4.6  ← current` (or without the current marker if another model is active)
2. User presses `s` → The row immediately gains a gold `★` prefix; the entry is moved to the top of the visible list (starred models sort first within any provider view); the `StarredIDs()` list is written to the persistent config file
3. User presses Esc to close the overlay without selecting a model → The overlay closes; the config file retains the star; the current model is unchanged

### Variations

- **Unstarring**: If Claude Opus 4.6 was already starred, pressing `s` removes the `★` and moves the model back to its alphabetical position in the unstarred group; the config file is updated immediately
- **Star during search**: The user types `opus` in the search filter (flat cross-provider view); with Claude Opus 4.6 highlighted, pressing `s` stars it in the same way — stars persist across search and browse modes

### Edge Cases

- **No visible models**: If the search query matches nothing, pressing `s` is a no-op (there is nothing to star)
- **Cursor out of range**: The toggle guard `m.Selected >= len(visible)` prevents a panic if the model list is empty

---

## STORY-MP-003: Cross-Provider Search from Any Level

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer who knows the model name but not which provider hosts it
**Goal**: Find and select "grok" without navigating the provider hierarchy
**Preconditions**: TUI running; `/model` overlay is open at Level 0 (provider list); no search is active

### Steps

1. User types `g` → The overlay switches to the flat cross-provider search view; the header shows "Switch Model" with a `Filter: g` line; visible models are filtered to any whose display name contains "g" (case-insensitive); starred matches appear first
2. User continues typing `r`, `o`, `k` → The filter narrows to "Grok 3 Mini" and "Grok 4.1 Fast [R]" from xAI; both rows show a `[xAI]` provider prefix in dim text; Grok 4.1 Fast shows `[R]` badge indicating reasoning mode
3. User navigates to Grok 3 Mini with `j`/`k` → The row highlights
4. User presses Enter → The config panel opens for Grok 3 Mini (non-reasoning model), showing Gateway and API Key sections only (no Reasoning Effort section)
5. User confirms with Enter → Model switches to grok-3-mini; overlay closes; status bar updates

### Variations

- **Search from Level 1**: If the user is already browsing inside a provider (Level 1) and starts typing, the search activates globally — results span all providers, not just the currently active one

### Edge Cases

- **No matches**: Typing a query that matches nothing shows "No models match" in dim text; Backspace progressively removes characters; pressing Esc clears the search and returns to whichever browse level was active before the search
- **Backspace to empty**: Deleting the entire search query restores the previous browse level view (Level 0 provider list or Level 1 provider model list)

---

## STORY-MP-004: Unconfigured Provider Redirects to Keys Overlay

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer trying a new provider for the first time
**Goal**: Select a Groq model even though no Groq API key has been set
**Preconditions**: TUI running; Groq API key is NOT configured; `/model` overlay is open at Level 0; Groq shows `○` (unconfigured) indicator

### Steps

1. User navigates to Groq with `j`/`k` and presses Enter → The overlay drills into Level 1, showing Groq models: Llama 3.3 70B and QwQ 32B; both rows display `(unavailable)` in muted text because the provider is unconfigured; the `●`/`○` key status indicator shows `○` next to each model
2. User selects QwQ 32B (reasoning model, `[R]` badge) and presses Enter → The config panel (Level 2) opens for QwQ 32B; the API Key section shows `○ not set` in dim style; the Gateway section defaults to Direct
3. User attempts to confirm with Enter (navigating to the Gateway section and pressing Enter) → The TUI detects the provider is unconfigured; instead of confirming the model switch, it closes the model overlay and opens the `/keys` overlay, pre-positioning the cursor on the Groq row in the provider list
4. The `/keys` overlay is now active, showing all providers with their configured status; the cursor sits on "Groq  GROQ_API_KEY  ○ unset"
5. User presses Enter → The overlay transitions to key input mode for Groq: title changes to "API Keys > Groq"; the env var name `GROQ_API_KEY` is shown in dim text; an input field `> █` appears with a block cursor
6. User types the API key value and presses Enter → The TUI fires `PUT /v1/providers/groq/key` with the key; on HTTP 204/200 the server emits `APIKeySetMsg`; the `/keys` overlay updates the Groq row to show `● set`; the config is persisted
7. User presses Esc to return to the provider list in the `/keys` overlay, then Esc again to close the overlay → The model selection is not automatically re-applied; the user re-opens `/model` to complete the switch

### Variations

- **Configure key in config panel directly**: The user focuses the API Key section in the config panel (Level 2) by pressing `↑`/`↓` to move section focus, then presses K to enter inline key input mode within the config panel itself, types the key, presses Enter to confirm, and continues to the gateway/confirm flow without leaving the config panel

### Edge Cases

- **Key set fails**: If the PUT returns a non-2xx status, the TUI falls through silently (no dedicated error state); the provider row remains `○ unset`; the user must try again
- **OpenRouter key**: Setting the OpenRouter key configures routing for all providers via OpenRouter; the model availability check for OpenRouter-routed sessions depends solely on whether the OpenRouter key is set, not the per-provider keys

---

## STORY-MP-005: Setting and Changing a Provider API Key via /keys

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Operator who needs to rotate an API key for an already-configured provider
**Goal**: Replace the existing Anthropic API key with a new one
**Preconditions**: TUI running; `/keys` command typed; Anthropic API key is currently configured (shows `● set`)

### Steps

1. User types `/keys` and presses Enter → The `/keys` overlay opens, listing all known providers with their configured status and env var names; the cursor starts at the top of the list; Anthropic shows `● set` in gold
2. User navigates with `j`/`k` to position the cursor on Anthropic → The row highlights with `▶` cursor
3. User presses Enter → Input mode activates for Anthropic; the title updates to "API Keys > Anthropic"; the env var `ANTHROPIC_API_KEY` is shown; the input field appears empty with a block cursor (the existing key value is never pre-filled for security)
4. User types the new API key value → Characters accumulate in the input field; the block cursor advances
5. (Optional) User presses Ctrl+U to clear the input if they mistyped → The input field clears to empty; the block cursor returns to position 0
6. User presses Enter → The TUI fires `PUT /v1/providers/anthropic/key`; on success the overlay shows Anthropic as `● set` (it already was, so the visual change is minimal); the new key is now active on the server
7. User presses Esc → Returns to the provider list (not input mode); pressing Esc again closes the `/keys` overlay entirely; the chat input regains focus

### Variations

- **Add a new provider key**: Same flow, but the target provider shows `○ unset` before step 3; after step 6 it transitions to `● set`
- **Via status bar hint**: If the user tries to send a run with an unconfigured model, the run fails with a clear backend error; the user can then open `/keys` to fix the issue

### Edge Cases

- **Empty key submission**: Pressing Enter with an empty input still fires the PUT; the server may accept or reject an empty key depending on its validation; there is no client-side guard
- **Esc during input mode**: Pressing Esc in input mode exits to the provider list view (does NOT close the whole overlay); the user must press Esc a second time to close

---

## STORY-MP-006: Selecting Gateway — Direct vs OpenRouter

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to route all traffic through OpenRouter to use a single API key
**Goal**: Switch the active gateway from Direct to OpenRouter
**Preconditions**: TUI running; gateway is currently set to Direct; OpenRouter API key is configured; `/model` overlay is open; user has navigated to a model and entered the config panel (Level 2)

### Steps

1. The config panel is open for GPT-4.1 (OpenAI); the Gateway section shows two options: `▶ Direct  Each model's native provider` and `  OpenRouter  Route all via openrouter.ai`; the cursor (`▶`) is on Direct, matching the current gateway
2. User presses `←`/`→` (or navigates with Up/Down when focused on the Gateway section) to move the cursor from Direct to OpenRouter → The cursor (`▶`) moves to the OpenRouter row; the row renders in gold/bold to indicate it is highlighted
3. User presses Enter to confirm → The gateway is saved as `"openrouter"`; the TUI emits `GatewaySelectedMsg{Gateway: "openrouter"}`; the overlay closes; the status bar label updates to show the gateway context (e.g. "Gateway: OpenRouter"); the TUI fires `fetchOpenRouterModelsCmd` to load the live OpenRouter model catalog, replacing the default model list with the full OpenRouter catalog (IDs like `openai/gpt-4.1`, `anthropic/claude-opus-4-6`, etc.)
4. Future runs use OpenRouter slugs (e.g. `openai/gpt-4.1`) and pass `provider_name: "openrouter"` in the `POST /v1/runs` body

### Variations

- **Gateway overlay via /provider command**: The gateway can also be changed by typing `/provider` (if wired as a slash command); this opens the standalone "Routing Gateway" overlay showing the same Direct/OpenRouter choice with `▶` cursor navigation and j/k support, centered on screen
- **Switching back to Direct**: Same flow; the user selects Direct; the TUI switches back to the server model list from `GET /v1/models`

### Edge Cases

- **OpenRouter key not set**: If the OpenRouter API key is not configured, the overlay still allows selecting OpenRouter; `fetchOpenRouterModelsCmd` makes an unauthenticated request to the OpenRouter public catalog; models load correctly but rate limits may apply; the availability indicator for all models will show `○` until the OpenRouter key is set
- **OpenRouter fetch fails**: `ModelsFetchErrorMsg` is emitted; the model switcher shows the error string in red; the gateway setting is still saved even if the model fetch fails

---

## STORY-MP-007: Configuring Reasoning Effort for a Reasoning Model

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to tune the trade-off between reasoning depth and cost/speed
**Goal**: Set DeepSeek Reasoner to "high" reasoning effort before a complex task
**Preconditions**: TUI running; DeepSeek Reasoner is NOT the current model; Anthropic key is configured (current is claude-sonnet-4-6); DeepSeek API key IS configured

### Steps

1. User opens `/model` → Level 0 provider list appears; DeepSeek shows `●` (configured); cursor is pre-positioned on Anthropic (current model's provider)
2. User navigates to DeepSeek and presses Enter → Level 1 shows DeepSeek models: DeepSeek Chat and DeepSeek Reasoner; DeepSeek Reasoner shows the `[R]` reasoning badge
3. User navigates to DeepSeek Reasoner and presses Enter → The config panel (Level 2) opens; it contains three sections: Gateway, API Key, and Reasoning Effort (because `ReasoningMode: true` for this model); the footer reads `↑/↓ sections  ←/→ gateway  enter confirm  esc back`
4. User presses Down (or `j`) to move section focus from Gateway → API Key → Reasoning Effort → The "Reasoning Effort" label renders in gold/bold to indicate it is focused; the four options are listed: Default (← current), Low, Medium, High
5. User presses Down to move the cursor within the Reasoning Effort section to "High" → The `▶` cursor moves to High
6. User presses Enter to confirm the entire config panel → The TUI saves: gateway = Direct (unchanged), reasoning effort = "high"; emits `GatewaySelectedMsg` and a model-selected message; closes the overlay; the status bar updates to `deepseek-reasoner  $0.00`; the next `POST /v1/runs` body includes `"reasoning_effort": "high"`

### Variations

- **Default effort**: The user skips the Reasoning Effort section and confirms with Enter; the effort field is `""` (empty string = server default); this is the default behavior for any reasoning model
- **Mid-run change**: The reasoning effort setting persists for all subsequent runs in the session until changed again via `/model`

### Edge Cases

- **Non-reasoning model**: Models without `ReasoningMode: true` do not show the Reasoning Effort section in the config panel; the section count is 2 (Gateway + API Key); Up/Down wrap between those two sections only
- **Current effort preserved**: If the user previously set "medium" effort and reopens the config panel for the same reasoning model, the `← current` marker appears next to "Medium" and the cursor starts there

---

## STORY-MP-008: Discovering and Using a Model from OpenRouter's Expanded Catalog

**Type**: long
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to try a model not in the default list (e.g. a new Mistral model)
**Goal**: Switch to an OpenRouter-exclusive model that is not in the default `DefaultModels` list
**Preconditions**: TUI running with Direct gateway; OpenRouter API key is configured; current model is gpt-4.1

### Steps

1. User opens the config panel for any model via `/model` → navigates to any provider → selects any model → config panel opens
2. In the Gateway section, the user moves the cursor to OpenRouter and presses Enter to confirm → Gateway switches to "openrouter"; the TUI fires `fetchOpenRouterModelsCmd`; the overlay closes
3. The TUI fetches `https://openrouter.ai/api/v1/models` with the Authorization header (because the OpenRouter key is set); a `ModelsFetchedMsg{Source: "openrouter"}` arrives with the full catalog, including hundreds of models not in `DefaultModels` (e.g. `mistralai/mistral-large`, `meta-llama/llama-4-70b`)
4. User opens `/model` again → Level 0 shows a richer provider list derived from the OpenRouter catalog (providers extracted from the `{provider}/` prefix of each OpenRouter model ID); the indicator column shows availability based on the OpenRouter key alone
5. User navigates to "mistralai" (or types `mis` to filter) and presses Enter → Level 1 shows Mistral models from OpenRouter with their OpenRouter-supplied display names
6. User selects `mistralai/mistral-large` and presses Enter → Config panel opens; Gateway shows OpenRouter selected (because that is the current gateway); API Key shows `● configured` (the OpenRouter key covers all models); the provider line shows "mistralai"
7. User confirms with Enter → The overlay closes; the model is set to the OpenRouter slug `mistralai/mistral-large`; the next run posts with `model: "mistralai/mistral-large"` and `provider_name: "openrouter"`

### Variations

- **Switching back to Direct**: The user re-opens `/model`, changes the gateway to Direct in the config panel, confirms → The TUI re-fetches `GET /v1/models` and restores the default model list; the OpenRouter-only model is no longer visible

### Edge Cases

- **OpenRouter fetch slow or times out**: The client has a 10-second timeout; on timeout, `ModelsFetchErrorMsg` is emitted with "openrouter fetch: ..." error; the overlay shows the error in red; the gateway setting is preserved but the model list is not updated
- **Model ID not in display name map**: OpenRouter models not in `modelDisplayNames` fall through to using the OpenRouter-supplied `name` field; if that is also empty, the raw ID is used as the display name
- **Stars for OpenRouter models**: A user can star any model in the flat search view, including OpenRouter-only models; the star persists to config by model ID and survives gateway switches

---

## STORY-MP-009: Navigating the Config Panel Sections with Keyboard

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Keyboard-centric developer who prefers not to use the mouse
**Goal**: Efficiently configure gateway, API key, and reasoning effort for QwQ 32B in one pass using only keyboard
**Preconditions**: TUI running; Groq is configured; current model is gpt-4.1; QwQ 32B (a Groq reasoning model) is NOT the current model; `/model` overlay is at Level 0

### Steps

1. User types `qwq` in the search filter → The flat search view narrows to "QwQ 32B [R]" with provider prefix `[Groq]`; the model is highlighted
2. User presses Enter → Config panel opens with three sections: Gateway (focused, section 0), API Key (section 1), Reasoning Effort (section 2)
3. User presses `→` to move the gateway cursor from Direct to OpenRouter → The cursor moves to OpenRouter
4. User presses `←` to move back to Direct → Cursor returns to Direct; the user prefers direct routing for Groq
5. User presses Down → Section focus moves from Gateway (0) to API Key (1); "API Key" label renders in gold; status shows `● configured` because Groq key is set
6. User presses Down again → Section focus moves from API Key (1) to Reasoning Effort (2); "Reasoning Effort" label renders in gold; the cursor is on "Default (← current)"
7. User presses Down within the Reasoning Effort option list to select "Medium" → The `▶` cursor moves to Medium
8. User presses Enter → The entire config is confirmed: gateway = Direct, reasoning effort = "medium"; the model switches to `qwen-qwq-32b` (Groq's QwQ 32B model ID); the overlay closes

### Variations

- **Up navigation**: The user can navigate sections in reverse (section 2 → 1 → 0) with Up key; wrap-around is not implemented — navigation stops at the top and bottom sections

### Edge Cases

- **Enter in gateway section**: Pressing Enter while focused on the Gateway section (section 0) confirms the entire config panel (not just the gateway sub-selection); the panel closes and all settings are applied
- **Key input mode activated accidentally**: If the user is in the API Key section and presses K to activate key input mode, they can press Esc to exit input mode without leaving the config panel, then continue navigating sections

---

## STORY-MP-010: Unstarring a Model that is No Longer Needed

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer cleaning up a cluttered starred model list
**Goal**: Remove the star from GPT-4.1 Mini so it returns to its alphabetical position
**Preconditions**: TUI running; GPT-4.1 Mini is starred (shows `★` in the OpenAI model list); `/model` overlay is open at Level 0

### Steps

1. User navigates to OpenAI and presses Enter → Level 1 shows OpenAI models; GPT-4.1 Mini appears at the top of the list with a gold `★` prefix because it is starred; GPT-4.1 appears below it without a star
2. User verifies the cursor is on GPT-4.1 Mini (it is at the top due to starred-first ordering) and presses `s` → The `★` is removed immediately; GPT-4.1 Mini moves to its alphabetical position below GPT-4.1; the cursor follows the model to its new position; the updated `StarredIDs()` list (now empty or without this ID) is written to the config file
3. User presses Esc to exit to the provider list, then Esc again to close the overlay → The model selection is unchanged; only the star status was modified

### Variations

- **Unstar during search**: The user types `mini` in the search filter; GPT-4.1 Mini (starred) appears at the top of the cross-provider flat list; pressing `s` removes the star and repositions the model alphabetically within the search results
- **Multiple starred models**: If several models are starred, pressing `s` on one removes only that one; others remain starred and continue to appear at the top

### Edge Cases

- **Cursor drift after unstar**: After unstarring, the cursor index (`m.Selected`) may point to a different model if the list reorders; the implementation moves the cursor to follow the toggled model by its new index position

---

## STORY-MP-011: First-Time Setup — No Models Configured

**Type**: long
**Topic**: Model & Provider Selection
**Persona**: New user who just installed the harness and launched the TUI for the first time
**Goal**: Configure an OpenAI API key and select GPT-4.1 to send the first message
**Preconditions**: TUI has just launched; no API keys are configured on the server; the welcome hint is visible in the viewport ("no model configured" state)

### Steps

1. The TUI shows an empty viewport with a first-time welcome hint because `selectedModel == ""` → The status bar shows no model name and `$0.00`
2. User types `/model` and presses Enter → The model overlay opens; the TUI fires `GET /v1/models` and `GET /v1/providers`; the loading indicator appears
3. Models and providers load; all providers show `○` (unconfigured) because no API keys are set → The provider list appears with all entries greyed; the `○` indicator appears next to each provider count
4. User navigates to OpenAI (press `j`/`k`) and presses Enter → Level 1 shows GPT-4.1 and GPT-4.1 Mini; both rows show `(unavailable)` in muted text with a `○` key status indicator
5. User selects GPT-4.1 and presses Enter → The config panel opens for GPT-4.1; the API Key section shows `○ not set`; the Gateway section defaults to Direct
6. User navigates to the API Key section (press Down) and presses K to activate inline key input mode → The API Key section expands to show an input field `> █` and the hint `enter confirm  ctrl+u clear  esc back`
7. User types the OpenAI API key and presses Enter → The TUI fires `PUT /v1/providers/openai/key`; on success the API Key section updates to `● configured` (green)
8. User presses Enter to confirm the config panel (with Direct gateway, no reasoning effort) → The model switches to `gpt-4.1`; the overlay closes; the status bar updates to `gpt-4.1  $0.00`
9. User types a prompt and presses Enter → The first run starts; `POST /v1/runs` is called with `model: "gpt-4.1"`, `provider_name: "openai"`; the streamed response appears in the viewport

### Variations

- **Keys first via /keys**: The user opens `/keys` before `/model`, sets the OpenAI key, then returns to `/model` to pick a model; the provider shows `●` configured from the start
- **Redirect from model overlay**: If the user tries to confirm a model whose provider is unconfigured (without setting the key first), the overlay redirects to `/keys` with the cursor pre-positioned on OpenAI, then the user follows steps 6-7 above

### Edge Cases

- **No server running**: If the TUI cannot reach the harness server, `GET /v1/models` fails with a network error; the model switcher shows the error in red; no providers are loaded; the user cannot select a model until the server is running
- **Welcome hint timing**: The welcome hint is shown only when `selectedModel == ""` AND the viewport is empty AND no run is active; after the model is selected in step 8, the hint disappears immediately even before the first message is sent

---

## STORY-MP-012: Esc Key Priority and Safe Dismissal of the Model Overlay

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer who accidentally opened the model overlay or changed their mind
**Goal**: Close the model overlay cleanly from various intermediate states without making unwanted changes
**Preconditions**: TUI running; `/model` overlay is open

### Steps (from config panel)

1. User is in the config panel (Level 2) with the API key input field active (key input mode) → The footer reads `enter confirm  ctrl+u clear  esc back`
2. User presses Esc → Key input mode exits; the config panel remains open (no model change); the footer returns to section navigation hints
3. User presses Esc again → The config panel closes; the user returns to Level 1 (model list for the active provider); no model was changed
4. User presses Esc again → Returns to Level 0 (provider list); search is cleared if it was active
5. User presses Esc at Level 0 with no search active → The model overlay closes entirely; the chat input regains focus; no model change was made

### Steps (from Level 1 with active search)

1. User is at Level 1 (or flat search) with a search query active
2. User presses Esc → The search query is cleared; the view returns to the Level 1 model list for the active provider (or Level 0 if no provider was active); the cursor resets
3. User presses Esc again → Returns to Level 0 (if at Level 1) or closes overlay (if already at Level 0)

### Variations

- **Esc at Level 0 with search**: If the user is at Level 0 (provider list) with a search query, pressing Esc clears the search first; a second Esc closes the overlay
- **Esc from /keys**: In the `/keys` overlay, Esc exits key input mode first (if active), then closes the overlay on the second press — the same double-Esc priority pattern applies

### Edge Cases

- **Search cleared on Esc at Level 1 exit**: When Esc transitions from Level 1 back to Level 0, any active search query is cleared; the provider cursor is repositioned to the provider that was active at Level 1
- **No-op Esc when no overlay is open**: If the user presses Esc with no overlay open and no active run, the input is cleared if it has content, otherwise nothing happens (no error)
