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

    /// Keeps the conversation-wide stream (issue #950) open for as long as a
    /// conversation is selected, independent of whether this app instance
    /// started the run producing events -- a delayed callback or cron run can
    /// fire after the run that scheduled it already ended, with no run left
    /// to `events(runID:)` subscribe to.
    private var conversationStreamTask: Task<Void, Never>?
    /// The conversation `conversationStreamTask` is currently open for, so
    /// re-setting `conversationID` to the same value (e.g. `rebind` called
    /// twice) does not tear down and reopen the stream needlessly.
    private var streamingConversationID: String?
    /// Event ids already applied to the transcript. The per-run stream
    /// started by `submit()` and the conversation-wide stream both observe
    /// the same events for a run this app started, so without this a reply
    /// would render twice. `HarnessEvent.id` (`<run id>:<sequence>`) is
    /// unique across every run, so a plain set is enough to dedupe.
    // ponytail: unbounded for the session's lifetime -- fine at chat-transcript
    // scale; revisit with an LRU/bounded cap if a conversation runs long
    // enough for this to matter.
    private var seenEventIDs: Set<String> = []
    /// The one run whose accounting may be rendered. A conversation can replay
    /// old events and advance through callbacks/cron runs, so transcript
    /// message ownership alone is not a safe accounting boundary.
    /// Internal for deterministic app-level lifecycle regressions. The UI
    /// reads only `transcript.usage`; this identity prevents a stale replay
    /// from treating another run's terminal event as the current run.
    private(set) var accountingRunID: String?
    private var accountingTimestamp: Date?

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
        clearAccounting()

        // `startingConversationID` is deliberately renamed away from the
        // property it's captured from: a capture named `conversationID`
        // would shadow `self.conversationID` for the rest of this closure,
        // so the `if let conversationID { trackConversationStream(...) }`
        // check below would silently see the stale pre-run value (nil, for
        // a brand-new conversation) forever instead of the id this run just
        // minted -- exactly the bug that left the conversation stream never
        // started for the run that most needs it.
        streamTask = Task {
            [client, model, planMode, startingConversationID = conversationID, extraDirs, profile]
            in
            do {
                var request = HarnessClient.StartRunRequest(prompt: prompt)
                request.model = model
                request.conversationID = startingConversationID
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
                activateAccounting(for: started.runID, timestamp: nil)
                if self.conversationID == nil { self.conversationID = started.runID }
                // Keyed by conversation, not by this run: on a conversation's
                // later runs `self.conversationID` is already the first run's
                // id, so this must not retarget the stream to `started.runID`.
                if let conversationID = self.conversationID {
                    trackConversationStream(conversationID)
                }

                for try await event in client.events(runID: started.runID) {
                    await apply(event, runID: started.runID)
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
        accountingRunID = nil
        accountingTimestamp = nil
        self.conversationID = conversationID
        currentRunID = nil
        connectionError = nil
        trackConversationStream(conversationID)
    }

    /// Reconciles durable message history without disturbing the
    /// conversation-wide stream. Used when Chat reappears after a completed
    /// callback/cron run may have advanced the conversation while another
    /// section was visible. An active user-started run remains event-driven so
    /// an incomplete persistence snapshot cannot replace streaming state.
    public func reconcilePersistedMessages(
        _ messages: [StoredMessage], retainingAccountingFor runID: String? = nil,
        preservingRunState: Bool = false
    ) {
        guard !isBusy else { return }
        let preserveAccounting = runID != nil && runID == accountingRunID
        transcript.reconcile(
            messages: messages, preservingUsage: preserveAccounting,
            preservingRunState: preservingRunState)
        if !preserveAccounting {
            accountingRunID = nil
            accountingTimestamp = nil
        }
        connectionError = nil
        pendingQuestions = nil
    }

    /// Stops following this session's conversation. Called on a fresh
    /// conversation, and by `ProjectSession.shutdown()` so a torn-down
    /// harnessd does not leave the reconnect loop spinning against a process
    /// that will never answer again.
    public func reset() {
        streamTask?.cancel()
        stopConversationStream()
        transcript.reset()
        accountingRunID = nil
        accountingTimestamp = nil
        conversationID = nil
        currentRunID = nil
        connectionError = nil
        pendingQuestions = nil
    }

    public func rebind(conversationID: String) {
        self.conversationID = conversationID
        trackConversationStream(conversationID)
    }

    public func recallPreviousPrompt() {
        guard let last = promptHistory.last else { return }
        draft = last
    }

    // MARK: - Conversation-wide event stream (issue #950)

    /// Opens the conversation-wide stream for `conversationID` unless it is
    /// already open for that same conversation.
    private func trackConversationStream(_ conversationID: String) {
        guard streamingConversationID != conversationID else { return }
        stopConversationStream()
        streamingConversationID = conversationID
        conversationStreamTask = Task { [client] in
            await self.streamConversation(client: client, conversationID: conversationID)
        }
    }

    public func stopConversationStream() {
        conversationStreamTask?.cancel()
        conversationStreamTask = nil
        streamingConversationID = nil
    }

    /// Keeps re-opening `client.conversationEvents` for as long as this task
    /// is not cancelled. The server-side stream only ends on client
    /// disconnect or genuine transport failure -- never on a terminal run
    /// event -- so any exit from the inner loop here is a dropped connection,
    /// not "the conversation is done"; reconnecting with the last seen event
    /// id is what makes a flaky connection recoverable instead of a silent
    /// dead subscription (the exact bug this stream exists to fix).
    private func streamConversation(client: HarnessClient, conversationID: String) async {
        var lastEventID: String?
        while !Task.isCancelled {
            do {
                for try await event in client.conversationEvents(
                    conversationID: conversationID, lastEventID: lastEventID)
                {
                    lastEventID = event.id
                    _ = await apply(event, runID: event.runID)
                    // A fresh app can open a durable message snapshot and then
                    // receive the same completed run in the conversation
                    // replay. Reconcile at each terminal boundary so replay
                    // cannot leave persisted assistant/tool rows duplicated.
                    // The busy guard in reconcilePersistedMessages protects a
                    // newer user-started run that begins during this fetch.
                    if event.type.isTerminal,
                        let messages = try? await client.messages(conversationID: conversationID)
                    {
                        let isStaleTerminal =
                            accountingRunID != nil && accountingRunID != event.runID
                        reconcilePersistedMessages(
                            messages,
                            retainingAccountingFor: accountingRunID,
                            preservingRunState: isStaleTerminal)
                    }
                }
            } catch is CancellationError {
                return
            } catch {
                // Transport error -- fall through to the reconnect delay below.
            }
            if Task.isCancelled { return }
            // ponytail: fixed short delay, no exponential backoff/jitter --
            // add one if this ever hammers a genuinely-down server.
            try? await Task.sleep(for: .milliseconds(500))
        }
    }

    /// Applies `event` to the transcript at most once. Both the per-run
    /// stream started by `submit()` and the conversation-wide stream deliver
    /// the same events for a run this app started, and rendering both copies
    /// would double every message (issue #950 requirement 4).
    @discardableResult
    private func apply(_ event: HarnessEvent, runID: String) async -> Bool {
        guard seenEventIDs.insert(event.id).inserted else { return false }
        let includedAccounting = admitAccounting(for: event)
        // Conversation replay can deliver any lifecycle frame for an older
        // run after a newer run has taken ownership. Its durable rows still
        // need terminal reconciliation, but an old `run.started`, approval,
        // waiting, or terminal frame must not mutate the newer run's visible
        // state or make the reconciliation busy-guard reject those rows.
        if suppressesForeignStateMutation(event, includedAccounting: includedAccounting) {
            return false
        }
        transcript.apply(event, includingAccounting: includedAccounting)
        await handleSideEffects(of: event, runID: runID)
        return includedAccounting
    }

    /// Accounting admission is the run-ownership fence. Content events from a
    /// foreign replay remain useful transcript history, but only the owner may
    /// change lifecycle or approval state. This deliberately covers more than
    /// terminal events: a stale `run.started` previously made a completed B
    /// look running, after which the stale terminal's durable reconciliation
    /// was skipped by `isBusy`.
    private func suppressesForeignStateMutation(
        _ event: HarnessEvent, includedAccounting: Bool
    ) -> Bool {
        guard !includedAccounting, let accountingRunID, accountingRunID != event.runID else {
            return false
        }
        switch event.type {
        case .runQueued, .runStarted, .runResumed,
            .runCompleted, .runFailed, .runCancelled,
            .toolApprovalRequired, .planApprovalRequired,
            .toolApprovalGranted, .toolApprovalDenied,
            .planApprovalGranted, .planApprovalDenied,
            .runWaitingForUser:
            return true
        default:
            return false
        }
    }

    private func clearAccounting() {
        accountingRunID = nil
        accountingTimestamp = nil
        transcript.clearUsage()
    }

    private func activateAccounting(for runID: String, timestamp: Date?) {
        guard accountingRunID != runID else {
            if let timestamp { accountingTimestamp = timestamp }
            return
        }
        accountingRunID = runID
        accountingTimestamp = timestamp
        transcript.clearUsage()
    }

    /// Returns whether this event may mutate visible usage. Lifecycle starts
    /// establish a newer run; a terminal-only recovered run can do the same
    /// only when its timestamp is not older than the current accounting run.
    private func admitAccounting(for event: HarnessEvent) -> Bool {
        let runID = event.runID
        guard !runID.isEmpty else { return false }
        let isStart: Bool
        switch event.type {
        case .runQueued, .runStarted, .runResumed: isStart = true
        default: isStart = false
        }
        if accountingRunID == runID {
            if let timestamp = event.timestamp { accountingTimestamp = timestamp }
            return true
        }
        let isNewer: Bool
        if let timestamp = event.timestamp {
            // `submit()` owns the run before its first SSE frame supplies a
            // timestamp. That provisional ownership is still authoritative:
            // a replay from another run must not steal it merely because it
            // has a timestamp while the current owner does not yet have one.
            isNewer = accountingTimestamp.map { timestamp >= $0 } ?? false
        } else {
            isNewer = false
        }
        if isStart && (accountingRunID == nil || isNewer) {
            activateAccounting(for: runID, timestamp: event.timestamp)
            return true
        }
        if event.type.isTerminal && (accountingRunID == nil || isNewer) {
            activateAccounting(for: runID, timestamp: event.timestamp)
            return true
        }
        return false
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
