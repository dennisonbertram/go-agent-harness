import Foundation
import HarnessKit

/// Validates that every structured answer is present and meaningful.
public enum AskUserAnswers {
    public static func isComplete(prompt: AskUserPrompt, answers: [String: String]) -> Bool {
        prompt.questions.allSatisfy { !(answers[$0.id]?.trimmed.isEmpty ?? true) }
    }
}
