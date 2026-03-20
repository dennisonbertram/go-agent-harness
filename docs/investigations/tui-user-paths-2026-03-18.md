# TUI User Paths — 20 Real-World Multi-Step Scenarios

**Date**: 2026-03-18
**Purpose**: Comprehensive UX/functionality testing of the BubbleTea TUI

---

## Path 1: First-Time Setup with OpenRouter
**Goal**: Brand new user configures OpenRouter as their gateway and runs first task.

1. Launch TUI: `./harnesscli --tui`
2. Observe initial state — verify status bar shows a default model, input area is active
3. Type `/keys` and press Enter
4. Navigate to OpenRouter provider with arrow keys
5. Press Enter to enter key input mode
6. Type the OpenRouter API key: `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`
7. Press Enter to submit the key
8. Observe success status message in footer
9. Press Escape to close /keys overlay
10. Type `/provider` and press Enter
11. Navigate to "OpenRouter" option
12. Press Enter to select it
13. Observe status bar updates with "↗OR" indicator
14. Type a simple message: "Hello, what's 2+2?"
15. Press Enter to submit
16. Observe streaming response from model via OpenRouter
17. Observe cost and token counts update in status bar

---

## Path 2: Model Switching Mid-Conversation
**Goal**: User switches models during active conversation, verifying conversation continuity.

1. Launch TUI (server must be running)
2. Type "What is the capital of France?" and press Enter
3. Wait for complete response
4. Observe conversationID is established
5. Type `/model` to open model switcher
6. Navigate through models list (3-4 presses of down arrow)
7. Type "gpt" in search box to filter models
8. Select GPT-4.1 with Enter
9. Observe Level-1 config panel opens
10. Press Enter again to confirm (no changes needed if key configured)
11. Observe status bar model name updates
12. Type "What did I just ask you?" (testing if conversation context continues)
13. Press Enter
14. Observe response — model may not remember (new model context, normal)
15. Type `/stats` to see session usage
16. Press Escape to close stats

---

## Path 3: Deep Stats and Export Workflow
**Goal**: Run multiple conversations, inspect stats, export transcript.

1. Launch TUI
2. Run first message: "Explain what a binary search tree is in one sentence"
3. Wait for response
4. Run second message: "Now explain a red-black tree"
5. Wait for response
6. Run third message: "What's the difference between them?"
7. Wait for response
8. Type `/stats` to open stats panel
9. Observe daily cost/token chart shows today's usage
10. Press Escape to close stats
11. Type `/context` to open context grid
12. Observe token allocation breakdown
13. Press Escape to close
14. Type `/export` to export transcript
15. Verify success message in status bar
16. Press Escape or Enter if needed
17. Check that transcript file was created in current directory

---

## Path 4: Reasoning Model Configuration (High Effort)
**Goal**: Configure DeepSeek Reasoner with high reasoning effort via /model Level-1.

1. Launch TUI
2. Type `/model` and press Enter
3. In search box, type "deepseek" to filter
4. Navigate to DeepSeek Reasoner entry
5. Press Enter to enter Level-1 config panel
6. Navigate to reasoning section with down arrow
7. Press `l` (right) to set reasoning effort to "high"
8. Observe reasoning effort changes to "high" in panel
9. Press Enter to confirm
10. Observe status bar shows "(high)" next to model name
11. Type a complex reasoning question: "What is the 17th prime number? Show your work."
12. Press Enter
13. Wait for thinking/reasoning response
14. Observe streaming response
15. Type `/model` again
16. Navigate to Level-1 config panel for this model
17. Check reasoning effort still shows "high" (verify persistence)
18. Press Escape to cancel, don't change
19. Press Escape again to close overlay

---

## Path 5: API Key Management Full Lifecycle
**Goal**: Add, verify, and test multiple provider API keys.

1. Launch TUI
2. Type `/keys` and press Enter
3. Observe provider list — note which have keys configured
4. Navigate to Anthropic provider
5. Press Enter to enter key input mode
6. Observe input field appears
7. Type the Anthropic API key from environment
8. Press Enter to submit
9. Observe success status message
10. Navigate to OpenAI provider
11. Press Enter to enter key input mode
12. Press Ctrl+U to clear any existing input
13. Type OpenAI API key
14. Press Enter to submit
15. Observe success message
16. Navigate to OpenRouter provider
17. Press Enter to input OpenRouter key
18. Type: `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`
19. Press Enter to submit
20. Press Escape to close /keys overlay
21. Verify all models now show as available in /model switcher

