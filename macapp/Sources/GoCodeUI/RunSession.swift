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
    /// True while `answer()`'s request is awaiting the server. The composer's
    /// Send button reads this to disable itself -- without it, an impatient
    /// second click fired a second `answerInput` request before the first
    /// one came back.
    public private(set) var answerInFlight = false

    public var draft: String = ""
    public var model: String?
    public var planMode = false
    public var extraDirs: [String] = []
    public var profile: String?
    /// Recalled with Up/Down in the composer via `recallPreviousPrompt()` /
    /// `recallNextPrompt()` below -- no external reader of this value exists,
    /// so it stays private rather than a public accessor with nothing on the
    /// other end of it.
    private var promptHistory = PromptHistory()

    private let client: HarnessClient
    private var streamTask: Task<Void, Never>?
    /// The two-stage interrupt's own state, replacing a single
    /// `cancelRequested` bool that used to flip to `true` *synchronously* --
    /// before the first cooperative cancel's request had even reached the
    /// server -- so a second press arriving during that round trip
    /// force-abandoned the stream locally, ahead of any server
    /// acknowledgement. `.requesting` is the round trip itself: a press
    /// during it is a no-op, not an escalation; only `.requested` (the
    /// server has actually acknowledged the first cancel) allows a further
    /// press to escalate.
    private enum CancelState {
        case idle
        case requesting
        case requested
    }
    private var cancelState: CancelState = .idle

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
        cancelState = .idle
        promptHistory.record(prompt)
        transcript.appendUserPrompt(prompt)

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
        switch cancelState {
        case .requested:
            // The server has already acknowledged the first cooperative
            // cancel -- a further press escalates to a local force-stop.
            streamTask?.cancel()
            transcript.markCancelled()
            cancelState = .idle
        case .requesting:
            // The first cancel has not come back yet. Escalating here would
            // force-stop before the server ever acknowledged it -- exactly
            // the mid-round-trip race a single `cancelRequested` bool (set
            // `true` before the request resolved) used to allow.
            return
        case .idle:
            cancelState = .requesting
            Task { [client] in
                do {
                    try await client.cancel(runID: runID)
                    // Only this run's own outcome may advance the state
                    // machine -- a `reset()`/new run in between already
                    // reset it, and a stale completion must not overwrite
                    // that.
                    guard currentRunID == runID else { return }
                    cancelState = .requested
                } catch let error as HarnessError {
                    guard currentRunID == runID else { return }
                    connectionError = error.message
                    // A cancel that never reached the server must not leave
                    // the operator's next press escalating to a local
                    // force-kill -- it has to retry the same cooperative
                    // request.
                    cancelState = .idle
                } catch {
                    guard currentRunID == runID else { return }
                    connectionError = error.localizedDescription
                    cancelState = .idle
                }
            }
        }
    }

    public func approve(option: String? = nil) {
        guard let runID = currentRunID else { return }
        let client = self.client
        runControlTask(runID: runID) { try await client.approve(runID: runID, option: option) }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
        let client = self.client
        runControlTask(runID: runID) { try await client.deny(runID: runID) }
    }

    /// Redirects an in-flight run without cancelling it. Applied at the run's
    /// next step boundary.
    public func steer() {
        let prompt = draft.trimmed
        guard !prompt.isEmpty, let runID = currentRunID else { return }
        draft = ""
        let client = self.client
        runControlTask(runID: runID) { try await client.steer(runID: runID, prompt: prompt) }
    }

    /// Shared error-surfacing shape for the three run-control calls
    /// (`approve`/`deny`/`steer`) whose only observable effect on failure is
    /// `connectionError` -- `cancel` and `answer` also touch other state in
    /// their catch blocks, so they keep their own `do`/`catch` rather than
    /// forcing this helper to take an `onFailure` hook for two call sites.
    ///
    /// `runID` is the run this call was issued for, captured at the call
    /// site the same way `cancel`/`answer` guard their own completions: a
    /// `reset()`/new run arriving before this Task's completion must not
    /// write `connectionError` into whatever context this `RunSession` has
    /// since moved on to.
    private func runControlTask(runID: String, _ operation: @escaping () async throws -> Void) {
        Task {
            do {
                try await operation()
            } catch let error as HarnessError {
                guard currentRunID == runID else { return }
                connectionError = error.message
            } catch {
                guard currentRunID == runID else { return }
                connectionError = error.localizedDescription
            }
        }
    }

    public func answer(_ answers: [String: String]) {
        guard let runID = currentRunID, let prompt = pendingQuestions,
            AskUserAnswers.isComplete(prompt: prompt, answers: answers), !answerInFlight
        else { return }
        answerInFlight = true
        Task { [client] in
            // Always releases the guard on exit, regardless of which branch
            // below returns early -- `answerInFlight` gates *this call*, not
            // a particular run, so it must clear even when the run/prompt
            // has since moved on and the branches below skip their own
            // writes.
            defer { answerInFlight = false }
            do {
                try await client.answerInput(runID: runID, answers: answers)
                // A `reset()`/new run in between must not have this stale
                // completion write into the new context.
                guard currentRunID == runID else { return }
                // Cleared only when the prompt still pending is the one this
                // very call answered, identified by its call id -- a newer
                // question (a fresh `run.waiting_for_user`) can be assigned
                // while this request was in flight and must survive an
                // older call's success, not be silently dropped by it.
                if pendingQuestions?.callID == prompt.callID {
                    pendingQuestions = nil
                }
            } catch let error as HarnessError {
                guard currentRunID == runID else { return }
                connectionError = error.message
            } catch {
                guard currentRunID == runID else { return }
                connectionError = error.localizedDescription
            }
        }
    }

    // MARK: - Conversation switching

    public func load(messages: [StoredMessage], conversationID: String) {
        streamTask?.cancel()
        transcript.load(messages: messages)
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
    public func reconcilePersistedMessages(_ messages: [StoredMessage]) {
        guard !isBusy else { return }
        transcript.reconcile(messages: messages)
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
        conversationID = nil
        currentRunID = nil
        connectionError = nil
        pendingQuestions = nil
        // `currentRunID = nil` above already makes any in-flight cancel/
        // answer Task's completion a no-op for *this* run -- but that guard
        // only skips overwriting NEW state; it does nothing about state this
        // reset needs to clear right now. Without resetting these here, a
        // Task that never completes (or completes late, harmlessly skipped
        // by the guard above) would leave `cancelState`/`answerInFlight`
        // stuck for every conversation this same `RunSession` goes on to
        // serve after this reset.
        cancelState = .idle
        answerInFlight = false
    }

    public func rebind(conversationID: String) {
        self.conversationID = conversationID
        trackConversationStream(conversationID)
    }

    /// Recalls one entry further into the past. Returns `false` (and leaves
    /// `draft` untouched) when history is empty, the oldest entry is already
    /// showing, or an in-progress draft would be clobbered -- the caller
    /// (the composer's key handler) uses this to decide whether it handled
    /// the key press or should let the field's own navigation run instead.
    @discardableResult
    public func recallPreviousPrompt() -> Bool {
        guard let recalled = promptHistory.recallPrevious(currentDraft: draft) else {
            return false
        }
        draft = recalled
        return true
    }

    /// Recalls one entry back toward the present, restoring the pre-recall
    /// draft once navigation runs past the newest entry. Returns `false`
    /// while not currently navigating history.
    @discardableResult
    public func recallNextPrompt() -> Bool {
        guard let recalled = promptHistory.recallNext() else { return false }
        draft = recalled
        return true
    }

    /// Ends any in-progress history navigation. The composer calls this for
    /// every draft edit that did not come from a recall, so typing after
    /// recalling an entry starts a fresh navigation next time Up is pressed
    /// instead of the next arrow press silently overwriting the edit.
    public func noteManualDraftEdit() {
        promptHistory.reset()
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
                    await apply(event, runID: event.runID)
                    // A fresh app can open a durable message snapshot and then
                    // receive the same completed run in the conversation
                    // replay. Reconcile at each terminal boundary so replay
                    // cannot leave persisted assistant/tool rows duplicated.
                    // The busy guard in reconcilePersistedMessages protects a
                    // newer user-started run that begins during this fetch.
                    if event.type.isTerminal,
                        let messages = try? await client.messages(conversationID: conversationID)
                    {
                        reconcilePersistedMessages(messages)
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
    private func apply(_ event: HarnessEvent, runID: String) async {
        guard seenEventIDs.insert(event.id).inserted else { return }
        transcript.apply(event)
        await handleSideEffects(of: event, runID: runID)
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
