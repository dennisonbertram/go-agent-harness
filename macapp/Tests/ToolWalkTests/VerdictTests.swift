import Testing

@testable import ToolWalk

/// `judge` is the whole point of the walk: it decides whether a tool's
/// transcript is evidence the tool actually ran, independent of the live
/// daemon or model. These tests build `ObservedRun` values by hand — the
/// same shape `Runner.observe(_:timedOut:)` produces from a real
/// `RunSession.transcript` — so the judging rule is provable without a
/// network call.
@Suite("judge")
struct VerdictTests {

    private func observed(
        completed: [String] = [], blocked: [String] = [], failed: [String] = [],
        reply: String = "", runFailed: Bool = false, runCancelled: Bool = false,
        connectionError: String? = nil, timedOut: Bool = false
    ) -> ObservedRun {
        ObservedRun(
            toolCompleted: completed, toolBlocked: blocked, toolFailed: failed,
            finalReply: reply, runFailed: runFailed, runCancelled: runCancelled,
            connectionError: connectionError, timedOut: timedOut)
    }

    @Test("passes when the tool completed and the assistant reports on it")
    func passesOnCompletedToolWithReply() {
        let result = judge(
            tool: "bash",
            observed: observed(completed: ["bash"], reply: "The command printed UIWALK_BASH_OK."))
        #expect(result.verdict == "pass")
        #expect(result.reply == "The command printed UIWALK_BASH_OK.")
    }

    /// The central regression case: a run that returned 200 and produced a
    /// plausible-sounding reply, but never actually invoked the tool under
    /// test (e.g. the model narrated success instead of calling it). If
    /// `judge` is ever changed to pass on reply text alone, this fails.
    @Test("fails when the reply sounds like success but the tool never ran")
    func failsWhenToolNeverInvokedDespiteConfidentReply() {
        let result = judge(
            tool: "deploy",
            observed: observed(
                completed: [], reply: "Deployment completed successfully with no errors."))
        #expect(result.verdict == "fail")
        #expect(result.reply.contains("never invoked"))
    }

    @Test("fails when the assistant explicitly disclaims calling the tool")
    func failsOnDeflection() {
        let result = judge(
            tool: "download",
            observed: observed(
                completed: ["download"],
                reply: "I don't have access to a download tool, so I can't do that."))
        #expect(result.verdict == "fail")
    }

    @Test("fails when the tool call itself reported failure")
    func failsOnToolFailure() {
        let result = judge(
            tool: "apply_patch",
            observed: observed(failed: ["apply_patch"], reply: "The patch could not be applied."))
        #expect(result.verdict == "fail")
    }

    @Test("fails when the tool call is still blocked awaiting approval")
    func failsOnBlockedApproval() {
        let result = judge(
            tool: "bash",
            observed: observed(blocked: ["bash"], reply: "Waiting for approval."))
        #expect(result.verdict == "fail")
    }

    @Test("fails on an empty reply even if the tool completed")
    func failsOnEmptyReply() {
        let result = judge(tool: "ls", observed: observed(completed: ["ls"], reply: ""))
        #expect(result.verdict == "fail")
    }

    @Test("fails when the run timed out")
    func failsOnTimeout() {
        let result = judge(
            tool: "agent",
            observed: observed(completed: ["agent"], reply: "still working", timedOut: true))
        #expect(result.verdict == "fail")
        #expect(result.reply.contains("timed out"))
    }

    @Test("fails on a transport error regardless of any reply")
    func failsOnConnectionError() {
        let result = judge(
            tool: "write",
            observed: observed(
                completed: ["write"], reply: "wrote the file", connectionError: "connection reset")
        )
        #expect(result.verdict == "fail")
    }

    @Test("fails when the run was cancelled before finishing")
    func failsOnCancellation() {
        let result = judge(
            tool: "grep",
            observed: observed(completed: ["grep"], reply: "found it", runCancelled: true))
        #expect(result.verdict == "fail")
    }
}
