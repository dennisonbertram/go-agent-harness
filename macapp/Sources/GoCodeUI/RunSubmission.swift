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
    /// Compatibility projection for clients which predate the split lifecycle
    /// and displacement model. New waiting code must inspect `outcome` so an
    /// A terminal/failure cannot be erased by an independently selected B.
    public enum State: Equatable {
        case starting
        case started(String)
        case terminal(String)
        case failed(String)
        case displaced
    }

    public enum Lifecycle: Equatable {
        case starting
        case started(String)
        case terminal(String)
        case failed(String)
    }

    /// The result owned by A. It intentionally does not contain displacement:
    /// B selection is an authority boundary, not a rewrite of A's history.
    public private(set) var lifecycle: Lifecycle = .starting
    /// A compatibility projection. A terminal/failure remains visible here
    /// even when `isDisplaced` is also true.
    public var state: State {
        switch lifecycle {
        case .starting: .starting
        case let .started(runID): .started(runID)
        case let .terminal(runID): .terminal(runID)
        case let .failed(message): .failed(message)
        }
    }

    public private(set) var transcript = Transcript()
    /// Assigned only from a successful `startRun` response. It remains
    /// available after a later stream failure or displacement so cleanup and
    /// diagnostics can still name A without consulting shared session state.
    private var resolvedRunID: String?
    public private(set) var isDisplaced = false
    /// An exact timeout capability is bound to the owning RunSession instance
    /// and its reset/load generation. It is consumed at most once, rather than
    /// being reconstructed from mutable selected-run state later.
    private let timeoutOwner: UUID
    private let timeoutGeneration: UInt
    private var timeoutCancellationConsumed = false

    public var runID: String? {
        resolvedRunID
    }

    public var failure: String? {
        guard case let .failed(message) = lifecycle else { return nil }
        return message
    }

    public var isTerminal: Bool {
        if case .terminal = lifecycle { return true }
        return false
    }

    init(prompt: String, timeoutOwner: UUID, timeoutGeneration: UInt) {
        self.timeoutOwner = timeoutOwner
        self.timeoutGeneration = timeoutGeneration
        transcript.appendUserPrompt(prompt)
    }

    /// Reducer-only construction has no session cancellation authority.
    convenience init(prompt: String) {
        self.init(prompt: prompt, timeoutOwner: UUID(), timeoutGeneration: 0)
    }

    /// Returns A's immutable timeout cancellation capability exactly once.
    /// Terminal/failure are definitive A outcomes and revoke it even if their
    /// per-run task has not yet unwound.
    func consumeTimeoutCancellation(owner: UUID, generation: UInt) -> String? {
        guard timeoutOwner == owner,
              timeoutGeneration == generation,
              !timeoutCancellationConsumed,
              case let .started(runID) = lifecycle
        else { return nil }
        timeoutCancellationConsumed = true
        return runID
    }

    func markStarted(runID: String) {
        guard case .starting = lifecycle else { return }
        resolvedRunID = runID
        lifecycle = .started(runID)
    }

    func apply(_ event: HarnessEvent) {
        guard runID == event.runID else { return }
        transcript.apply(event)
        // A's terminal lifecycle is local evidence. B selection prevents
        // automatic controls, but it must never turn an actual A completion
        // into a false timeout or discard its transcript for ToolWalk.
        if event.type.isTerminal, !isTerminal, failure == nil {
            lifecycle = .terminal(event.runID)
        }
    }

    func markFailed(_ message: String) {
        guard !isTerminal, failure == nil else { return }
        lifecycle = .failed(message)
        transcript.markFailed()
    }

    func markDisplaced() {
        isDisplaced = true
    }
}
