import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP+SSE stub scoped to this file's tests, keyed on HTTP method
/// *and* path (not path alone), because `GET /v1/runs/{id}/input`
/// (`pendingInput`) and `POST /v1/runs/{id}/input` (`answerInput`) share a
/// path and must be scripted independently.
private final class RunControlStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var headers: [String: String] = ["Content-Type": "application/json"]
        var body: Data = Data()
        /// Simulates a connection that never terminates. Used for the run's
        /// (and conversation's) `/events` stream so `RunSession.currentRunID`
        /// stays set for the duration of a test instead of clearing the
        /// moment an empty stream finishes normally.
        var neverFinishes = false
    }

    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    nonisolated(unsafe) private static var recorded: [URLRequest] = []
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    static func reset() {
        lock.withLock {
            handler = nil
            recorded = []
        }
    }

    static func requests(matching path: String) -> [URLRequest] {
        lock.withLock { recorded.filter { $0.url?.path == path } }
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response = Self.lock.withLock {
            Self.recorded.append(request)
            return Self.handler?(request) ?? Response()
        }
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: response.headers)!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        if !response.neverFinishes {
            client?.urlProtocolDidFinishLoading(self)
        }
    }

    override func stopLoading() {}
}

/// Exercises the fix for #994 (F3): `RunSession.cancel/approve/deny/answer`
/// discarded their server acknowledgement with `try?`, so a rejected or
/// failed call left the UI asserting an action had succeeded when it had
/// not. These tests drive `RunSession` directly against the stub above, the
/// same shape `RunSessionConversationStreamTests` uses.
@Suite("RunSession control acknowledgements", .serialized)
@MainActor
struct RunControlAckTests {

    private func makeSession() -> RunSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [RunControlStub.self]
        let client = HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:8897")!,
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

