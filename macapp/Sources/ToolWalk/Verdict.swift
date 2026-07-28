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

// STUB — deliberately wrong (always "pass"), to prove the tests above fail
// for the right reason before the real judging rule is written.
func judge(tool: String, observed: ObservedRun) -> ToolResult {
    ToolResult(name: tool, verdict: "pass", reply: observed.finalReply)
}
