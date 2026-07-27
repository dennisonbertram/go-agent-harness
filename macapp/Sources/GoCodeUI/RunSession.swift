import Foundation
import HarnessKit
import Observation

/// Drives one conversation against one harnessd.
///
/// All rendering state lives in `transcript`, whose reduction logic is a plain
/// value type tested by replaying captured streams. This class owns only the
/// async plumbing around it.
@MainActor
@Observable
public final class RunSession {
    public private(set) var transcript = Transcript()
    public private(set) var connectionError: String?
    public var draft: String = ""

    private let client: HarnessClient
    private var streamTask: Task<Void, Never>?
    private var currentRunID: String?

    public init(baseURL: URL, token: String? = nil) {
        self.client = HarnessClient(baseURL: baseURL, token: token)
    }

    public var isBusy: Bool { transcript.runState.isActive }
    public var canSubmit: Bool { !draft.trimmed.isEmpty && !isBusy }

    /// Submits the composer's contents as a new run.
    public func submit() {
        let prompt = draft.trimmed
        guard !prompt.isEmpty, !isBusy else { return }
        draft = ""
        connectionError = nil
        transcript.appendUserPrompt(prompt)

        streamTask = Task { [client] in
            do {
                var request = HarnessClient.StartRunRequest(prompt: prompt)
                // The key-free fake provider is only reachable via default-provider
                // fallback; harmless against a real provider.
                request.allowFallback = true
                let started = try await client.startRun(request)
                currentRunID = started.runID

                for try await event in client.events(runID: started.runID) {
                    transcript.apply(event)
                }
            } catch let error as HarnessError {
                connectionError = error.message
                transcript.markFailed()
            } catch {
                connectionError = error.localizedDescription
                transcript.markFailed()
            }
            currentRunID = nil
        }
    }

    public func cancel() {
        guard let runID = currentRunID else {
            streamTask?.cancel()
            return
        }
        Task { [client] in
            // Cooperative cancel; the run's terminal event ends the stream.
            try? await client.cancel(runID: runID)
        }
    }

    public func approve() {
        guard let runID = currentRunID else { return }
        Task { [client] in try? await client.approve(runID: runID) }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
        Task { [client] in try? await client.deny(runID: runID) }
    }
}

extension String {
    var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}

extension Transcript {
    /// Marks the run failed when the transport dies before a terminal event
    /// arrives — otherwise the spinner would never stop.
    mutating func markFailed() {
        apply(syntheticFailure)
    }
}

private let syntheticFailure: HarnessEvent = {
    let json = #"{"id":"local:0","run_id":"local","type":"run.failed","payload":{}}"#
    return try! HarnessEvent(frame: SSEFrame(id: "local:0", event: "run.failed", data: json))
}()
