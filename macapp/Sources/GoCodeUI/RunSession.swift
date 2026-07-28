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
    public private(set) var conversationID: String?
    public private(set) var currentRunID: String?
    /// Set when the agent asks a structured question mid-run.
    public private(set) var pendingQuestions: AskUserPrompt?

    public var draft: String = ""
    public var model: String?
    public var planMode = false
    public var extraDirs: [String] = []
    public var profile: String?
    /// Recalled with Up/Down in the composer.
    public private(set) var promptHistory: [String] = []

    private let client: HarnessClient
    private var streamTask: Task<Void, Never>?
    /// Escalates a second interrupt from cooperative cancel to a hard stop.
    private var cancelRequested = false

    public init(client: HarnessClient) {
        self.client = client
    }

    public convenience init(baseURL: URL, token: String? = nil) {
        self.init(client: HarnessClient(baseURL: baseURL, token: token))
    }

    public var isBusy: Bool { transcript.runState.isActive }
    public var canSubmit: Bool { !draft.trimmed.isEmpty && !isBusy }
    /// True while a run is active, so the composer can offer steering instead.
    public var canSteer: Bool { isBusy && transcript.pendingApproval == nil }

    // MARK: - Running

    public func submit() {
        let prompt = draft.trimmed
        guard !prompt.isEmpty, !isBusy else { return }
        draft = ""
        connectionError = nil
        cancelRequested = false
        promptHistory.append(prompt)
        transcript.appendUserPrompt(prompt)

        streamTask = Task { [client, model, planMode, conversationID, extraDirs, profile] in
            do {
                var request = HarnessClient.StartRunRequest(prompt: prompt)
                request.model = model
                request.conversationID = conversationID
                if planMode { request.planMode = true }
                if !extraDirs.isEmpty { request.extraDirs = extraDirs }
                request.profile = profile
                // Fallback exists so the key-free fake provider stays reachable
                // through the default provider. But once a model is picked by
                // hand, silently running a different one is worse than failing:
                // the substitute provider gets a model it does not serve and
                // reports an error that names neither the model nor the swap.
                request.allowFallback = model == nil

                let started = try await client.startRun(request)
                currentRunID = started.runID
                if self.conversationID == nil { self.conversationID = started.runID }

                for try await event in client.events(runID: started.runID) {
                    transcript.apply(event)
                    await handleSideEffects(of: event, runID: started.runID)
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

    /// Two-stage interrupt, matching the TUI: the first request asks harnessd to
    /// stop cooperatively; a second abandons the stream locally.
    public func cancel() {
        guard let runID = currentRunID else {
            streamTask?.cancel()
            return
        }
        if cancelRequested {
            streamTask?.cancel()
            transcript.markCancelled()
            return
        }
        cancelRequested = true
        Task { [client] in try? await client.cancel(runID: runID) }
    }

    public func approve(option: String? = nil) {
        guard let runID = currentRunID else { return }
        Task { [client] in try? await client.approve(runID: runID, option: option) }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
        Task { [client] in try? await client.deny(runID: runID) }
    }

    /// Redirects an in-flight run without cancelling it. Applied at the run's
    /// next step boundary.
    public func steer() {
        let prompt = draft.trimmed
        guard !prompt.isEmpty, let runID = currentRunID else { return }
        draft = ""
        Task { [client] in
            do {
                try await client.steer(runID: runID, prompt: prompt)
            } catch let error as HarnessError {
                connectionError = error.message
            } catch {
                connectionError = error.localizedDescription
            }
        }
    }

    public func answer(_ answers: [String: String]) {
        guard let runID = currentRunID else { return }
        pendingQuestions = nil
        Task { [client] in try? await client.answerInput(runID: runID, answers: answers) }
    }

    // MARK: - Conversation switching

    public func load(messages: [StoredMessage], conversationID: String) {
        streamTask?.cancel()
        transcript.load(messages: messages)
        self.conversationID = conversationID
        currentRunID = nil
        connectionError = nil
    }

    public func reset() {
        streamTask?.cancel()
        transcript.reset()
        conversationID = nil
        currentRunID = nil
        connectionError = nil
        pendingQuestions = nil
    }

    public func rebind(conversationID: String) {
        self.conversationID = conversationID
    }

    public func recallPreviousPrompt() {
        guard let last = promptHistory.last else { return }
        draft = last
    }

    // MARK: - Side effects

    private func handleSideEffects(of event: HarnessEvent, runID: String) async {
        // The question text lives behind a separate endpoint, not in the event.
        guard event.type == .runWaitingForUser || event.type == .other("run.waiting_for_user")
        else { return }
        pendingQuestions = try? await client.pendingInput(runID: runID)
    }
}

extension String {
    var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}

extension Transcript {
    /// Marks the run failed when the transport dies before a terminal event
    /// arrives — otherwise the spinner would never stop.
    mutating func markFailed() {
        apply(localTerminalEvent(type: "run.failed"))
    }

    /// Marks the run cancelled when the user abandons the stream locally.
    mutating func markCancelled() {
        apply(localTerminalEvent(type: "run.cancelled"))
    }
}

private func localTerminalEvent(type: String) -> HarnessEvent {
    let json = #"{"id":"local:0","run_id":"local","type":"\#(type)","payload":{}}"#
    // Constructed locally from a literal, so decoding cannot fail.
    return try! HarnessEvent(frame: SSEFrame(id: "local:0", event: type, data: json))
}
