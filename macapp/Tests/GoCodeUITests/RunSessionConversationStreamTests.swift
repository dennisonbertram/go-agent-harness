import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP+SSE stub scoped to this file's tests.
///
/// Mirrors `HarnessKitTests.StubURLProtocol` (different test target, not
/// exported, so it can't be reused directly). Responses are queued per path:
/// each request to that path pops the next scripted response, repeating the
/// last one once the queue drains, so a test can script a dropped connection
/// followed by a reconnect without hand-rolling per-call state everywhere.
private final class ConversationStreamStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var headers: [String: String] = ["Content-Type": "application/json"]
        var chunks: [Data] = []
        /// Makes a duplicate conversation event wait until the per-run
        /// stream has delivered its terminal accounting frame. This keeps the
        /// regression deterministic: the per-run reducer owns accounting
        /// first, then the conversation replay is deduped and reconciles.
        var waitForPathToFinish: String?
    }

    nonisolated(unsafe) private static var handlers: [String: [Response]] = [:]
    nonisolated(unsafe) private static var recorded: [URLRequest] = []
    nonisolated(unsafe) private static var completedPaths: Set<String> = []
    private static let lock = NSLock()
    private static let completionCondition = NSCondition()

    static func queue(_ path: String, _ responses: [Response]) {
        lock.withLock { handlers[path] = responses }
    }

    static func reset() {
        lock.withLock {
            handlers = [:]
            recorded = []
        }
        completionCondition.lock()
        completedPaths = []
        completionCondition.unlock()
    }

    static var requests: [URLRequest] { lock.withLock { recorded } }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response: Response = Self.lock.withLock {
            Self.recorded.append(request)
            guard let path = request.url?.path, var queue = Self.handlers[path], !queue.isEmpty
            else { return Response() }
            let next = queue.removeFirst()
            Self.handlers[path] = queue.isEmpty ? [next] : queue
            return next
        }
        DispatchQueue.global().async { [weak self] in
            if let prerequisite = response.waitForPathToFinish {
                Self.waitForCompletion(of: prerequisite)
            }
            guard let self else { return }
            let http = HTTPURLResponse(
                url: request.url!, statusCode: response.status,
                httpVersion: "HTTP/1.1", headerFields: response.headers)!
            self.client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
            for chunk in response.chunks {
                self.client?.urlProtocol(self, didLoad: chunk)
            }
            self.client?.urlProtocolDidFinishLoading(self)
            if let path = request.url?.path {
                Self.markCompleted(path)
            }
        }
    }

    override func stopLoading() {}

    private static func waitForCompletion(of path: String) {
        completionCondition.lock()
        defer { completionCondition.unlock() }
        let deadline = Date().addingTimeInterval(5)
        while !completedPaths.contains(path), completionCondition.wait(until: deadline) {}
    }

    private static func markCompleted(_ path: String) {
        completionCondition.lock()
        completedPaths.insert(path)
        completionCondition.broadcast()
        completionCondition.unlock()
    }
}

/// Exercises the fix for issue #950: harnessd exposes a conversation-wide SSE
/// stream (`GET /v1/conversations/{id}/events`) so a delayed callback or cron
/// run that fires after the run that scheduled it already ended still reaches
/// the app. These tests drive `RunSession` directly against the stub above,
/// the same layer `RunSessionLiveTests` exercises against a real harnessd.
@Suite("RunSession conversation-wide event stream", .serialized)
@MainActor
struct RunSessionConversationStreamTests {

