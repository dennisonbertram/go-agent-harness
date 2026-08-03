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
        runControlTask(runID: runID) { [client] in
            try await client.approve(runID: runID, option: option)
        }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
        runControlTask(runID: runID) { [client] in
            try await client.deny(runID: runID)
        }
    }

    /// Redirects an in-flight run without cancelling it. Applied at the run's
    /// next step boundary.
    public func steer() {
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
        _ operation: @escaping () async throws -> Void
    ) {
        guard !runControlInFlight else { return }
        runControlInFlight = true
        runControlRequestGeneration &+= 1
        let generation = runControlRequestGeneration
        Task {
            defer {
                if runControlRequestGeneration == generation {
                    runControlInFlight = false
                }
            }
            do {
                try await operation()
            } catch let error as HarnessError {
                guard currentRunID == runID, runControlRequestGeneration == generation else {
                    return
                }
                connectionError = error.message
                if let restoreDraft, draft.isEmpty { draft = restoreDraft }
            } catch {
                guard currentRunID == runID, runControlRequestGeneration == generation else {
                    return
                }
                connectionError = error.localizedDescription
                if let restoreDraft, draft.isEmpty { draft = restoreDraft }
            }
        }
    }
}
