import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Exercises the fix for #996 (F5, R6): delete and undo fired immediately
/// with no statement of what would be lost. `DeletePreview` / `UndoPreview`
/// are pure, client-derived preview builders (KTD-5 -- there is no server
/// dry-run) reused by every destructive entry point through the shared
/// `DestructiveConfirmation` presentation (KTD-4).
@Suite("Destructive confirmation previews")
struct DestructiveConfirmationTests {

    // MARK: - DeletePreview

    @Test("a delete preview names the conversation title and a known message count")
    func deletePreviewNamesKnownCount() throws {
        let conversation = try makeConversation(title: "Fix the parser", messageCount: 12)
        let message = DeletePreview.message(for: conversation)
        #expect(message.contains("Fix the parser"))
        #expect(message.contains("12"))
    }

    /// Core regression: no fabricated count. `ConversationInfo.messageCount`
    /// is `nil` whenever the server omitted it, and the preview must say so
    /// plainly rather than inventing a number.
    @Test("a delete preview with an unknown message count never invents a number")
    func deletePreviewNeverInventsCount() throws {
        let conversation = try makeConversation(title: "Untitled conversation", messageCount: nil)
        let message = DeletePreview.message(for: conversation)
        #expect(message.contains("unknown"))
        #expect(!message.contains(where: { $0.isNumber }))
    }

    @Test("a very long conversation title is truncated to a bounded length")
    func deletePreviewTruncatesLongTitles() throws {
        let conversation = try makeConversation(
            title: String(repeating: "a", count: 500), messageCount: 3)
        let message = DeletePreview.message(for: conversation)
        #expect(message.count < 200)
    }

    // MARK: - UndoPreview

    @Test("an undo preview quotes the last prompt")
    func undoPreviewQuotesLastPrompt() {
        let message = UndoPreview.message(lastPrompt: "fix the parser")
        #expect(message.contains("fix the parser"))
    }

    /// When the transcript holds no prior user prompt, the wording must stay
    /// neutral rather than quoting an empty string.
    @Test("an undo preview with no prior prompt reads neutrally")
    func undoPreviewWithNoPromptIsNeutral() {
        let message = UndoPreview.message(lastPrompt: nil)
        #expect(message.contains("the last turn"))
        #expect(!message.contains("\"\""))
    }

    @Test("an undo preview flattens a multi-line prompt to one line")
    func undoPreviewFlattensMultilinePrompt() {
        let message = UndoPreview.message(lastPrompt: "line one\nline two")
        #expect(!message.contains("\n"))
        #expect(message.contains("line one line two"))
    }

    @Test("lastUserPrompt finds the most recent user prompt in a transcript")
    func lastUserPromptFindsMostRecent() {
        var transcript = Transcript()
        transcript.appendUserPrompt("first")
        transcript.appendUserPrompt("second")
        #expect(UndoPreview.lastUserPrompt(in: transcript.items) == "second")
    }

    @Test("lastUserPrompt returns nil for a transcript with no user prompts")
    func lastUserPromptReturnsNilWithoutOne() {
        let transcript = Transcript()
        #expect(UndoPreview.lastUserPrompt(in: transcript.items) == nil)
    }

    // MARK: - Reachability (KTD-4 wiring)

    /// #996's finding was that delete and undo fired the moment the button
    /// was tapped. This pins that the immediate-action shape is gone from
    /// the whole module and that the shared confirmation is actually used,
    /// not merely that `DeletePreview`/`UndoPreview` exist unreferenced.
    @Test("delete and undo route through the shared destructive confirmation, not an immediate action")
    func destructiveActionsRouteThroughSharedConfirmation() throws {
        let source = try sourceDirectory()
        #expect(!source.contains("Button(\"Delete\", role: .destructive) { delete("))
        #expect(source.contains("destructiveConfirmation("))
    }

    // MARK: - Helpers

    private func makeConversation(title: String, messageCount: Int?) throws -> ConversationInfo {
        let json = """
            {"id": "c1", "title": "\(title)", "message_count": \(messageCount.map(String.init) ?? "null")}
            """
        return try JSONDecoder().decode(ConversationInfo.self, from: Data(json.utf8))
    }

    private func sourceDirectory() throws -> String {
        let directory = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
        return try FileManager.default
            .contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "swift" }
            .map { try String(contentsOf: $0, encoding: .utf8) }
            .joined(separator: "\n")
    }
}
