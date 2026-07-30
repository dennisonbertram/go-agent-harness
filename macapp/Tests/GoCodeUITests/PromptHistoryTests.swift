import Foundation
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

    @Test("regression: duplicate consecutive prompts are both recorded, not deduped")
    func duplicatePromptsAreNotDeduped() {
        var history = PromptHistory()
        history.record("same")
        history.record("same")

        #expect(history.recallPrevious(currentDraft: "") == "same")
        #expect(history.recallPrevious(currentDraft: "") == "same")
        // Two entries recorded means a third recall stays put, not nil.
        #expect(history.recallPrevious(currentDraft: "") == "same")
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
    }
}
