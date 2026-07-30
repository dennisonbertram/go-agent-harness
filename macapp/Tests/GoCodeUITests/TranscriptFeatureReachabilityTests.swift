import Foundation
import Testing

@testable import GoCodeUI

@Suite("Transcript feature reachability")
struct TranscriptFeatureReachabilityTests {

    @Test("usage and whole-conversation copy retain production call sites")
    func featuresHaveProductionCallSites() throws {
        let source = try ReachabilitySource.wholeModule()

        #expect(source.contains("UsageLabel(usage: usage)"))
        #expect(source.contains("TranscriptText.plain(items)"))
    }

    /// Scans the whole module rather than named files. These treatments are
    /// meant to exist *somewhere* in the UI; pinning them to AppShell.swift
    /// made the test fail the moment the sidebar was extracted into its own
    /// file, which is a refactor, not a regression. Module-wide still catches
    /// the thing worth catching — a treatment being dropped entirely.
    @Test("rail selection and user prompts retain their semantic layout tokens")
    func transcriptAndRailUseSemanticTokens() throws {
        let source = try ReachabilitySource.wholeModule()

        #expect(source.contains("Theme.selectedRowSurface"))
        #expect(source.contains("Theme.selectedRowForeground"))
        #expect(source.contains("Theme.surface.ignoresSafeArea(.container, edges: .top)"))
        #expect(source.contains("Layout.userMessageMaximumWidth"))
        #expect(source.contains(".foregroundStyle(Theme.foreground)"))
    }

    /// The transcript once rendered at the macOS 13pt system default because
    /// its message views set no font at all. Every token test still passed —
    /// the scale was correct and simply unread — so raising the type scale
    /// changed nothing on screen. These assert the transcript is actually
    /// wired to the scale, which is the part that silently broke.
    @Test("transcript prose is bound to the shared type scale")
    func transcriptConsumesTypeScale() throws {
        let chatView = try ReachabilitySource.file("ChatView.swift")

        #expect(chatView.contains(".font(Typography.body)"))
        #expect(chatView.contains(".lineSpacing(Typography.bodyLineSpacing)"))
    }

    @Test("transcript leading resolves to the reference line pitch")
    func lineSpacingMatchesReferencePitch() {
        #expect(Typography.bodyLineSpacing == Typography.bodyLinePitch - Typography.bodyLineHeight)
        #expect(Typography.bodyLinePitch == 26.5)
    }

    /// #992's finding was a pure model with no call site: `pinnedToBottom` was
    /// declared and read but never mutated by scroll geometry. This pins the
    /// wiring, not just the value type's own tests.
    @Test("transcript autoscroll is actually wired to a live scroll pin")
    func autoscrollPinIsWiredToScrollGeometry() throws {
        let chatView = try ReachabilitySource.file("ChatView.swift")

        #expect(chatView.contains("pin.update(distanceFromBottom:"))
        #expect(chatView.contains("guard pin.isPinned"))
    }

    /// Distinct from the wiring test above: that one only proves the *consumer*
    /// side (`pin.update`/`guard pin.isPinned`) is present, which a stray
    /// hardcoded distance would still satisfy textually. This proves the
    /// *feed* is real scroll geometry — a named coordinate space plus a
    /// preference key carrying the anchor's frame — so a change that keeps
    /// the call site but rips out the geometry plumbing underneath it (e.g.
    /// replacing the fed value with a constant) is still caught.
    @Test("the scroll pin is fed by a real geometry preference, not a placeholder value")
    func autoscrollPinIsFedByLiveGeometry() throws {
        let chatView = try ReachabilitySource.file("ChatView.swift")

        #expect(chatView.contains(".coordinateSpace(name: scrollSpace)"))
        #expect(chatView.contains("TranscriptBottomAnchorKey"))
        #expect(chatView.contains(".onPreferenceChange(TranscriptBottomAnchorKey.self)"))
    }

