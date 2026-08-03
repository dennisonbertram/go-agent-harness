import Foundation
import HarnessKit

extension RunSession {
    public func answer(_ answers: [String: String]) {
        guard let runID = currentRunID,
            let prompt = pendingQuestions,
            AskUserAnswers.isComplete(prompt: prompt, answers: answers),
            !answerInFlight
        else { return }

        answerInFlight = true
        connectionError = nil
        answerRequestGeneration &+= 1
        let generation = answerRequestGeneration
        Task { [client] in
            defer {
                if answerRequestGeneration == generation {
                    answerInFlight = false
                }
            }
            do {
                try await client.answerInput(runID: runID, answers: answers)
                guard currentRunID == runID, pendingQuestions?.callID == prompt.callID else {
                    return
                }
                pendingQuestions = nil
            } catch let error as HarnessError {
                guard currentRunID == runID else { return }
                connectionError = error.message
            } catch {
                guard currentRunID == runID else { return }
                connectionError = error.localizedDescription
            }
        }
    }
}
