import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

@Suite("PromptHistory cursor navigation")
struct PromptHistoryTests {

    @Test("Up walks backwards through recorded prompts, newest first")
    func recallsBackwardsFromNewest() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")
        history.record("c")

        #expect(history.recallPrevious(currentDraft: "") == "c")
        #expect(history.recallPrevious(currentDraft: "") == "b")
        #expect(history.recallPrevious(currentDraft: "") == "a")
    }

    @Test("Up at the oldest entry stays there -- no wraparound, no nil-after-start")
    func staysAtOldestEntry() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")
        history.record("c")

        _ = history.recallPrevious(currentDraft: "")
        _ = history.recallPrevious(currentDraft: "")
        _ = history.recallPrevious(currentDraft: "")
        #expect(history.recallPrevious(currentDraft: "") == "a")
    }

    @Test("Down walks forward and back out to the stashed draft")
    func recallsForwardToStashedDraft() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")
        history.record("c")

        _ = history.recallPrevious(currentDraft: "")
        _ = history.recallPrevious(currentDraft: "")
        _ = history.recallPrevious(currentDraft: "")
        // now at "a"

        #expect(history.recallNext() == "b")
        #expect(history.recallNext() == "c")
        #expect(history.recallNext() == "")
        #expect(history.recallNext() == nil)
    }

    @Test(
        "core regression: Up on a half-typed draft declines and leaves the cursor unmoved"
    )
    func declinesToClobberAnInProgressDraft() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")

        #expect(history.recallPrevious(currentDraft: "half-typed thought") == nil)
        // The cursor never moved, so a subsequent Up from an empty draft still
        // starts from the newest entry -- proof the decline did not silently
        // advance navigation state.
        #expect(history.recallPrevious(currentDraft: "") == "b")
    }

    @Test("Down past the newest restores the exact stashed draft, including an empty one")
    func restoresStashedDraftExactly() {
        var history = PromptHistory()
        history.record("a")

        _ = history.recallPrevious(currentDraft: "")
        #expect(history.recallNext() == "")
    }

    @Test("recording while navigating resets the cursor so the next Up starts from the newest")
    func recordingResetsNavigation() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")

        _ = history.recallPrevious(currentDraft: "")
        _ = history.recallPrevious(currentDraft: "")
        // now at "a"; recording a new prompt mid-navigation must reset
        history.record("c")

        #expect(history.recallPrevious(currentDraft: "") == "c")
    }

    @Test("empty history declines without crashing")
    func emptyHistoryDeclines() {
        var history = PromptHistory()
        #expect(history.recallPrevious(currentDraft: "") == nil)
    }

    /// KTD-7's approximation of caret-awareness: since this package cannot
    /// read the text caret (macOS 14 floor, `TextSelection` needs 15), Up
    /// must still never silently replace an edit made to an already-recalled
    /// entry -- it declines and leaves the cursor exactly where it was, so a
    /// later Up (once the draft matches again) resumes normally.
    @Test("regression: Up declines without clobbering once the recalled entry has been edited")
    func declinesWhenRecalledEntryHasBeenEdited() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")

        #expect(history.recallPrevious(currentDraft: "") == "b")
        #expect(history.recallPrevious(currentDraft: "b, but edited") == nil)
        #expect(history.recallPrevious(currentDraft: "b") == "a")
    }

    /// Exercises the fix for #995 (F7): repeating Up once already at the
    /// oldest entry used to keep returning that same string. Reassigning
    /// `draft` to a value it already holds never fires SwiftUI's `onChange`,
    /// which left the composer's `isRecallingHistory` flag stuck `true` --
    /// the next real keystroke was then misattributed to the recall instead
    /// of calling `noteManualDraftEdit()`, so a later Down silently clobbered
    /// the user's edit. Declining (nil) instead of repeating the same value
    /// is what lets the composer tell "nothing happened" apart from "the
    /// same entry was recalled again".
    @Test(
        "regression: Up again at the oldest entry, once the draft already shows it, declines instead of repeating a same-value no-op"
    )
    func staysAtOldestEntryDeclinesNoOpRepeat() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")

        #expect(history.recallPrevious(currentDraft: "") == "b")
        #expect(history.recallPrevious(currentDraft: "b") == "a")
        // Now at the oldest entry ("a") and the composer's draft already
        // shows it -- Up again must decline, not return "a" again.
        #expect(history.recallPrevious(currentDraft: "a") == nil)
    }

    @Test("regression: duplicate consecutive prompts are both recorded, not deduped")
    func duplicatePromptsAreNotDeduped() {
        var history = PromptHistory()
        history.record("same")
        history.record("same")

        #expect(history.recallPrevious(currentDraft: "") == "same")
        // The second entry is still recalled even though it has the same
        // string as the first and therefore will not trigger SwiftUI's
        // `onChange` when assigned to the text field.
        #expect(history.recallPrevious(currentDraft: "same") == "same")
        // The cursor advanced through both identical entries, rather than
        // deduping them or remaining on the newest one.
        #expect(history.recallNext() == "same")
        #expect(history.recallNext() == "")
    }

    @Test("reset clears navigation state without touching recorded entries")
    func resetClearsNavigation() {
        var history = PromptHistory()
        history.record("a")
        history.record("b")

        _ = history.recallPrevious(currentDraft: "")
        history.reset()

        #expect(history.recallPrevious(currentDraft: "") == "b")
    }

    @Test("reachability: ChatView wires Up/Down to prompt-history recall in production")
    func chatViewWiresArrowKeysToRecall() throws {
        let chatViewURL = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI/ChatView.swift")
        let source = try String(contentsOf: chatViewURL, encoding: .utf8)

        #expect(source.contains(".onKeyPress(.upArrow"))
        #expect(source.contains(".onKeyPress(.downArrow"))
        #expect(source.contains("isRecallingHistory = run.draft != draftBeforeRecall"))
        #expect(
            source.components(
                separatedBy:
                    "isRecallingHistory = false\n                                return .ignored"
            ).count - 1 == 2,
            "both declined Up and declined Down must clear recall suppression before returning")
    }
}

