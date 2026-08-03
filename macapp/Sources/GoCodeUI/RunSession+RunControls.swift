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
            currentRunID = nil
            streamTask?.cancel()
            transcript.markCancelled()
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

    public func approve(option: String? = nil) {
        guard let runID = currentRunID else { return }
        runControlTask(runID: runID, awaitingLifecycle: true) { [client] in
            try await client.approve(runID: runID, option: option)
        }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
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
                try await operation()
                guard currentRunID == runID, runControlRequestGeneration == generation else {
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
                guard currentRunID == runID, runControlRequestGeneration == generation else {
                    return
                }
                runControlInFlight = false
                connectionError = error.message
                if let restoreDraft, draft.isEmpty { draft = restoreDraft }
            } catch {
                guard currentRunID == runID, runControlRequestGeneration == generation else {
                    return
                }
                runControlInFlight = false
                connectionError = error.localizedDescription
                if let restoreDraft, draft.isEmpty { draft = restoreDraft }
            }
        }
    }
}
