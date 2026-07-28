import Foundation

/// A structured question the agent asked mid-run.
///
/// Field names mirror `tools.AskUserQuestion`; note `multiSelect` is camelCase
/// on the wire while its siblings are snake_case.
public struct AskUserQuestion: Sendable, Decodable, Hashable, Identifiable {
    public struct Option: Sendable, Decodable, Hashable {
        public let label: String
        public let description: String?
    }

    public let question: String
    public let header: String?
    public let options: [Option]?
    public let multiSelect: Bool?
    /// This question's position in its prompt's `questions` array, stamped by
    /// `AskUserPrompt.init(from:)` right after decoding. The wire payload has
    /// no id field for a question, and `question` text alone collides when
    /// the agent asks the same thing twice — `id` used to just be `question`,
    /// so duplicates glitched `ForEach` diffing (#951 finding 6).
    var index: Int = 0

    enum CodingKeys: String, CodingKey { case question, header, options, multiSelect }

    public var id: String { "\(index):\(question)" }

    public var allowsMultipleAnswers: Bool { multiSelect ?? false }
    /// With no options the answer is free text.
    public var isFreeform: Bool { options?.isEmpty ?? true }
}

public struct AskUserPrompt: Sendable, Decodable, Hashable {
    public let runID: String
    public let callID: String
    public let tool: String?
    public let questions: [AskUserQuestion]
    public let deadlineAt: Date?

    enum CodingKeys: String, CodingKey {
        case runID = "run_id"
        case callID = "call_id"
        case tool, questions
        case deadlineAt = "deadline_at"
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        runID = try container.decode(String.self, forKey: .runID)
        callID = try container.decode(String.self, forKey: .callID)
        tool = try container.decodeIfPresent(String.self, forKey: .tool)
        deadlineAt = try container.decodeIfPresent(Date.self, forKey: .deadlineAt)
        // Stamp array position here — the only place a question's index into
        // its own prompt is available — so `AskUserQuestion.id` stays unique
        // even when two questions share identical wording.
        questions = try container.decode([AskUserQuestion].self, forKey: .questions)
            .enumerated().map { index, question in
                var question = question
                question.index = index
                return question
            }
    }
}

extension HarnessClient {
    /// Fetches the run's pending question, or nil when nothing is pending.
    ///
    /// harnessd answers `409 no_pending_input` rather than 404 when the run
    /// exists but is not waiting; both mean "nothing to ask" to the UI.
    public func pendingInput(runID: String) async throws -> AskUserPrompt? {
        do {
            return try await get("/v1/runs/\(runID)/input") as AskUserPrompt
        } catch let error as HarnessError where error.statusCode == 409 || error.statusCode == 404 {
            return nil
        }
    }
}
