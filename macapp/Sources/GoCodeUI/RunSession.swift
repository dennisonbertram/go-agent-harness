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
    public internal(set) var transcript = Transcript()
    public internal(set) var connectionError: String?
    public internal(set) var conversationID: String?
    public internal(set) var currentRunID: String?
    /// Set when the agent asks a structured question mid-run.
    public internal(set) var pendingQuestions: AskUserPrompt?
    public internal(set) var answerInFlight = false
    public internal(set) var runControlInFlight = false

    public var draft: String = ""
    public var model: String?
    public var planMode = false
    public var extraDirs: [String] = []
    public var profile: String?
    /// Recalled with Up/Down in the composer.
    public private(set) var promptHistory: [String] = []

    let client: HarnessClient
    var streamTask: Task<Void, Never>?
    enum CancelState { case idle, requesting, requested }
    var cancelState: CancelState = .idle
    var answerRequestGeneration: UInt = 0
    var pendingInputRequestGeneration: UInt = 0
    var runControlRequestGeneration: UInt = 0
    /// A successful approve/deny remains disabled until the run's own SSE
    /// stream confirms that the decision advanced. HTTP 2xx only says the
    /// daemon accepted the request, not that the run has consumed it.
    var acknowledgedRunControlRunID: String?
    var runControlLifecycleGenerationByRunID: [String: UInt] = [:]

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
    /// Active runs in activation order. A conversation stream may legitimately
    /// contain overlapping local and scheduled work, but the native transcript
    /// has one visual lifecycle and one action target.
    private var activeRunIDs: [String] = []
    private var externalRunIDs: Set<String> = []
    private var activeRunTimestamps: [String: Date?] = [:]
    /// A terminal run may be replayed after a reconnect. Do not let a late
    /// non-terminal frame resurrect it as the action target.
    private var terminalRunIDs: Set<String> = []
    /// Only the per-run stream for this local run may be force-cancelled by a
    /// second Stop press. A scheduled run must never cancel an unrelated local
    /// stream merely because it is currently selected.
    var localStreamRunID: String?
    /// One RunSubmission stream can survive visual displacement while a later
    /// C stream starts. Reset/load must detach every such local stream, not
    /// only the last compatibility `streamTask`.
    private var submissionStreamTasks: [ObjectIdentifier: Task<Void, Never>] = [:]
    /// The locally submitted run whose caller may need A-only lifecycle and
    /// transcript evidence. A selected external continuation displaces this
    /// handle; it must never cause its caller to act on the continuation.
    private var activeSubmission: RunSubmission?
    /// Each submission captures this unforgeable owner token and the current
    /// generation. It remains independent from selected-run UI state.
    let submissionOwnerToken = UUID()
    /// The single monotonic source used to derive and observe submission
    /// deadlines. Production uses `ContinuousClock.now`; the internal
    /// initializer makes deterministic native timing tests possible without
    /// exposing a clock choice to GUI or ToolWalk callers.
    let submissionTimeoutNow: @MainActor () -> ContinuousClock.Instant
    /// Reset/load detach the old session permanently; their generation invalidates
    /// every outstanding submission timeout capability.
    var submissionGeneration: UInt = 0
    var submissionTimeoutGates: [ObjectIdentifier: SubmissionTimeoutGate] = [:]

    public init(client: HarnessClient) {
        self.client = client
        submissionTimeoutNow = { ContinuousClock.now }
    }

    init(
        client: HarnessClient,
        submissionTimeoutNow: @escaping @MainActor () -> ContinuousClock.Instant
    ) {
        self.client = client
        self.submissionTimeoutNow = submissionTimeoutNow
    }

    public convenience init(baseURL: URL, token: String? = nil) {
        self.init(client: HarnessClient(baseURL: baseURL, token: token))
    }

    public var isBusy: Bool {
        transcript.runState.isActive
    }

    /// Keyboard submission must share the composer button's single-flight
    /// boundary. A control POST can outlive its run's terminal SSE; allowing
    /// a new run during that acknowledgement would let the old completion
    /// mutate the newer conversation.
    public var canSubmit: Bool {
        !draft.trimmed.isEmpty && !isBusy && !runControlInFlight
    }

    /// True while a run is active, so the composer can offer steering instead.
    public var canSteer: Bool {
        isBusy && transcript.pendingApproval == nil
    }

    /// Accessible copy shown while a cron/callback continuation, rather than a
    /// prompt submitted by this app instance, owns the active controls.
    public var scheduledRunStatus: String? {
        guard let currentRunID, externalRunIDs.contains(currentRunID), isBusy else { return nil }
        return "Scheduled run active"
    }

    // MARK: - Running

    @discardableResult
    public func submit() -> RunSubmission? {
        submit(timeoutAfter: nil)
    }

    /// ToolWalk's bounded execution path is the sole caller permitted to bind
    /// a timeout policy to a submission. GUI callers intentionally receive
    /// the parameter-free overload above.
    @discardableResult
    package func submit(timeoutAfter: Duration?) -> RunSubmission? {
        let prompt = draft.trimmed
        guard !prompt.isEmpty, !isBusy, !runControlInFlight else { return nil }
        let submission = RunSubmission(
            prompt: prompt, timeoutOwner: submissionOwnerToken,
            timeoutGeneration: submissionGeneration, timeoutAfter: timeoutAfter,
            timeoutNow: submissionTimeoutNow
        )
        activeSubmission = submission
        draft = ""
        connectionError = nil
        cancelState = .idle
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
        let submissionID = ObjectIdentifier(submission)
        let task = Task {
            [client, model, planMode, startingConversationID = conversationID, extraDirs, profile]
            in
            var startedRunID: String?
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
                startedRunID = started.runID
                // A reset/load can cancel this task while its HTTP response
                // races back. The server may have admitted A, but a torn-down
                // session must not revive it or let it displace whatever
                // conversation the user selected next.
                submission.markStarted(runID: started.runID)
                // The server accepted A even if scheduled B became visible
                // while the HTTP response was in flight. Bind that immutable
                // identity to A's handle for truthful local outcome reporting,
                // but never let the late response steal B's selection,
                // accounting, or conversation stream.
                if ownsVisibleSubmission(submission, runID: started.runID) {
                    localStreamRunID = started.runID
                    activate(runID: started.runID, isExternal: false, timestamp: nil, select: true)
                    activateAccounting(for: started.runID, timestamp: nil)
                    if self.conversationID == nil { self.conversationID = started.runID }
                    // Keyed by conversation, not by this run: on a conversation's
                    // later runs `self.conversationID` is already the first run's
                    // id, so this must not retarget the stream to `started.runID`.
                    if let conversationID = self.conversationID {
                        trackConversationStream(conversationID)
                    }
                }

                for try await event in client.events(runID: started.runID) {
                    submission.apply(event)
                    // Once B owns the visible conversation, A's stream is
                    // private evidence for the submission handle. Feeding it
                    // through the shared reducer would let an old A mutate
                    // B's lifecycle/accounting despite the selected-run fence.
                    if !submission.isDisplaced {
                        await applyRunEvent(event, expectedRunID: started.runID)
                    }
                }
                if !submission.isTerminal {
                    recordSubmissionFailure(
                        submission,
                        runID: started.runID,
                        message: "run event stream ended before a terminal event"
                    )
                }
            } catch is CancellationError {
                // reset/load intentionally detaches the handle and cancels its
                // stream. That is not an A transport failure.
            } catch let error as HarnessError {
                recordSubmissionFailure(submission, runID: startedRunID, message: error.message)
                if let startedRunID { releaseUnstartedAccounting(for: startedRunID) }
            } catch {
                if !Task.isCancelled {
                    recordSubmissionFailure(
                        submission, runID: startedRunID, message: error.localizedDescription
                    )
                }
                if let startedRunID { releaseUnstartedAccounting(for: startedRunID) }
            }
            finishRunIfCurrent(startedRunID: startedRunID, submission: submission)
            submissionStreamTasks.removeValue(forKey: submissionID)
        }
        streamTask = task
        submissionStreamTasks[submissionID] = task
        return submission
    }

    // MARK: - Conversation switching

    public func load(messages: [StoredMessage], conversationID: String) {
        streamTask?.cancel()
        invalidateRequestOwnership()
        transcript.load(messages: messages)
        accountingRunID = nil
        accountingTimestamp = nil
        self.conversationID = conversationID
        clearActiveRuns()
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
            preservingRunState: preservingRunState
        )
        if !preserveAccounting {
            accountingRunID = nil
            accountingTimestamp = nil
        }
        connectionError = nil
        clearPendingInteractions()
    }

    /// Stops following this session's conversation. Called on a fresh
    /// conversation, and by `ProjectSession.shutdown()` so a torn-down
    /// harnessd does not leave the reconnect loop spinning against a process
    /// that will never answer again.
    public func reset() {
        streamTask?.cancel()
        stopConversationStream()
        invalidateRequestOwnership()
        transcript.reset()
        accountingRunID = nil
        accountingTimestamp = nil
        conversationID = nil
        clearActiveRuns()
        connectionError = nil
        clearPendingInteractions()
        cancelState = .idle
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
                    conversationID: conversationID, lastEventID: lastEventID
                ) {
                    lastEventID = event.id
                    await applyConversationEvent(event, conversationID: conversationID)
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
                            preservingRunState: isStaleTerminal
                        )
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
    ///
    /// Control ownership is deliberately decided before accounting. A foreign
    /// terminal can be newer in wall-clock time while a different run remains
    /// selected; it must not steal accounting or clear the visible controls.
    @discardableResult
    func apply(_ event: HarnessEvent, runID: String) async -> Bool {
        guard !event.runID.isEmpty, seenEventIDs.insert(event.id).inserted else { return false }

        let wasSelected = currentRunID == event.runID
        let hadSelection = currentRunID != nil
        if event.type.isTerminal {
            // A terminal for another active/replayed run is authoritative for
            // that run alone. Remember it so late replay frames cannot revive
            // it, but do not let it mutate the selected run's visual state.
            guard wasSelected || !hadSelection else {
                retire(runID: event.runID)
                return false
            }
            let includedAccounting = admitAccounting(for: event)
            // A terminal-only replay is authoritative only when it can own
            // accounting. Once a newer terminal/local run owns accounting,
            // an old terminal is still retained for durable reconciliation but
            // cannot replace that newer visual terminal state.
            if !includedAccounting, let accountingRunID, accountingRunID != event.runID {
                retire(runID: event.runID, terminalTimestamp: event.timestamp)
                return false
            }
            transcript.apply(event, includingAccounting: includedAccounting)
            advanceAcknowledgedRunControl(for: event)
            retire(runID: event.runID, terminalTimestamp: event.timestamp)
            // Terminal must render before restoring the fallback run's active
            // state. This preserves the durable terminal row while returning
            // Stop/steer/input authority to the still-live continuation.
            if let fallback = currentRunID {
                activateAccounting(for: fallback, timestamp: activeRunTimestamps[fallback] ?? nil)
                transcript.resumeActiveRun()
            }
            return includedAccounting
        }

        let admitsControlEvidence = registerActiveEvidence(for: event)
        let appliesToSelectedRun = currentRunID == event.runID
        let includedAccounting = appliesToSelectedRun && admitAccounting(for: event)
        if (!admitsControlEvidence && eventChangesLifecycleOrControls(event))
            || suppressesForeignStateMutation(event, includedAccounting: includedAccounting)
        {
            return false
        }
        if appliesToSelectedRun, transcript.runState.isTerminalState {
            // Reconnects can start at an assistant/tool frame after the
            // `run.started` cursor was trimmed. First active external evidence
            // must visibly continue the conversation, not merely set an ID.
            transcript.resumeActiveRun()
        }
        transcript.apply(event, includingAccounting: includedAccounting)
        if appliesToSelectedRun { advanceAcknowledgedRunControl(for: event) }
        await handleSideEffects(of: event, runID: runID, appliesToSelectedRun: appliesToSelectedRun)
        return includedAccounting
    }

    /// The conversation entry point is internal for stale-stream acceptance
    /// tests. A cancelled previous conversation stream can still yield once.
    func applyConversationEvent(_ event: HarnessEvent, conversationID: String) async {
        guard self.conversationID == conversationID else { return }
        _ = await apply(event, runID: event.runID)
    }

    private func applyRunEvent(_ event: HarnessEvent, expectedRunID: String) async {
        guard event.runID == expectedRunID else { return }
        _ = await apply(event, runID: expectedRunID)
    }

    private func registerActiveEvidence(for event: HarnessEvent) -> Bool {
        guard !terminalRunIDs.contains(event.runID) else { return false }
        let isLifecycleStart =
            event.type == .runQueued || event.type == .runStarted || event.type == .runResumed
        let select: Bool =
            if currentRunID == nil {
                // After a terminal leaves no live action target, a replay start
                // still must not displace newer accounting. Timestamp-less legacy
                // frames remain admissible because their order cannot be compared.
                if let accountingTimestamp, let timestamp = event.timestamp {
                    timestamp >= accountingTimestamp
                } else {
                    true
                }
            } else if isLifecycleStart, let currentRunID {
                // A local `startRun` result is a provisional owner until its first
                // timestamped event. Do not replace it with an older replay simply
                // because the replay supplies a timestamp first.
                if activeRunTimestamps[currentRunID] == nil, !externalRunIDs.contains(currentRunID)
                {
                    false
                } else if let timestamp = event.timestamp,
                    let current = activeRunTimestamps[currentRunID] ?? nil
                {
                    timestamp >= current
                } else {
                    false
                }
            } else {
                false
            }
        if !activeRunIDs.contains(event.runID) {
            activate(
                runID: event.runID, isExternal: event.runID != localStreamRunID,
                timestamp: event.timestamp, select: select
            )
        } else if event.runID == currentRunID, let timestamp = event.timestamp {
            // A locally admitted run is provisional only until its own first
            // timestamped lifecycle evidence arrives. Preserve that timestamp
            // so a genuinely later scheduled continuation can take visual
            // ownership, while an older replay still cannot.
            activeRunTimestamps[event.runID] = timestamp
        } else if select {
            selectActive(runID: event.runID)
        }
        return true
    }

    private func eventChangesLifecycleOrControls(_ event: HarnessEvent) -> Bool {
        switch event.type {
        case .runQueued, .runStarted, .runResumed,
            .toolApprovalRequired, .planApprovalRequired,
            .toolApprovalGranted, .toolApprovalDenied,
            .planApprovalGranted, .planApprovalDenied,
            .runWaitingForUser:
            true
        default:
            false
        }
    }

    private func activate(runID: String, isExternal: Bool, timestamp: Date?, select: Bool) {
        activeRunIDs.removeAll { $0 == runID }
        activeRunIDs.append(runID)
        activeRunTimestamps[runID] = timestamp
        if isExternal { externalRunIDs.insert(runID) } else { externalRunIDs.remove(runID) }
        if select { selectActive(runID: runID) }
    }

    private func selectActive(runID: String) {
        guard currentRunID != runID else { return }
        if let activeSubmission, activeSubmission.runID != runID {
            activeSubmission.markDisplaced()
        }
        invalidateRequestOwnership()
        clearPendingInteractions()
        currentRunID = runID
        cancelState = .idle
    }

    func retire(runID: String, terminalTimestamp: Date? = nil) {
        terminalRunIDs.insert(runID)
        activeRunIDs.removeAll { $0 == runID }
        externalRunIDs.remove(runID)
        activeRunTimestamps.removeValue(forKey: runID)
        guard currentRunID == runID else { return }
        clearPendingInteractions()
        // An older run may still have an active row, but it is not a valid
        // fallback after a newer terminal. Only a run activated no earlier
        // than this terminal may resume the visible lifecycle.
        let fallback = activeRunIDs.last.flatMap { candidate -> String? in
            guard let terminalTimestamp else { return candidate }
            guard let candidateTimestamp = activeRunTimestamps[candidate] ?? nil,
                candidateTimestamp >= terminalTimestamp
            else { return nil }
            return candidate
        }
        currentRunID = fallback
        // A terminal of the run that owns a still-pending control POST must
        // not invalidate that request. Its completion decides whether the
        // composer is released or an error/draft is restored. Switching to a
        // real fallback does invalidate displaced A ownership.
        if fallback != nil { invalidateRequestOwnership() }
        cancelState = .idle
    }

    private func finishRunIfCurrent(startedRunID: String?, submission: RunSubmission) {
        submissionStreamTasks.removeValue(forKey: ObjectIdentifier(submission))
        guard let startedRunID else { return }
        if localStreamRunID == startedRunID { localStreamRunID = nil }
        // A late completion must never clear a newer submission solely because
        // both runs have sequentially occupied this session.
        if activeSubmission === submission { activeSubmission = nil }
        // A stream can end after an external continuation became selected. It
        // may retire its own run, never clear the newer selected target.
        if currentRunID == startedRunID {
            retire(runID: startedRunID)
        } else {
            activeRunIDs.removeAll { $0 == startedRunID }
            externalRunIDs.remove(startedRunID)
            activeRunTimestamps.removeValue(forKey: startedRunID)
        }
    }

    private func clearActiveRuns() {
        submissionGeneration &+= 1
        for task in submissionStreamTasks.values {
            task.cancel()
        }
        submissionStreamTasks = [:]
        submissionTimeoutGates = [:]
        activeSubmission?.markDisplaced()
        activeSubmission = nil
        activeRunIDs = []
        externalRunIDs = []
        activeRunTimestamps = [:]
        terminalRunIDs = []
        currentRunID = nil
        clearPendingInteractions()
        localStreamRunID = nil
        cancelState = .idle
    }

    /// Shared visible state belongs to the exact `RunSubmission` that still
    /// owns selected A. A displacement (including one before `startRun`
    /// acknowledges) makes all later A errors handle-local.
    private func ownsVisibleSubmission(_ submission: RunSubmission, runID: String?) -> Bool {
        guard activeSubmission === submission, !submission.isDisplaced else { return false }
        guard let runID else { return currentRunID == nil }
        return currentRunID == nil || currentRunID == runID
    }

    func consumeTimeoutCancellation(for submission: RunSubmission) -> String? {
        submission.consumeTimeoutCancellation(
            owner: submissionOwnerToken, generation: submissionGeneration
        )
    }

    private func recordSubmissionFailure(
        _ submission: RunSubmission, runID: String?, message: String
    ) {
        submission.markFailed(message)
        guard ownsVisibleSubmission(submission, runID: runID) else { return }
        connectionError = message
        transcript.markFailed()
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

    /// A local stream can fail after `startRun` accepted it but before its
    /// first event supplies an ordering timestamp. Release only that
    /// unobserved provisional owner: it must not block a later timestamped
    /// external run, while a queued local run remains protected from stale
    /// replay until the failure is actually known.
    private func releaseUnstartedAccounting(for runID: String) {
        guard accountingRunID == runID, accountingTimestamp == nil else { return }
        accountingRunID = nil
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
        let isStart =
            switch event.type {
            case .runQueued, .runStarted, .runResumed: true
            default: false
            }
        if accountingRunID == runID {
            if let timestamp = event.timestamp { accountingTimestamp = timestamp }
            return true
        }
        // The control reducer selected this lifecycle start first. That is the
        // ordering authority for timestamp-less compatibility frames; the
        // comparison below remains for terminal-only replay.
        if isStart, currentRunID == runID {
            activateAccounting(for: runID, timestamp: event.timestamp)
            return true
        }
        let isNewer: Bool =
            if let timestamp = event.timestamp {
                // `submit()` owns the run before its first SSE frame supplies a
                // timestamp. That provisional ownership is still authoritative:
                // a replay from another run must not steal it merely because it
                // has a timestamp while the current owner does not yet have one.
                accountingTimestamp.map { timestamp >= $0 } ?? false
            } else {
                false
            }
        if isStart, accountingRunID == nil || isNewer {
            activateAccounting(for: runID, timestamp: event.timestamp)
            return true
        }
        if event.type.isTerminal, accountingRunID == nil || isNewer {
            activateAccounting(for: runID, timestamp: event.timestamp)
            return true
        }
        return false
    }

    // MARK: - Side effects

    private func handleSideEffects(
        of event: HarnessEvent, runID: String, appliesToSelectedRun: Bool
    ) async {
        // The question text lives behind a separate endpoint, not in the event.
        guard event.type == .runWaitingForUser || event.type == .other("run.waiting_for_user")
        else { return }
        guard appliesToSelectedRun, currentRunID == runID else { return }
        pendingInputRequestGeneration &+= 1
        let generation = pendingInputRequestGeneration
        do {
            let prompt = try await client.pendingInput(runID: runID)
            guard currentRunID == runID, pendingInputRequestGeneration == generation else { return }
            pendingQuestions = prompt
        } catch let error as HarnessError {
            guard currentRunID == runID, pendingInputRequestGeneration == generation else { return }
            connectionError = error.message
        } catch {
            guard currentRunID == runID, pendingInputRequestGeneration == generation else { return }
            connectionError = error.localizedDescription
        }
    }

    private func invalidateRequestOwnership() {
        answerRequestGeneration &+= 1
        pendingInputRequestGeneration &+= 1
        runControlRequestGeneration &+= 1
        answerInFlight = false
        runControlInFlight = false
        acknowledgedRunControlRunID = nil
    }

    /// Pending UI is owned by `currentRunID`, not merely by the transcript.
    /// Clear synchronously at each ownership fence so the following SwiftUI
    /// render cannot offer A's action against B while async fetches unwind.
    private func clearPendingInteractions() {
        transcript.clearPendingInteractions()
        pendingQuestions = nil
    }

    private func advanceAcknowledgedRunControl(for event: HarnessEvent) {
        switch event.type {
        case .toolApprovalGranted, .toolApprovalDenied,
            .planApprovalGranted, .planApprovalDenied,
            .runCompleted, .runFailed, .runCancelled:
            runControlLifecycleGenerationByRunID[event.runID, default: 0] &+= 1
            if event.runID == acknowledgedRunControlRunID {
                acknowledgedRunControlRunID = nil
                runControlInFlight = false
            }
        default:
            break
        }
    }
}

extension String {
    var trimmed: String {
        trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

extension RunState {
    fileprivate var isTerminalState: Bool {
        switch self {
        case .completed, .failed, .cancelled: true
        default: false
        }
    }
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