---

## Path 6: Multi-Turn Conversation with Tool Use
**Goal**: Run a long multi-turn conversation that exercises tool calls.

1. Launch TUI
2. Type "List the 5 most recently modified files in /tmp" and press Enter
3. Wait for response (tool use expected — bash/filesystem tools)
4. Observe tool call blocks appear in viewport (⏺ toolname...)
5. Observe tool completion markers (✓)
6. After response, type "How large are those files?"
7. Wait for response with more tool use
8. Type "Ctrl+O" on a visible tool call block to expand it
9. Observe expanded tool call details
10. Type "Ctrl+O" again to collapse it
11. Type "Create a temporary test file /tmp/harness-test-$(date +%s).txt with content 'hello'"
12. Wait for tool execution and response
13. Type "Verify the file was created and show its contents"
14. Wait for response
15. Type `/export` to export this conversation

---

## Path 7: Help System Exploration
**Goal**: Thoroughly navigate the help system and verify all content is accurate.

1. Launch TUI
2. Type `/help` and press Enter
3. Observe help overlay opens
4. Read through command list — verify all 9 commands listed
5. Scroll down in help if scrollable
6. Press `?` key to close (same as Ctrl+H)
7. Press `?` key again to reopen help
8. Observe keyboard shortcuts section
9. Verify Ctrl+C, Esc, Shift+Enter are documented
10. Press Ctrl+H to close
11. Press `/` in input field to trigger slash autocomplete
12. Observe dropdown with command options appears
13. Navigate dropdown with arrow keys
14. Press Tab to complete a command
15. Press Escape to dismiss autocomplete
16. Type `?` as first character of input (NOT as standalone keybinding)
17. Verify `?` appears in input field (not triggering help)
18. Clear input with Escape

---

## Path 8: Context Window Management
**Goal**: Test context display and scrolling with a long conversation.

1. Launch TUI
2. Run a long message: "Write a complete implementation of a merge sort algorithm in Go with documentation and tests. Be thorough."
3. Wait for complete response (will be long)
4. Observe viewport auto-scrolls to bottom
5. Press Up arrow to scroll up in viewport
6. Press PgUp to scroll up a page
7. Press Down to scroll back down
8. Type `/context` to open context grid
9. Observe system prompt tokens shown
10. Observe conversation tokens (should be significant after long response)
11. Observe remaining capacity percentage
12. Press Escape to close context grid
13. Run second long message: "Now write the same for quicksort with generics"
14. Wait for response
15. Open `/context` again and compare token counts
16. Run third long message to push towards context limits
17. Observe how TUI handles near-context-limit state

---

## Path 9: Interrupt and Resume Workflow
**Goal**: Test cancellation of in-flight runs and starting new ones.

1. Launch TUI
2. Type "Count from 1 to 100 slowly, outputting each number with a 50ms pause" and press Enter
3. Observe run starts, streaming begins
4. While run is active, press Ctrl+C
5. Observe interrupt confirmation UI appears
6. Read the interrupt UI message (verify it makes sense)
7. Press Ctrl+C again to confirm interrupt
8. Observe "Stopping..." message
9. Observe run terminates and runActive = false
10. Verify input area re-enables
11. Type new message: "Start over — just say hello"
12. Press Enter
13. Observe new run starts successfully (same conversation)
14. Wait for complete response
15. Verify response is coherent

---

## Path 10: Gateway Switching Direct vs OpenRouter
**Goal**: Compare behavior with Direct and OpenRouter routing.

1. Launch TUI
2. Ensure status bar shows current gateway (no ↗OR = Direct)
3. Type "What model are you?" and press Enter
4. Note the response/model identity
5. Type `/provider` and press Enter
6. Navigate to "OpenRouter" and press Enter
7. Observe ↗OR appears in status bar
8. Type "What model are you?" and press Enter again
9. Note any differences in response format/identity
10. Type `/provider` again
11. Switch back to "Direct"
12. Observe ↗OR disappears from status bar
13. Run another message to verify direct routing works
14. Check `/stats` to see if costs tracked correctly for both routings
15. Press Escape to close stats