/// Exercises the RunSession/composer wiring that sits outside PromptHistory
/// itself: `submit()` recording, and `noteManualDraftEdit()` -- which the
/// composer's key handlers and `onChange` cooperate to call -- ending
/// navigation so the *next* arrow press cannot silently replace an edit.
/// None of these calls touch the network, so a plain client pointed at an
/// address nothing is listening on is enough (mirrors `RunControlAckTests`'
/// construction, minus its stub since these paths never dial out).
@Suite("RunSession prompt-history wiring")
@MainActor
struct RunSessionPromptHistoryWiringTests {

    private func makeSession() -> RunSession {
        RunSession(baseURL: URL(string: "http://127.0.0.1:0")!)
    }

    @Test("submit() records the prompt so a later Up recalls it")
    func submitRecordsPrompt() {
        let session = makeSession()
        session.draft = "first prompt"
        session.submit()

        #expect(session.recallPreviousPrompt())
        #expect(session.draft == "first prompt")
    }

    @Test("recallPreviousPrompt declines and returns false when history is empty")
    func recallPreviousDeclinesOnEmptyHistory() {
        let session = makeSession()
        #expect(session.recallPreviousPrompt() == false)
        #expect(session.draft.isEmpty)
    }

    @Test(
        "regression: noteManualDraftEdit resets navigation so Down cannot silently overwrite an edit made after a recall"
    )
    func manualEditAfterRecallResetsNavigation() {
        // A single entry is deliberate: `submit()` optimistically marks the
        // run `.queued` (`Transcript.appendUserPrompt`), so a second
        // synchronous `submit()` in the same test would be silently blocked
        // by `canSubmit`'s `isBusy` guard before ever reaching this method.
        let session = makeSession()
        session.draft = "first prompt"
        session.submit()

        #expect(session.recallPreviousPrompt())
        #expect(session.draft == "first prompt")

        // Simulates the composer's onChange firing for a manual keystroke,
        // not the recall that just happened.
        session.draft = "first prompt, but edited"
        session.noteManualDraftEdit()

        // `recallNext` (unlike `recallPrevious`) takes no draft parameter
        // to compare against -- per the plan's `PromptHistory` signature --
        // so without the reset it would still see itself "at" the one
        // recalled entry and restore the pre-recall stash (empty string),
        // silently erasing the edit. The reset ends navigation first, so
        // Down correctly declines instead.
        #expect(session.recallNextPrompt() == false)
        #expect(session.draft == "first prompt, but edited")
    }
}