    /// #994's finding (R4) was that `AskUserView`'s `Send` button used
    /// `answers.count < prompt.questions.count`, which counts a field typed
    /// into and then cleared back to `""` as answered. `AskUserAnswersTests`
    /// pins the predicate's own logic but does not prove the view still
    /// calls it -- a revert that reintroduces the count comparison while
    /// leaving `AskUserAnswers.swift` itself untouched (and passing) would
    /// slip through that suite alone. This pins the call site.
    @Test("AskUserView's Send button is gated by the shared completeness predicate")
    func askUserViewUsesSharedCompletenessPredicate() throws {
        let chatView = try ReachabilitySource.file("ChatView.swift")

        #expect(chatView.contains("AskUserAnswers.isComplete(prompt: prompt, answers: answers)"))
        #expect(!chatView.contains("answers.count < prompt.questions.count"))
    }

    /// Exercises the fix for #995 (F1a): `RunSession.answer()` gained an
    /// `answerInFlight` guard so a second call while the first is still
    /// awaiting the server is a no-op (`RunControlAckTests` proves that at
    /// the model level). The composer's own Send button must reflect the
    /// same in-flight state, or an impatient double-click still reads as
    /// "nothing happened" instead of "still sending".
    @Test("AskUserView's Send button is disabled while an answer is in flight")
    func askUserViewSendDisabledWhileAnswerInFlight() throws {
        let chatView = try ReachabilitySource.file("ChatView.swift")

        // Two substrings rather than one exact multi-line `.disabled(...)`
        // string: swift-format may re-wrap the expression across lines, and
        // this must keep matching either shape.
        #expect(chatView.contains("let answerInFlight: Bool"))
        #expect(
            chatView.contains(
                "!AskUserAnswers.isComplete(prompt: prompt, answers: answers)\n"))
        #expect(chatView.contains("|| answerInFlight)"))
    }

    /// #994's finding (R3) was that `RunSession.cancel/approve/deny/answer`
    /// discarded the server's acknowledgement with `try? await client....`.
    /// `RunControlAckTests` proves each method surfaces a failure through a
    /// live stub, but a partial revert of just one call site back to `try?`
    /// -- while the other three, and this source-scan, stay untouched --
    /// would otherwise only be caught if that one method happened to be
    /// re-run; this pins the absence of the bug shape across the whole file
    /// in one assertion.
    @Test("run-control calls no longer discard their acknowledgement with try?")
    func runControlCallsDoNotDiscardAcknowledgement() throws {
        let runSession = try ReachabilitySource.file("RunSession.swift")

        #expect(!runSession.contains("try? await client.cancel"))
        #expect(!runSession.contains("try? await client.approve"))
        #expect(!runSession.contains("try? await client.deny"))
        #expect(!runSession.contains("try? await client.answerInput"))
    }

    /// Exercises the fix for #995 (F8): the geometry reader backing
    /// `TranscriptBottomAnchorKey` fires on *every* frame of the `scrollTo`
    /// animation `scrollIfPinned` starts, not just its final frame -- so
    /// `pin.update` used to see the anchor still mid-flight, far from the
    /// viewport bottom, and unpin autoscroll from the very scroll it had just
    /// triggered. `TranscriptScrollPin` itself stays a pure decision (it has
    /// no notion of "an animation is in flight"); the suppression has to live
    /// in the view that knows when it started one.
    @Test(
        "the scroll pin ignores geometry updates while its own scrollTo animation is in flight, and before a real viewport height is known"
    )
    func pinUpdateSuppressedDuringOwnAnimation() throws {
        let chatView = try ReachabilitySource.file("ChatView.swift")

        #expect(
            chatView.contains("isAutoScrolling"),
            "no view-layer flag guards pin.update against its own scrollTo animation")
        #expect(
            chatView.contains("guard !isAutoScrolling, scrollViewportHeight > 0 else { return }"),
            "pin.update must be skipped both mid-animation and before the viewport reports a real height"
        )
        #expect(
            chatView.contains("isAutoScrolling = true"),
            "scrollIfPinned must raise the flag before starting its scrollTo animation")

        let pin = try ReachabilitySource.file("TranscriptScrollPin.swift")
        #expect(
            !pin.contains("isAutoScrolling"),
            "TranscriptScrollPin must stay a pure decision with no view-layer animation state")
    }
}