---

## Path 11: Slash Command Autocomplete UX
**Goal**: Test the slash command autocomplete dropdown thoroughly.

1. Launch TUI
2. Type `/` in input field
3. Observe autocomplete dropdown appears immediately
4. Count the number of options shown (should be all 9 commands)
5. Press down arrow to navigate to second item
6. Press up arrow to go back to first
7. Press Tab to complete selected command
8. Observe command is filled in input
9. Press Escape to clear input
10. Type `/mo` (partial match)
11. Observe dropdown filters to only `/model`
12. Press Enter to accept
13. Observe /model overlay opens
14. Press Escape to close
15. Type `/x` (no match)
16. Observe dropdown shows no matches or "no commands found"
17. Press Escape to clear
18. Type `/quit` and press Enter (tests quit path — may need restart)

---

## Path 12: Multi-Line Input Workflow
**Goal**: Test multi-line message composition and submission.

1. Launch TUI
2. Type first line: "Please analyze this code:"
3. Press Shift+Enter to insert newline (NOT submit)
4. Type second line: "```go"
5. Press Shift+Enter
6. Type: "func main() {"
7. Press Shift+Enter
8. Type: `    fmt.Println("hello")`
9. Press Shift+Enter
10. Type: "}"
11. Press Shift+Enter
12. Type: "```"
13. Observe full multi-line input in input area
14. Press Enter to submit entire message
15. Wait for analysis response
16. Verify the server received the complete multi-line message
17. Observe markdown code block rendered in response

---

## Path 13: Model Search and Star/Unstar
**Goal**: Test model search filtering and starring functionality.

