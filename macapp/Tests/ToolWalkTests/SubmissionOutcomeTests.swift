import GoCodeUI
import Testing

@testable import ToolWalk

@Suite("ToolWalk submission wait outcomes")
@MainActor
struct SubmissionOutcomeTests {
    @Test("terminal A wins over later B displacement")
    func terminalPrecedesDisplacement() {
        #expect(
            Runner.outcome(for: .terminal("run_a"), isDisplaced: true) == .terminal)
    }

    @Test("failed A wins over later B displacement")
    func failurePrecedesDisplacement() {
        #expect(
            Runner.outcome(for: .failed("A stream ended"), isDisplaced: true)
                == .failed("A stream ended"))
    }

    @Test("only a genuine timeout permits guarded cancellation")
    func onlyTimeoutCancels() {
        #expect(Runner.shouldCancel(for: .timedOut))
        #expect(!Runner.shouldCancel(for: .terminal))
        #expect(!Runner.shouldCancel(for: .failed("A failed")))
        #expect(!Runner.shouldCancel(for: .displaced))
        #expect(!Runner.shouldCancel(for: .started))
    }
}
