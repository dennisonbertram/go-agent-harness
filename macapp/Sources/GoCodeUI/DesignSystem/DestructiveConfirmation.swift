import HarnessKit
import SwiftUI

/// One shared destructive-confirmation presentation (KTD-4): delete, undo,
/// rewind, force-rewind, and provider-remove read identically instead of
/// each hand-rolling its own alert wording and severity.
struct DestructiveConfirmation: Identifiable {
    let id = UUID()
    let title: String
    let message: String
    let confirmLabel: String
    let action: () -> Void
}

extension View {
    /// Presents `confirmation` as a destructive alert bound to an optional
    /// value. Mirrors `CheckpointsView`'s existing alert shape: `Cancel` is
    /// `.cancel`, the destructive verb is `.destructive`. Confirming or
    /// cancelling clears the binding.
    func destructiveConfirmation(_ confirmation: Binding<DestructiveConfirmation?>) -> some View {
        alert(
            confirmation.wrappedValue?.title ?? "",
            isPresented: Binding(
                get: { confirmation.wrappedValue != nil },
                set: { if !$0 { confirmation.wrappedValue = nil } }
            )
        ) {
            if let item = confirmation.wrappedValue {
                Button("Cancel", role: .cancel) { confirmation.wrappedValue = nil }
                Button(item.confirmLabel, role: .destructive) {
                    item.action()
                    confirmation.wrappedValue = nil
                }
            }
        } message: {
            Text(confirmation.wrappedValue?.message ?? "")
        }
    }
}

/// Client-derived preview text for deleting a conversation (KTD-5: no
/// server dry-run exists for delete, so this is composed only from data the
/// app already holds). Never fabricates a message count.
enum DeletePreview {
    static let titleCharacterLimit = 60

    static func message(for conversation: ConversationInfo) -> String {
        let title = truncated(conversation.displayTitle, limit: titleCharacterLimit)
        guard let count = conversation.messageCount else {
            return "\"\(title)\" will be permanently deleted. Its message count is unknown."
        }
        let noun = count == 1 ? "message" : "messages"
        return "\"\(title)\" and its \(count) \(noun) will be permanently deleted."
    }

    static func truncated(_ text: String, limit: Int) -> String {
        guard text.count > limit else { return text }
        return String(text.prefix(limit)) + "…"
    }
}

/// Client-derived preview text for undoing the last turn.
enum UndoPreview {
    static let promptCharacterLimit = 80

    /// The last `.userPrompt` item in a run's transcript -- the turn an undo
    /// would remove. Reused by every undo entry point so each states the
    /// same fact instead of hand-rolling its own reading of the transcript.
    static func lastUserPrompt(in items: [TranscriptItem]) -> String? {
        for item in items.reversed() {
            if case .userPrompt(let text) = item.kind, !text.isEmpty { return text }
        }
        return nil
    }

    static func message(lastPrompt: String?) -> String {
        guard let lastPrompt else {
            return "This removes the last turn. It cannot be undone."
        }
        let flattened = lastPrompt.split(whereSeparator: \.isNewline).joined(separator: " ")
        let truncated = DeletePreview.truncated(flattened, limit: promptCharacterLimit)
        return "This removes the last turn — \"\(truncated)\" — and cannot be undone."
    }
}