1. Launch TUI
2. Type `/model` and press Enter
3. Count total number of models visible in list
4. Type "claude" in search box
5. Observe list filters to Anthropic Claude models only
6. Press Escape to clear search
7. Type "grok" in search box
8. Observe Grok models appear
9. Clear search with Escape
10. Navigate to any unstarred model
11. Press `s` or star keybinding (if any)
12. Observe model gets starred/bookmarked indicator
13. Clear search and look for starred models at top (if that's the behavior)
14. Navigate to a currently configured model
15. Press Enter to open Level-1 config
16. Navigate through all sections with arrow keys
17. Press Escape to go back to Level-0
18. Press Escape again to close overlay

---

## Path 14: Subagents Inspection
**Goal**: Run a task that spawns subagents, then inspect via /subagents.

1. Launch TUI
2. Type: "Use a bash tool to find all .go files in /tmp and count them"
3. Press Enter
4. Wait for response
5. Type: "Use multiple steps to: 1) create a temp dir, 2) create 3 test files in it, 3) list them"
6. Press Enter
7. While/after run, type `/subagents`
8. Observe subagent list
9. Press Escape to close
10. Type: "What is the weather like?" (test graceful failure for unavailable tools)
11. Wait for response
12. Observe how model handles unavailable info
13. Type `/stats` to check cumulative costs
14. Verify costs are tracked correctly
15. Press Escape

---

## Path 15: Keyboard-Only Navigation (Pure Keyboard Flow)
**Goal**: Complete entire session using only keyboard, no mouse.

1. Launch TUI via keyboard shortcut in terminal
2. Verify cursor is in input area by default
3. Use Ctrl+H to open help
4. Navigate help content with j/k keys
5. Close with Escape
6. Type a message using only keyboard
7. Use Shift+Enter for multi-line
8. Submit with Enter
9. Wait for response; use Up/Down to scroll viewport
10. Use PgUp/PgDn to navigate long responses
11. Open `/model` via keyboard
12. Navigate with j/k
13. Press Enter to open Level-1
14. Navigate sections with Up/Down
15. Press Escape twice to close
16. Use Ctrl+S to copy last response
17. Verify clipboard confirmation in status bar
18. Use Ctrl+C to exit (if no active run)

---

## Path 16: Status Bar Verification
**Goal**: Verify all status bar information displays correctly.

1. Launch TUI fresh
2. Observe initial status bar state (model, cost: $0.00, tokens: 0)
3. Run a simple message: "Hi"
4. Observe token count updates after response
5. Observe cost updates (should be small but non-zero)
6. Switch model via `/model`
7. Verify status bar model name changes
8. Switch to OpenRouter via `/provider`
9. Verify ↗OR indicator appears
10. Switch back to Direct
11. Verify ↗OR disappears
12. Run another message
13. Observe tokens accumulate across model switches
14. Type `/stats` to see detailed breakdown
15. Cross-check stats panel numbers with status bar totals
16. Press Escape to close stats

---

## Path 17: Error Recovery and Edge Cases
**Goal**: Test error handling and recovery from various error states.

1. Launch TUI
2. Try submitting empty message (just press Enter with empty input)
3. Verify TUI does NOT start a run on empty input (or shows appropriate feedback)
4. Type `/model` and select a model whose API key is NOT configured
5. Try to submit a message with unconfigured model
6. Observe error handling (graceful failure vs crash)
7. Press Escape if overlay appears
8. Select a properly configured model
9. Type a very long message (500+ characters)
10. Verify input handles it gracefully (no overflow, scroll works)
11. Submit the long message
12. Wait for response
13. Type `/model` with a malformed search (special chars: "!@#$%")
14. Verify search handles special characters without panic
15. Press Escape to close
16. Test Esc key from various states (input with text, overlay open, nothing active)

---

## Path 18: Config Persistence Across Restarts
**Goal**: Verify TUI configuration persists after quitting and restarting.

1. Launch TUI
2. Use `/model` to select GPT-4.1
3. Configure reasoning if applicable
4. Use `/provider` to set OpenRouter
5. Verify status bar shows the configuration
6. Type `/quit` to exit TUI
7. Re-launch: `./harnesscli --tui`
8. Observe initial state — verify selected model is restored
9. Verify ↗OR gateway indicator is restored
10. Open `/model` to confirm model selection persisted
11. Open `/keys` to verify key configurations persisted
12. Press Escape
13. Run a message to confirm everything works after restart
14. Type `/stats` to verify fresh stats for new session
15. Verify previous session's costs are NOT carried over (fresh session)

---

## Path 19: Transcript Export Format Validation
**Goal**: Export transcript and verify its format and content are correct.

1. Launch TUI
2. Run message: "My name is TestUser and I'm debugging the TUI"
3. Wait for response
4. Run message: "What did I say my name was?"
5. Wait for response (should say TestUser)
6. Run message: "Calculate 42 * 17"
7. Wait for response
8. Type `/export` and press Enter
9. Observe success message with filename
10. Note the filename (timestamp format)
11. Press Escape or Enter to dismiss
12. Open the exported file in another terminal/window
13. Verify it contains: user messages, assistant messages, timestamps
14. Verify markdown format is valid
15. Verify all 3 exchanges are captured (6 messages total)
16. Re-run `/export` immediately again
17. Verify second file is created with different timestamp
18. Verify both files have same content

---

## Path 20: Full Feature Integration Test
**Goal**: Exercise all major features in one comprehensive session.

1. Launch TUI
2. Type `/help` → scroll through → Escape
3. Type `/keys` → add OpenRouter key → Escape
4. Type `/provider` → select OpenRouter → Enter
5. Type `/model` → select Claude Sonnet 4.6 → configure → Enter
6. Type multi-line code analysis message with Shift+Enter
7. Submit with Enter → wait for response
8. Observe tool calls if any, expand with Ctrl+O
9. Type `/context` → verify token count → Escape
10. Type `/stats` → verify cost tracking → Escape
11. Switch model mid-session via `/model`
12. Type another message → wait for response
13. Type `/subagents` → inspect → Escape
14. Type `/export` → save transcript
15. Verify all features worked without errors
16. Final check: status bar accurate, input responsive, no visual glitches
17. Type `/quit` to exit cleanly

---

## Bug Tracking

| # | Path | Step | Bug Description | Severity | Fixed? |
|---|------|------|-----------------|----------|--------|
| | | | | | |

---

## Test Results Summary

| Path | Status | Issues Found |
|------|--------|-------------|
| 1 | PENDING | — |
| 2 | PENDING | — |
| ... | ... | ... |