    /// Starts a run and waits for `currentRunID` to be set, with its events
    /// streams left open (never a terminal event) so the run stays "current"
    /// for the rest of the test -- cancel/approve/deny all guard on
    /// `currentRunID`. `extra` answers every other path the test needs
    /// (cancel/approve/deny).
    private func startBusyRun(
        _ session: RunSession, extra: @escaping @Sendable (URLRequest) -> RunControlStub.Response
    ) async throws {
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_1/events"), ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            default:
                return extra(request)
            }
        }
        session.draft = "hi"
        session.submit()
        try await wait { session.currentRunID == "run_1" }
    }

    @Test("a failed approve surfaces via connectionError -- core regression")
    func approveFailureSurfaces() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/approve" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(
                    #"{"error":{"code":"internal_error","message":"approve rejected"}}"#.utf8))
        }

        session.approve()
        try await wait { session.connectionError != nil }
        #expect(session.connectionError == "approve rejected")

        session.reset()
    }

    @Test("a failed deny surfaces via connectionError")
    func denyFailureSurfaces() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/deny" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"code":"internal_error","message":"deny rejected"}}"#.utf8))
        }

        session.deny()
        try await wait { session.connectionError != nil }
        #expect(session.connectionError == "deny rejected")

        session.reset()
    }

    @Test("answerInput succeeding clears pendingQuestions")
    func answerSuccessClearsPendingQuestions() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let promptJSON =
            #"{"run_id":"run_1","call_id":"call_1","questions":[{"question":"Continue?"}]}"#
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.waiting_for_user
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.waiting_for_user","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(frame.utf8), neverFinishes: true)
            case ("GET", "/v1/runs/run_1/input"):
                return .init(status: 200, body: Data(promptJSON.utf8))
            case ("POST", "/v1/runs/run_1/input"):
                return .init(status: 200)
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions != nil }
        let questionID = try #require(session.pendingQuestions?.questions.first?.id)

        session.answer([questionID: "yes"])
        try await wait { session.pendingQuestions == nil }
        #expect(session.connectionError == nil)

        session.reset()
    }

    @Test("a rejected answerInput keeps pendingQuestions and surfaces the error -- core regression")
    func answerFailureKeepsPendingQuestions() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let promptJSON =
            #"{"run_id":"run_1","call_id":"call_1","questions":[{"question":"Continue?"}]}"#
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.waiting_for_user
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.waiting_for_user","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(frame.utf8), neverFinishes: true)
            case ("GET", "/v1/runs/run_1/input"):
                return .init(status: 200, body: Data(promptJSON.utf8))
            case ("POST", "/v1/runs/run_1/input"):
                return .init(
                    status: 409,
                    body: Data(
                        #"{"error":{"code":"no_pending_input","message":"already answered"}}"#.utf8)
                )
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions != nil }
        let questionID = try #require(session.pendingQuestions?.questions.first?.id)

        session.answer([questionID: "yes"])
        try await wait { session.connectionError != nil }
        #expect(
            session.pendingQuestions != nil, "a rejected answer must not clear the pending question"
        )

        session.reset()
    }

    @Test(
        "a failed cancel surfaces via connectionError and a second press stays cooperative -- core regression"
    )
    func cancelFailureStaysCooperative() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"code":"internal_error","message":"cancel rejected"}}"#.utf8)
            )
        }

        session.cancel()
        try await wait { session.connectionError != nil }
        #expect(session.transcript.runState != .cancelled)

        session.cancel()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 2 }
        #expect(
            session.transcript.runState != .cancelled,
            "a failed first cancel must not let the second press force-abandon the stream locally"
        )

        session.reset()
    }

    @Test("cancel succeeding leaves no connectionError, and a second press marks cancelled")
    func cancelSuccessThenSecondPressCancels() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            return .init(status: 200)
        }

        session.cancel()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1 }
        try await Task.sleep(for: .milliseconds(50))
        #expect(session.connectionError == nil)

        session.cancel()
        try await wait { session.transcript.runState == .cancelled }
        #expect(
            RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1,
            "the second press must abandon locally, not call cancel again")

        session.reset()
    }

    /// Exercises the fix for #995 (F2): `cancelRequested` used to flip to
    /// `true` *synchronously*, before the first cooperative cancel's request
    /// had even reached the server -- so a second press arriving during that
    /// round trip force-abandoned the stream locally, ahead of any server
    /// acknowledgement. The replacement state machine must not escalate
    /// until the first cancel actually comes back.
    @Test(
        "a second press while the first cooperative cancel is still in flight does not escalate -- core regression"
    )
    func secondPressDuringInFlightCancelDoesNotEscalate() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let cancelArrived = Flag()
        let releaseCancel = DispatchSemaphore(value: 0)
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            cancelArrived.set()
            releaseCancel.wait()
            return .init(status: 200)
        }

        session.cancel()
        // `RunControlStub` serializes every request through one global lock
        // held for the duration of the handler call -- polling
        // `.requests(matching:)` here would try to take that same lock while
        // the handler above is still blocked holding it, deadlocking the
        // test. `cancelArrived` observes "the request reached the stub"
        // without touching the lock. A *blocking* wait here (rather than
        // this cooperative poll) would be just as wrong: it would stall the
        // MainActor executor that `session.cancel()`'s own `Task` needs in
        // order to run far enough to send the request in the first place.
        try await wait { cancelArrived.isSet }

        // The first cancel is now blocked in flight. A second press here
        // must not force-abandon the stream.
        session.cancel()
        try await Task.sleep(for: .milliseconds(80))
        #expect(
            session.transcript.runState != .cancelled,
            "a press during the in-flight cooperative cancel must not force-stop before the server acknowledges it"
        )

        // Signalled generously: harmless if only one request is blocked
        // (the fixed/expected shape), but insurance against a leaked,
        // permanently blocked background thread if a revert lets a second
        // press fire its own request here too.
        for _ in 0..<5 { releaseCancel.signal() }
        try await Task.sleep(for: .milliseconds(150))
        #expect(
            RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1,
            "a second press while the first cancel is in flight must not send a duplicate request")

        // Now that the first cancel has been acknowledged, a further press
        // is free to escalate.
        session.cancel()
        try await wait { session.transcript.runState == .cancelled }

        session.reset()
    }

    /// Exercises the fix for #995 (F3): a fire-and-forget `cancel()` Task
    /// that completes *after* `reset()` has already moved this session onto
    /// a different (or no) run must not write `connectionError` for that
    /// stale context -- reset() already cleared it, and a late failure
    /// response arriving afterwards must not resurrect it.
    @Test(
        "a cancel Task that resolves after reset() must not surface a stale connectionError -- core regression"
    )
    func cancelTaskAfterResetDoesNotWriteStaleError() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let cancelArrived = Flag()
        let releaseCancel = DispatchSemaphore(value: 0)
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            cancelArrived.set()
            releaseCancel.wait()
            return .init(
                status: 500,
                body: Data(#"{"error":{"code":"internal_error","message":"cancel rejected"}}"#.utf8)
            )
        }

        session.cancel()
        try await wait { cancelArrived.isSet }

        session.reset()
        #expect(session.connectionError == nil)

        for _ in 0..<5 { releaseCancel.signal() }
        try await Task.sleep(for: .milliseconds(80))
        #expect(
            session.connectionError == nil,
            "a cancel response arriving after reset() must not resurrect state for an abandoned run")
    }

    /// Exercises the fix for #995 (F1a): a second `answer()` call while the
    /// first is still awaiting the server must not fire a second request --
    /// this is the model-level guard behind the composer's disabled Send
    /// button.
    @Test(
        "a second answer() call while the first is still in flight sends only one request -- core regression"
    )
    func answerGuardsAgainstDoubleSubmission() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let promptJSON =
            #"{"run_id":"run_1","call_id":"call_1","questions":[{"question":"Continue?"}]}"#
        let answerArrived = Flag()
        let releaseAnswer = DispatchSemaphore(value: 0)
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.waiting_for_user
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.waiting_for_user","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(frame.utf8), neverFinishes: true)
            case ("GET", "/v1/runs/run_1/input"):
                return .init(status: 200, body: Data(promptJSON.utf8))
            case ("POST", "/v1/runs/run_1/input"):
                answerArrived.set()
                releaseAnswer.wait()
                return .init(status: 200)
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions != nil }
        let questionID = try #require(session.pendingQuestions?.questions.first?.id)

        session.answer([questionID: "yes"])
        // `RunControlStub` serializes every request through one global lock
        // held for the handler's duration -- `answerArrived` observes "the
        // POST reached the stub" without contending for that lock, which the
        // blocked handler is still holding. Polling cooperatively (not a
        // blocking wait) matters here too: a blocking wait would stall the
        // MainActor executor that `answer()`'s own `Task` needs in order to
        // run far enough to send this very request.
        try await wait { answerArrived.isSet }

        // A second press before the first answer's request resolves must not
        // fire a second POST.
        session.answer([questionID: "yes"])
        try await Task.sleep(for: .milliseconds(80))

        // Signalled generously rather than once: pre-fix, a second press
        // fires a genuine second POST that queues (and then blocks) on this
        // same stub -- a single `.signal()` would release only one waiter
        // and leak the other blocked forever, stalling URLSession's shared
        // thread pool for every test that runs after this one. Extra
        // signals with no waiter left are a harmless no-op.
        for _ in 0..<5 { releaseAnswer.signal() }
        try await wait { session.pendingQuestions == nil }
        // Let the stub's lock fully drain before counting requests -- a
        // stray second POST (pre-fix) may still be queued on it at the
        // moment `pendingQuestions` clears.
        try await Task.sleep(for: .milliseconds(150))

        #expect(
            RunControlStub.requests(matching: "/v1/runs/run_1/input").filter {
                $0.httpMethod == "POST"
            }.count == 1,
            "a second answer() call while the first is in flight must not send a duplicate request"
        )

        session.reset()
    }

    /// Exercises the fix for #995 (F1b): a success response for an answer
    /// must only clear `pendingQuestions` when it is still the very prompt
    /// that was answered -- a newer question assigned while the request was
    /// in flight (a second `run.waiting_for_user`) must survive.
    @Test(
        "a success answer only clears the prompt it actually answered, not a newer one assigned mid-flight -- core regression"
    )
    func answerSuccessOnlyClearsItsOwnPrompt() async throws {
        RunControlStub.reset()
        let promptJSON1 =
            #"{"run_id":"run_1","call_id":"call_1","questions":[{"question":"First?"}]}"#
        let promptJSON2 =
            #"{"run_id":"run_1","call_id":"call_2","questions":[{"question":"Second?"}]}"#
        let session = makeSession()
        let releaseSecondFetch = DispatchSemaphore(value: 0)
        let releaseAnswer = DispatchSemaphore(value: 0)
        let inputGetCount = Locked(0)
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.waiting_for_user
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.waiting_for_user","payload":{}}

                    id: run_1:1
                    event: run.waiting_for_user
                    data: {"id":"run_1:1","run_id":"run_1","type":"run.waiting_for_user","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(frame.utf8), neverFinishes: true)
            case ("GET", "/v1/runs/run_1/input"):
                let callNumber = inputGetCount.increment()
                if callNumber == 1 {
                    return .init(status: 200, body: Data(promptJSON1.utf8))
                }
                releaseSecondFetch.wait()
                return .init(status: 200, body: Data(promptJSON2.utf8))
            case ("POST", "/v1/runs/run_1/input"):
                releaseAnswer.wait()
                return .init(status: 200)
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions?.callID == "call_1" }
        let firstQuestionID = try #require(session.pendingQuestions?.questions.first?.id)

        // Answer the first prompt. `RunControlStub` serializes every request
        // through one global lock held for the handler's duration, so this
        // POST cannot even reach its own gate yet -- it queues behind the
        // still-blocked second `pendingInput` fetch above.
        session.answer([firstQuestionID: "yes"])
        try await Task.sleep(for: .milliseconds(80))

        // Release the second fetch: it completes, assigning a newer prompt
        // (call_2) while the first answer's request is still queued.
        for _ in 0..<5 { releaseSecondFetch.signal() }
        try await wait { session.pendingQuestions?.callID == "call_2" }

        // The queued POST can now reach its own gate; release it too, then
        // let its success response finish processing.
        for _ in 0..<5 { releaseAnswer.signal() }
        try await Task.sleep(for: .milliseconds(150))

        #expect(
            session.pendingQuestions?.callID == "call_2",
            "an older answer's success must not clear a newer prompt assigned while it was in flight")
        #expect(session.connectionError == nil)

        session.reset()
    }
}

/// A plain thread-safe boolean, set by a stub handler running on a
/// background (non-Swift-concurrency) thread and observed by `wait { }`'s
/// cooperative polling loop. A blocking `DispatchSemaphore.wait()` on the
/// test side would stall the MainActor executor that the `Task` under test
/// needs in order to run far enough to send the very request this flag
/// waits for -- polling instead (via `Task.sleep` between checks) leaves
/// that executor free to make progress.
private final class Flag: @unchecked Sendable {
    private let lock = NSLock()
    private var value = false

    func set() { lock.withLock { value = true } }
    var isSet: Bool { lock.withLock { value } }
}

/// A tiny lock-guarded counter -- `RunControlStub`'s handler runs on a
/// background queue owned by the URL loading system, so a plain `var`
/// captured by its `@Sendable` closure is not safe to mutate directly.
private final class Locked: @unchecked Sendable {
    private let lock = NSLock()
    private var value: Int

    init(_ value: Int) { self.value = value }

    func increment() -> Int {
        lock.withLock {
            value += 1
            return value
        }
    }
}
