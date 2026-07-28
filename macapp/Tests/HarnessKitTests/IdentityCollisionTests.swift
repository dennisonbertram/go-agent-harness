import Foundation
import Testing

@testable import HarnessKit

/// `AskUserQuestion.id` used to be the question text itself, so two questions
/// with identical wording in one prompt collided and glitched `ForEach`
/// diffing. The companion fix for `TodoItem.stableID` is covered where it is
/// actually exercised, in `HarnessClientTests` (`todos(runID:)` is what stamps
/// the array position the fix relies on). See #951 finding 6.
@Suite("Identifiable id collisions")
struct IdentityCollisionTests {

    @Test("AskUserQuestion.id differs for duplicate question text in one prompt")
    func askUserQuestionIDsDontCollide() throws {
        let json = """
            {"run_id":"run_1","call_id":"call_1","questions":[
                {"question":"Continue?"},
                {"question":"Continue?"},
                {"question":"Continue?"}
            ]}
            """
        let prompt = try HarnessClient.decoder.decode(AskUserPrompt.self, from: Data(json.utf8))
        #expect(Set(prompt.questions.map(\.id)).count == 3)
    }
}
