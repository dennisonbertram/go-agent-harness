import Foundation
import HarnessKit

/// The identity and evidence owned by one locally submitted prompt.
///
/// `RunSession` has one rendered conversation lifecycle, which a scheduled
/// callback or cron continuation may legitimately replace while a local run's
/// HTTP response or SSE stream is still in flight. Callers that need to judge
/// or control *their* submission must therefore retain this handle instead of
/// consulting `RunSession.currentRunID` or its shared transcript later.
@MainActor
public final class RunSubmission {
    public enum State: Equatable {
        case starting
        case started(String)
        case terminal(String)
        case failed(String)
        case displaced
    }

    public private(set) var state: State = .starting
    public private(set) var transcript = Transcript()
    /// Assigned only from a successful `startRun` response. It remains
    /// available after a later stream failure or displacement so cleanup and
    /// diagnostics can still name A without consulting shared session state.
    private var resolvedRunID: String?

    public var runID: String? { resolvedRunID }

    public var failure: String? {
        guard case .failed(let message) = state else { return nil }
        return message
    }

    public var isTerminal: Bool {
        if case .terminal = state { return true }
        return false
    }

    public var isDisplaced: Bool {
        if case .displaced = state { return true }
        return false
    }

    init(prompt: String) {
        transcript.appendUserPrompt(prompt)
    }

    func markStarted(runID: String) {
        guard case .starting = state else { return }
        resolvedRunID = runID
        state = .started(runID)
    }

    func apply(_ event: HarnessEvent) {
        guard runID == event.runID else { return }
        transcript.apply(event)
        // Retain A's evidence for diagnostics, but once B displaced this
        // submission its later terminal frame must not turn ToolWalk back into
        // an apparent successful A lifecycle. The caller must still abort
        // rather than act on or judge through B's selected state.
        if event.type.isTerminal, !isDisplaced { state = .terminal(event.runID) }
    }

    func markFailed(_ message: String) {
        guard !isTerminal, !isDisplaced else { return }
        state = .failed(message)
        transcript.markFailed()
    }

    func markDisplaced() {
        guard !isTerminal, failure == nil else { return }
        state = .displaced
    }
}
