import Foundation
import Testing

@testable import GoCodeUI

@Suite("Transcript feature reachability")
struct TranscriptFeatureReachabilityTests {

    @Test("usage and whole-conversation copy retain production call sites")
    func featuresHaveProductionCallSites() throws {
        let sourceDirectory = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
        let source = try FileManager.default
            .contentsOfDirectory(at: sourceDirectory, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "swift" }
            .map { try String(contentsOf: $0, encoding: .utf8) }
            .joined(separator: "\n")

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
        let sourceDirectory = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
        let source = try FileManager.default
            .contentsOfDirectory(at: sourceDirectory, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "swift" }
            .map { try String(contentsOf: $0, encoding: .utf8) }
            .joined(separator: "\n")

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
        let chatView = try String(
            contentsOf: URL(filePath: #filePath)
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appending(path: "Sources/GoCodeUI/ChatView.swift"),
            encoding: .utf8)

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
        let chatView = try String(
            contentsOf: URL(filePath: #filePath)
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appending(path: "Sources/GoCodeUI/ChatView.swift"),
            encoding: .utf8)

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
        let chatView = try String(
            contentsOf: URL(filePath: #filePath)
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appending(path: "Sources/GoCodeUI/ChatView.swift"),
            encoding: .utf8)

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
        let chatView = try String(
            contentsOf: URL(filePath: #filePath)
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appending(path: "Sources/GoCodeUI/ChatView.swift"),
            encoding: .utf8)

        #expect(chatView.contains("AskUserAnswers.isComplete(prompt: prompt, answers: answers)"))
        #expect(!chatView.contains("answers.count < prompt.questions.count"))
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
        let runSession = try String(
            contentsOf: URL(filePath: #filePath)
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appending(path: "Sources/GoCodeUI/RunSession.swift"),
            encoding: .utf8)

        #expect(!runSession.contains("try? await client.cancel"))
        #expect(!runSession.contains("try? await client.approve"))
        #expect(!runSession.contains("try? await client.deny"))
        #expect(!runSession.contains("try? await client.answerInput"))
    }
}
