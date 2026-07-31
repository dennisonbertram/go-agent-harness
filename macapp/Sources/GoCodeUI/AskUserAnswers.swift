import Foundation
import HarnessKit

/// Whether `answers` covers every question in `prompt` with a non-blank
/// value.
///
/// Replaces `answers.count < prompt.questions.count`, which happily counted
/// a freeform field the operator typed into and then cleared back to `""`
/// (or left as only whitespace) as answered (#994 / F3). Shared by
/// `RunSession.answer`'s guard -- the root-cause fix, covering any future
/// caller -- and `AskUserView`'s `Send` button, so the button's enabled
/// state and the guard that actually submits agree on the same rule.
public enum AskUserAnswers {
    public static func isComplete(prompt: AskUserPrompt, answers: [String: String]) -> Bool {
        prompt.questions.allSatisfy { question in
            !(answers[question.id]?.trimmed.isEmpty ?? true)
        }
    }
}
