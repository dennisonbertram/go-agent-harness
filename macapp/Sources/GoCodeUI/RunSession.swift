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
    public private(set) var promptHistory = PromptHistory()

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
        if cancelRequested {
            streamTask?.cancel()
            transcript.markCancelled()
            return
        }
        cancelRequested = true
        Task { [client] in
            do {
                try await client.cancel(runID: runID)
            } catch let error as HarnessError {
                connectionError = error.message
                // A cancel that never reached the server must not leave the
                // operator's next press escalating to a local force-kill --
                // it has to retry the same cooperative request.
                cancelRequested = false
            } catch {
                connectionError = error.localizedDescription
                cancelRequested = false
            }
        }
    }

    public func approve(option: String? = nil) {
        guard let runID = currentRunID else { return }
        Task { [client] in
            do {
                try await client.approve(runID: runID, option: option)
            } catch let error as HarnessError {
                connectionError = error.message
            } catch {
                connectionError = error.localizedDescription
            }
        }
    }

    public func deny() {
        guard let runID = currentRunID else { return }
        Task { [client] in
            do {
                try await client.deny(runID: runID)
            } catch let error as HarnessError {
                connectionError = error.message
            } catch {
                connectionError = error.localizedDescription
            }
        }
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
        guard let runID = currentRunID, let prompt = pendingQuestions,
            AskUserAnswers.isComplete(prompt: prompt, answers: answers)
        else { return }
        Task { [client] in
            do {
                try await client.answerInput(runID: runID, answers: answers)
                // Cleared only on server acceptance -- a rejected answer
                // (e.g. the run moved on, or the answer set was incomplete
                // server-side) must leave the prompt on screen rather than
                // silently claiming it was answered.
                pendingQuestions = nil
            } catch let error as HarnessError {
                connectionError = error.message
            } catch {
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
