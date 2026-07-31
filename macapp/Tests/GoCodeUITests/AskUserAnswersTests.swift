import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Exercises the fix for #994 (F3): the `Send` button and `RunSession.answer`'s
/// guard used to accept `answers.count < prompt.questions.count`, which counts
/// a question whose field was typed into and then cleared back to `""` as
/// answered. `AskUserAnswers.isComplete` is the shared, testable replacement.
@Suite("AskUserAnswers completeness")
struct AskUserAnswersTests {

    /// `AskUserPrompt` only decodes from the wire shape (no memberwise init),
    /// so tests build one the same way production code does.
    private func prompt(questionTexts: [String]) throws -> AskUserPrompt {
        let questions = questionTexts.map { #"{"question":"\#($0)"}"# }.joined(separator: ",")
        let json = #"{"run_id":"run_1","call_id":"call_1","questions":[\#(questions)]}"#
        return try JSONDecoder().decode(AskUserPrompt.self, from: Data(json.utf8))
    }

    @Test("every question answered with non-blank text is complete")
    func allAnswered() throws {
        let prompt = try prompt(questionTexts: ["a?", "b?"])
        let answers = [prompt.questions[0].id: "yes", prompt.questions[1].id: "no"]
        #expect(AskUserAnswers.isComplete(prompt: prompt, answers: answers))
    }

    @Test("a missing answer id is incomplete")
    func missingID() throws {
        let prompt = try prompt(questionTexts: ["a?", "b?"])
        let answers = [prompt.questions[0].id: "yes"]
        #expect(!AskUserAnswers.isComplete(prompt: prompt, answers: answers))
    }

    @Test("an empty-string answer is incomplete")
    func emptyAnswer() throws {
        let prompt = try prompt(questionTexts: ["a?"])
        let answers = [prompt.questions[0].id: ""]
        #expect(!AskUserAnswers.isComplete(prompt: prompt, answers: answers))
    }

    @Test("a whitespace-only answer is incomplete -- the finding this predicate fixes")
    func whitespaceOnlyAnswer() throws {
        let prompt = try prompt(questionTexts: ["a?"])
        let answers = [prompt.questions[0].id: "   "]
        #expect(!AskUserAnswers.isComplete(prompt: prompt, answers: answers))
    }

    @Test("a single freeform question answered is complete")
    func singleFreeformAnswered() throws {
        let prompt = try prompt(questionTexts: ["What is the file path?"])
        #expect(prompt.questions[0].isFreeform)
        let answers = [prompt.questions[0].id: "/tmp/x.txt"]
        #expect(AskUserAnswers.isComplete(prompt: prompt, answers: answers))
    }
}