    private func makeSession() -> RunSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ConversationStreamStub.self]
        let client = HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:8899")!,
            session: URLSession(configuration: config))
        return RunSession(client: client)
    }

    private func wait(
        timeout: Duration = .seconds(5), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(20))
        }
        Issue.record("timed out waiting for condition")
    }

    private func hasAssistantText(_ session: RunSession, _ text: String) -> Bool {
        session.transcript.items.contains {
            if case .assistantMessage(let message) = $0.kind { return message.text == text }
            return false
        }
    }

    /// BT-002: a run this app instance never called `startRun`/`submit()` for
    /// -- exactly the shape of a fired delayed callback -- must still render
    /// in the transcript, because opening a conversation subscribes to every
    /// run on it, not just the ones this client started.
    @Test("renders output from a run this app never started")
    func rendersUnstartedRunEvents() async throws {
        ConversationStreamStub.reset()
        let frames = """
            id: run_cb:0
            event: assistant.message
            data: {"id":"run_cb:0","run_id":"run_cb","type":"assistant.message","payload":{"content":"CALLBACK_SURFACED_IN_UI"}}

            id: run_cb:1
            event: run.completed
            data: {"id":"run_cb:1","run_id":"run_cb","type":"run.completed","payload":{}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_1/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_1")

        try await wait { hasAssistantText(session, "CALLBACK_SURFACED_IN_UI") }

        session.reset()
    }

    /// Regression for requirement 4: without the dedup in `apply(_:runID:)`,
    /// a run this app *did* start renders twice, because submit()'s per-run
    /// stream and the conversation-wide stream both observe the same events
    /// for it. Reverting the `seenEventIDs` guard added in the green commit
    /// makes this fail by producing two assistant-message items instead of
    /// one.
    @Test("does not double-render a self-started run's events across both streams")
    func dedupesEventsSeenOnBothStreams() async throws {
        ConversationStreamStub.reset()
        let frames = """
            id: run_1:0
            event: run.started
            data: {"id":"run_1:0","run_id":"run_1","type":"run.started","payload":{}}

            id: run_1:1
            event: assistant.message
            data: {"id":"run_1:1","run_id":"run_1","type":"assistant.message","payload":{"content":"hello there"}}

            id: run_1:2
            event: run.completed
            data: {"id":"run_1:2","run_id":"run_1","type":"run.completed","payload":{"usage_totals":{"prompt_tokens_total":120,"completion_tokens_total":10,"total_tokens":130},"cost_totals":{"cost_usd_total":0.0025,"cost_status":"available"}}}


            """
        ConversationStreamStub.queue(
            "/v1/runs",
            [.init(status: 202, chunks: [Data(#"{"run_id":"run_1","status":"queued"}"#.utf8)])])
        ConversationStreamStub.queue(
            "/v1/runs/run_1/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/run_1/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)],
                    waitForPathToFinish: "/v1/runs/run_1/events")
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/run_1/messages",
            [
                .init(
                    status: 200,
                    chunks: [
                        Data(
                            #"{"messages":[{"role":"user","content":"hi","step":0},{"role":"assistant","content":"hello there","step":0}]}"#
                                .utf8)
                    ])
            ])

        let session = makeSession()
        session.draft = "hi"
        session.submit()

        try await wait { session.transcript.runState == .completed }
        // Both streams deliver identical frames concurrently; give the
        // conversation-wide one a moment to also arrive before asserting
        // nothing doubled.
        try await Task.sleep(for: .milliseconds(200))

        let assistantMessages = session.transcript.items.filter {
            if case .assistantMessage = $0.kind { return true }
            return false
        }
        #expect(
            assistantMessages.count == 1,
            "the same event arrived on the per-run stream and the conversation stream and was rendered twice"
        )
        // The per-run stream wins the terminal race; the conversation copy is
        // deduped but still requests durable rows. That reconciliation must
        // retain the accounting owned by run_1 rather than clear it because
        // the duplicate frame was not newly reduced.
        try await wait {
            ConversationStreamStub.requests.contains {
                $0.url?.path == "/v1/conversations/run_1/messages"
            }
        }
        #expect(session.transcript.usage.promptTokens == 120)
        #expect(session.transcript.usage.completionTokens == 10)
        #expect(session.transcript.usage.totalTokens == 130)
        #expect(session.transcript.usage.costUSD == 0.0025)
        #expect(session.transcript.usage.costStatus == "available")

        session.reset()
    }

    /// Regression for requirement 5: a conversation stream that silently dies
    /// is the same bug the whole feature exists to fix. If the reconnect loop
    /// in `streamConversation` were removed, only "first" would ever arrive
    /// and the request recorded for `/v1/conversations/conv_2/events` would
    /// stay at one with no `Last-Event-ID` on any follow-up.
    @Test("reconnects the conversation stream with Last-Event-ID after a drop")
    func reconnectsAfterConnectionDrop() async throws {
        ConversationStreamStub.reset()
        let firstFrames = """
            id: run_cb:0
            event: assistant.message
            data: {"id":"run_cb:0","run_id":"run_cb","type":"assistant.message","payload":{"content":"first"}}


            """
        let secondFrames = """
            id: run_cb:1
            event: assistant.message
            data: {"id":"run_cb:1","run_id":"run_cb","type":"assistant.message","payload":{"content":"second"}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_2/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(firstFrames.utf8)]),
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(secondFrames.utf8)]),
            ])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_2")

        try await wait { hasAssistantText(session, "second") }

        let requests = ConversationStreamStub.requests.filter {
            $0.url?.path == "/v1/conversations/conv_2/events"
        }
        #expect(requests.count >= 2, "expected a reconnect after the first response ended")
        #expect(requests[1].value(forHTTPHeaderField: "Last-Event-ID") == "run_cb:0")

        session.reset()
    }

    /// Durable conversation replay is intentionally complete, including runs
    /// that were already represented by the message snapshot used to open a
    /// historical conversation. A terminal replay must reconcile back to that
    /// snapshot instead of leaving the same assistant reply rendered twice.
    @Test("completed replay does not duplicate an already persisted reply")
    func completedReplayReconcilesPersistedSnapshot() async throws {
        ConversationStreamStub.reset()
        let storedJSON = """
            {"messages":[
              {"role":"user","content":"watch deployment","step":0},
              {"role":"assistant","content":"DEPLOYMENT_PASSED","step":0}
            ]}
            """
        let storedMessages = try JSONDecoder().decode(
            StoredMessageEnvelope.self,
            from: Data(storedJSON.utf8)
        ).messages
        let frames = """
            id: run_cron:0
            event: assistant.message
            data: {"id":"run_cron:0","run_id":"run_cron","type":"assistant.message","payload":{"content":"DEPLOYMENT_PASSED"}}

            id: run_cron:1
            event: run.completed
            data: {"id":"run_cron:1","run_id":"run_cron","type":"run.completed","payload":{}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_persisted/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/conv_persisted/messages",
            [.init(status: 200, chunks: [Data(storedJSON.utf8)])])

        let session = makeSession()
        session.load(messages: storedMessages, conversationID: "conv_persisted")

        try await wait {
            ConversationStreamStub.requests.contains {
                $0.url?.path == "/v1/conversations/conv_persisted/messages"
            }
        }

        let matchingReplies = session.transcript.items.filter {
            if case .assistantMessage(let message) = $0.kind {
                return message.text == "DEPLOYMENT_PASSED"
            }
            return false
        }
        #expect(matchingReplies.count == 1)

        session.reset()
    }

    /// A later scheduled run can begin after the conversation previously
    /// completed with priced usage. Its empty terminal snapshot is still the
    /// new run's accounting state; retaining the prior run's cost/tokens lies
    /// about what this visible completion consumed.
    @Test("an incomplete later terminal clears prior run accounting")
    func incompleteLaterTerminalClearsPriorUsage() async throws {
        ConversationStreamStub.reset()
        let frames = """
            id: run_a:0
            event: run.started
            data: {"id":"run_a:0","run_id":"run_a","type":"run.started","timestamp":"2026-08-03T00:00:01Z","payload":{}}

            id: run_a:1
            event: usage.delta
            data: {"id":"run_a:1","run_id":"run_a","type":"usage.delta","timestamp":"2026-08-03T00:00:02Z","payload":{"cumulative_usage":{"prompt_tokens":120,"completion_tokens":10,"total_tokens":130},"cumulative_cost_usd":0.0025,"cost_status":"available"}}

            id: run_a:2
            event: run.completed
            data: {"id":"run_a:2","run_id":"run_a","type":"run.completed","timestamp":"2026-08-03T00:00:03Z","payload":{"usage_totals":{"prompt_tokens_total":120,"completion_tokens_total":10,"total_tokens":130},"cost_totals":{"cost_usd_total":0.0025,"cost_status":"available"}}}

            id: run_b:0
            event: run.started
            data: {"id":"run_b:0","run_id":"run_b","type":"run.started","timestamp":"2026-08-03T00:00:04Z","payload":{}}

            id: run_b:1
            event: run.completed
            data: {"id":"run_b:1","run_id":"run_b","type":"run.completed","timestamp":"2026-08-03T00:00:05Z","payload":{}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_accounting/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/conv_accounting/messages",
            [.init(status: 200, chunks: [Data(#"{"messages":[]}"#.utf8)])])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_accounting")

        // Both terminal frames trigger a durable message reconciliation. Do
        // not stop at the first `.completed` state: that would prove run A
        // only and miss the run-B overwrite this regression guards.
        try await wait {
            ConversationStreamStub.requests.filter {
                $0.url?.path == "/v1/conversations/conv_accounting/messages"
            }.count >= 2
        }
        #expect(session.transcript.usage.promptTokens == 0)
        #expect(session.transcript.usage.completionTokens == 0)
        #expect(session.transcript.usage.totalTokens == 0)
        #expect(session.transcript.usage.costUSD == 0)
        #expect(session.transcript.usage.costStatus == "pending")
        session.reset()
    }

    /// A submit that fails before the new run's stream can supply accounting
    /// must not keep displaying a previous scheduled run's totals.
    @Test("a local failure clears previous run accounting")
    func localFailureClearsPriorUsage() async throws {
        ConversationStreamStub.reset()
        let firstRun = """
            id: run_a:0
            event: run.started
            data: {"id":"run_a:0","run_id":"run_a","type":"run.started","timestamp":"2026-08-03T00:00:01Z","payload":{}}

            id: run_a:1
            event: run.completed
            data: {"id":"run_a:1","run_id":"run_a","type":"run.completed","timestamp":"2026-08-03T00:00:02Z","payload":{"usage_totals":{"total_tokens":130},"cost_totals":{"cost_usd_total":0.0025,"cost_status":"available"}}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_local_failure/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(firstRun.utf8)])
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/conv_local_failure/messages",
            [.init(status: 200, chunks: [Data(#"{"messages":[]}"#.utf8)])])
        ConversationStreamStub.queue(
            "/v1/runs",
            [.init(status: 202, chunks: [Data(#"{"run_id":"run_b","status":"queued"}"#.utf8)])])
        ConversationStreamStub.queue(
            "/v1/runs/run_b/events",
            [.init(status: 500, chunks: [Data(#"{"error":"stream unavailable"}"#.utf8)])])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_local_failure")
        try await wait { session.transcript.usage.totalTokens == 130 }

        session.draft = "start a new check"
        session.submit()
        try await wait { session.transcript.runState == .failed }

        #expect(session.transcript.usage.totalTokens == 0)
        #expect(session.transcript.usage.costUSD == 0)
        #expect(session.transcript.usage.costStatus == "pending")
        session.reset()
    }

    /// Regression for #1028: durable row reconciliation after a terminal event
    /// must not reinterpret an authoritative failure as success or discard the
    /// event-only error detail.
    @Test("failed replay reconciliation preserves failure state and detail")
    func failedReplayReconciliationPreservesFailureState() async throws {
        ConversationStreamStub.reset()
        let storedJSON = """
            {"messages":[
              {"role":"user","content":"check deployment","step":0}
            ]}
            """
        let frames = """
            id: run_failed:0
            event: run.failed
            data: {"id":"run_failed:0","run_id":"run_failed","type":"run.failed","payload":{"error":"deployment probe failed"}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_failed/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/conv_failed/messages",
            [.init(status: 200, chunks: [Data(storedJSON.utf8)])])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_failed")

        try await wait {
            ConversationStreamStub.requests.contains {
                $0.url?.path == "/v1/conversations/conv_failed/messages"
            }
        }
        #expect(session.transcript.runState == .failed)
        #expect(
            session.transcript.items.contains {
                if case .error(let message) = $0.kind {
                    return message == "deployment probe failed"
                }
                return false
            },
            "terminal replay reconciliation discarded event-derived failure detail")

        session.reset()
    }

    /// Cancellation is also authoritative terminal state even though it carries
    /// no durable error message.
    @Test("cancelled replay reconciliation preserves cancelled state")
    func cancelledReplayReconciliationPreservesCancelledState() async throws {
        ConversationStreamStub.reset()
        let storedJSON = """
            {"messages":[
              {"role":"user","content":"stop deployment check","step":0}
            ]}
            """
        let frames = """
            id: run_cancelled:0
            event: run.cancelled
            data: {"id":"run_cancelled:0","run_id":"run_cancelled","type":"run.cancelled","payload":{}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_cancelled/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])
        ConversationStreamStub.queue(
            "/v1/conversations/conv_cancelled/messages",
            [.init(status: 200, chunks: [Data(storedJSON.utf8)])])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_cancelled")

        try await wait {
            ConversationStreamStub.requests.contains {
                $0.url?.path == "/v1/conversations/conv_cancelled/messages"
            }
        }
        #expect(session.transcript.runState == .cancelled)

        session.reset()
    }
}

private struct StoredMessageEnvelope: Decodable {
    let messages: [StoredMessage]
}
