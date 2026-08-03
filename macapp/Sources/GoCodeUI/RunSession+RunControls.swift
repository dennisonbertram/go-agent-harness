import Foundation
import HarnessKit

extension RunSession {
    /// True only while the first, cooperative cancel request awaits harnessd's
    /// acknowledgement. Once it succeeds, a second press remains available
    /// for the existing local force-stop behavior.
    public var cancelInFlight: Bool { cancelState == .requesting }

    public func cancel() {
        guard let runID = currentRunID else {
            streamTask?.cancel()
            return
        }
        switch cancelState {
        case .requested:
            // The selected continuation may be external while a distinct local
            // per-run stream is still draining. Never cancel that unrelated
            // stream on an external force-stop.
            if localStreamRunID == runID { streamTask?.cancel() }
            transcript.markCancelled()
            retire(runID: runID)
            cancelState = .idle
        case .requesting:
            return
        case .idle:
            cancelState = .requesting
        }
        connectionError = nil
        Task { [client] in
            do {
                try await client.cancel(runID: runID)
                guard currentRunID == runID else { return }
                cancelState = .requested
            } catch let error as HarnessError {
                guard currentRunID == runID else { return }
                connectionError = error.message
                cancelState = .idle
            } catch {
                guard currentRunID == runID else { return }
                connectionError = error.localizedDescription
                cancelState = .idle
            }
        }
    }

    public func approve(expectedRunID: String, option: String? = nil) {
        guard currentRunID == expectedRunID,
            transcript.pendingApproval?.runID == expectedRunID
                || transcript.pendingPlan?.runID == expectedRunID
        else { return }
        let runID = expectedRunID
        runControlTask(runID: runID, awaitingLifecycle: true) { [client] in
            try await client.approve(runID: runID, option: option)
        }
    }

    public func approve(option: String? = nil) {
        guard let runID = currentRunID else { return }
        // Legacy programmatic control remains available to existing callers
        // that have a run but do not render an approval affordance (for
        // example diagnostic/control tests). UI and ToolWalk use the explicit
        // `expectedRunID` overload above, which is the stale-closure fence.
        runControlTask(runID: runID, awaitingLifecycle: true) { [client] in
            try await client.approve(runID: runID, option: option)
        }
    }

    public func deny(expectedRunID: String) {
        guard currentRunID == expectedRunID,
            transcript.pendingApproval?.runID == expectedRunID
                || transcript.pendingPlan?.runID == expectedRunID
        else { return }
        let runID = expectedRunID
        runControlTask(runID: runID, awaitingLifecycle: true) { [client] in
            try await client.deny(runID: runID)
        }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
        // See `approve(option:)`: this compatibility overload is not used by
        // visible interaction UI, which always carries the pending run id.
        runControlTask(runID: runID, awaitingLifecycle: true) { [client] in
            try await client.deny(runID: runID)
        }
    }

    /// Redirects an in-flight run without cancelling it. Applied at the run's
    /// next step boundary.
    public func steer() {
        // The guard must happen before reading/clearing the draft: keyboard
        // submission can invoke steer while the first POST is still awaiting
        // acknowledgement, and that later draft remains the user's text.
        guard !runControlInFlight else { return }
        let originalDraft = draft
        let prompt = originalDraft.trimmed
        guard !prompt.isEmpty, let runID = currentRunID else { return }
        draft = ""
        runControlTask(runID: runID, restoreDraft: originalDraft) { [client] in
            try await client.steer(runID: runID, prompt: prompt)
        }
    }

    private func runControlTask(
        runID: String,
        restoreDraft: String? = nil,
        awaitingLifecycle: Bool = false,
        _ operation: @escaping () async throws -> Void
    ) {
        guard !runControlInFlight else { return }
        runControlInFlight = true
        connectionError = nil
        runControlRequestGeneration &+= 1
        let generation = runControlRequestGeneration
        let lifecycleGeneration = runControlLifecycleGenerationByRunID[runID, default: 0]
        Task {
            do {
                // The explicit action must still own the selected run when
                // the asynchronous task begins. Otherwise stale SwiftUI
                // callbacks could make a network request for a new owner.
                guard currentRunID == runID else {
                    if runControlRequestGeneration == generation { runControlInFlight = false }
                    return
                }
                try await operation()
                // The run stream can reach a terminal event while the daemon
                // is still returning this POST. A terminal clears
                // `currentRunID`, but it must not leave this request owning
                // the composer forever. Request generation, invalidated by a
                // conversation switch/reset or a newer control request, is
                // the ownership fence; current-run identity is deliberately
                // not one because terminal completion is a valid outcome for
                // the request's own run.
                guard runControlRequestGeneration == generation else {
                    return
                }
                if awaitingLifecycle,
                    runControlLifecycleGenerationByRunID[runID, default: 0] == lifecycleGeneration
                {
                    acknowledgedRunControlRunID = runID
                } else {
                    runControlInFlight = false
                }
            } catch let error as HarnessError {
                guard runControlRequestGeneration == generation else {
                    return
                }
                runControlInFlight = false
                connectionError = error.message
                if let restoreDraft, draft.isEmpty { draft = restoreDraft }
            } catch {
                guard runControlRequestGeneration == generation else {
                    return
                }
                runControlInFlight = false
                connectionError = error.localizedDescription
                if let restoreDraft, draft.isEmpty { draft = restoreDraft }
            }
        }
    }
}
