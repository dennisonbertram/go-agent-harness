import Foundation

/// What the transcript showed for one tool's run, reduced to the primitives
/// `judge` needs. Deliberately independent of `HarnessKit.Transcript` (whose
/// item initializers are not public outside that module) so this — the part
/// that decides pass/fail — is constructible and testable with plain values.
struct ObservedRun: Sendable {
    var toolCompleted: [String]
    var toolBlocked: [String]
    var toolFailed: [String]
    var finalReply: String
    var runFailed: Bool
    var runCancelled: Bool
    var connectionError: String?
    var timedOut: Bool
}

/// One tool's verdict, written into `/tmp/toolwalk-results.json`.
struct ToolResult: Codable, Sendable {
    let name: String
    let verdict: String
    let reply: String
}

/// Phrases a model uses to describe *not* calling a tool. A tool call can
/// show up as "completed" in the transcript and the model can still, in the
/// same turn, tell the user it could not do the thing — that is a failure,
/// not evidence the tool worked.
private let deflectionPhrases = [
    "i cannot call", "i can't call", "i do not have access to", "i don't have access to",
    "unable to call", "no such tool", "not available to me", "i am not able to call",
    "i'm not able to call", "cannot invoke", "can't invoke",
    "i don't have the ability to call", "i do not have the ability to call",
    "as an ai, i cannot", "i wasn't able to call", "i was not able to call",
]

/// Judges one tool's transcript. A tool passes only when its own
/// `ToolActivity` reached `.completed` *and* the final assistant reply is a
/// non-empty message that does not itself disclaim the call — a 200 status
/// or a confident-sounding reply is not, by itself, evidence the tool ran.
func judge(tool: String, observed: ObservedRun) -> ToolResult {
    if observed.timedOut {
        return ToolResult(
            name: tool, verdict: "fail",
            reply: "timed out waiting for the run to reach a terminal state "
                + "(reply so far: \(observed.finalReply))")
    }
    if let error = observed.connectionError, !error.isEmpty {
        return ToolResult(name: tool, verdict: "fail", reply: "transport error: \(error)")
    }
    if observed.runCancelled {
        return ToolResult(
            name: tool, verdict: "fail", reply: "run was cancelled before completing")
    }
    if observed.toolFailed.contains(tool) {
        return ToolResult(
            name: tool, verdict: "fail",
            reply: "tool call reported failure; reply: \(observed.finalReply)")
    }
    if observed.toolBlocked.contains(tool) {
        return ToolResult(
            name: tool, verdict: "fail",
            reply: "tool call is still blocked awaiting approval")
    }
    guard observed.toolCompleted.contains(tool) else {
        let suffix =
            observed.finalReply.isEmpty ? "" : "; assistant said: \(observed.finalReply)"
        return ToolResult(
            name: tool, verdict: "fail", reply: "the tool '\(tool)' was never invoked\(suffix)")
    }

    let reply = observed.finalReply.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !reply.isEmpty else {
        return ToolResult(
            name: tool, verdict: "fail",
            reply: "tool call completed but the assistant gave no reply")
    }
    let lowered = reply.lowercased()
    if let phrase = deflectionPhrases.first(where: { lowered.contains($0) }) {
        return ToolResult(
            name: tool, verdict: "fail",
            reply: "assistant reply disclaims the tool call (matched \"\(phrase)\"): \(reply)")
    }

    return ToolResult(name: tool, verdict: "pass", reply: reply)
}
