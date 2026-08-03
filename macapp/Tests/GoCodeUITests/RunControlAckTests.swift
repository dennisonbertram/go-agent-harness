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
        /// Delays delivery of the HTTP acknowledgement to model a pending
        /// control request without relying on a completed SSE stream.
        var responseDelay: TimeInterval = 0
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

    override static func canInit(with request: URLRequest) -> Bool { true }
    override static func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response = Self.lock.withLock {
            Self.recorded.append(request)
            return Self.handler?(request) ?? Response()
        }
        if response.responseDelay > 0 {
            DispatchQueue.global().asyncAfter(deadline: .now() + response.responseDelay) {
                self.deliver(response, for: request)
            }
            return
        }
        deliver(response, for: request)
    }

    private func deliver(_ response: Response, for request: URLRequest) {
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

private final class RequestCounter: @unchecked Sendable {
    private let lock = NSLock()
    private var value = 0

    func next() -> Int {
        lock.withLock {
            value += 1
            return value
        }
    }
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

    /// Starts a run whose own SSE stream completes shortly after its control
    /// POST begins. This is intentionally a completed stream, unlike
    /// `startBusyRun`: it proves a delayed acknowledgement still releases the
    /// composer after the stream task has cleared `currentRunID`.
    private func startRunThatTerminatesDuringControl(
        _ session: RunSession, control: @escaping @Sendable (URLRequest) -> RunControlStub.Response
    ) async throws {
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_1/events"):
                let terminal = """
                    id: run_1:done
                    event: run.completed
                    data: {"id":"run_1:done","run_id":"run_1","type":"run.completed","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(terminal.utf8), responseDelay: 0.2)
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            default:
                return control(request)
            }
        }
        session.draft = "start"
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

    @Test("a rejected answer keeps its prompt, then retries once without retaining the old error")
    func answerFailureKeepsPendingQuestions() async throws {
        RunControlStub.reset()
        let counter = RequestCounter()
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
                return counter.next() == 1
                    ? .init(
                        status: 409,
                        body: Data(
                            #"{"error":{"code":"no_pending_input","message":"already answered"}}"#
                                .utf8))
                    : .init(status: 200, responseDelay: 1)
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions != nil }
        let questionID = try #require(session.pendingQuestions?.questions.first?.id)

        session.answer([questionID: "yes"])
        try await wait { session.connectionError == "already answered" }
        #expect(
            session.pendingQuestions != nil, "a rejected answer must not clear the pending question"
        )
        session.answer([questionID: "yes"])
        session.answer([questionID: "yes"])
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/input").count == 3 }
        #expect(session.answerInFlight)
        #expect(session.connectionError == nil)
        try await wait { session.pendingQuestions == nil }

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

    @Test("a pending cancel exposes in-flight state and suppresses duplicate requests")
    func pendingCancelSuppressesDuplicates() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            return .init(status: 200, responseDelay: 1)
        }

        session.cancel()
        session.cancel()
        try await wait {
            session.cancelInFlight
                && RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1
        }
        #expect(session.cancelInFlight)
        #expect(RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1)

        session.reset()
    }

    @Test("a second steer while the first acknowledgement is pending preserves its draft")
    func pendingSteerPreservesLaterDraft() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/steer" else {
                return .init()
            }
            return .init(status: 200, responseDelay: 1)
        }

        session.draft = "first steer"
        session.steer()
        try await wait { session.runControlInFlight }
        session.draft = "second steer"
        session.steer()

        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/steer").count == 1 }
        #expect(session.draft == "second steer")
        #expect(RunControlStub.requests(matching: "/v1/runs/run_1/steer").count == 1)
        session.reset()
    }

    @Test("a failed approve retry clears its surfaced error before acknowledgement")
    func approveRetryClearsFailure() async throws {
        RunControlStub.reset()
        let counter = RequestCounter()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/approve" else {
                return .init()
            }
            return counter.next() == 1
                ? .init(status: 500, body: Data(#"{"error":{"message":"approve rejected"}}"#.utf8))
                : .init(status: 200, responseDelay: 1)
        }

        session.approve()
        try await wait { session.connectionError == "approve rejected" }
        session.approve()
        try await wait {
            RunControlStub.requests(matching: "/v1/runs/run_1/approve").count == 2
        }
        #expect(session.connectionError == nil)
        session.reset()
    }

    @Test("an acknowledged approval stays disabled until its matching SSE lifecycle advances")
    func approvalAcknowledgementWaitsForLifecycle() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/approve" else {
                return .init()
            }
            return .init(status: 200)
        }

        session.approve()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/approve").count == 1 }
        try await wait { session.acknowledgedRunControlRunID == "run_1" }
        #expect(session.runControlInFlight)
        let stale = try HarnessEvent(
            frame: SSEFrame(
                id: "run_a:done", event: "run.completed",
                data: #"{"id":"run_a:done","run_id":"run_a","type":"run.completed","payload":{}}"#))
        _ = await session.apply(stale, runID: "run_a")
        #expect(
            session.runControlInFlight,
            "a stale-run completion must not release B's acknowledgement")
        let granted = try HarnessEvent(
            frame: SSEFrame(
                id: "run_1:grant", event: "tool.approval_granted",
                data:
                    #"{"id":"run_1:grant","run_id":"run_1","type":"tool.approval_granted","payload":{}}"#
            ))
        _ = await session.apply(granted, runID: "run_1")
        try await wait { !session.runControlInFlight }
        session.reset()
    }

    @Test("matching approval SSE before a delayed HTTP acknowledgement stays guarded then releases")
    func approvalLifecycleBeforeAcknowledgement() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/approve" else {
                return .init()
            }
            return .init(status: 200, responseDelay: 1)
        }

        session.approve()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/approve").count == 1 }
        let granted = try HarnessEvent(
            frame: SSEFrame(
                id: "run_1:early-grant", event: "tool.approval_granted",
                data:
                    #"{"id":"run_1:early-grant","run_id":"run_1","type":"tool.approval_granted","payload":{}}"#
            ))
        _ = await session.apply(granted, runID: "run_1")
        #expect(
            session.runControlInFlight, "the request remains guarded until its HTTP ACK arrives")
        try await wait { !session.runControlInFlight }
        #expect(session.connectionError == nil)
        session.reset()
    }

    @Test("a terminal SSE before a delayed approve acknowledgement releases the composer")
    func terminalBeforeApproveAcknowledgementReleasesComposer() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startRunThatTerminatesDuringControl(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/approve" else {
                return .init()
            }
            return .init(status: 200, responseDelay: 1)
        }

        session.approve()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/approve").count == 1 }
        try await wait { session.currentRunID == nil && !session.isBusy }
        #expect(session.runControlInFlight, "the delayed acknowledgement still owns the control")

        try await wait { !session.runControlInFlight }
        #expect(session.acknowledgedRunControlRunID == nil)
        session.draft = "next prompt"
        #expect(session.canSubmit, "a terminal run must not leave the composer disabled")
        session.reset()
    }

    @Test(
        "a terminal SSE before a delayed steer failure restores the draft and releases the composer"
    )
    func terminalBeforeSteerFailureRestoresDraftAndReleasesComposer() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startRunThatTerminatesDuringControl(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/steer" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"message":"steer rejected"}}"#.utf8),
                responseDelay: 1)
        }

        session.draft = "redirect this"
        session.steer()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/steer").count == 1 }
        try await wait { session.currentRunID == nil && !session.isBusy }
        try await wait { !session.runControlInFlight }
        #expect(session.connectionError == "steer rejected")
        #expect(session.draft == "redirect this")
        #expect(session.canSubmit, "the restored draft remains usable after terminal completion")
        session.reset()
    }

    @Test("reset invalidates a delayed terminal-era control completion")
    func resetInvalidatesDelayedControlCompletion() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/steer" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"message":"old steer rejected"}}"#.utf8),
                responseDelay: 1)
        }

        session.draft = "old redirect"
        session.steer()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/steer").count == 1 }
        session.reset()
        try await Task.sleep(for: .milliseconds(1200))

        #expect(session.connectionError == nil)
        #expect(session.draft.isEmpty, "an old request must not restore text into a reset session")
        #expect(!session.runControlInFlight)
    }

    @Test("loading another conversation invalidates a delayed terminal-era control completion")
    func conversationSwitchInvalidatesDelayedControlCompletion() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/steer" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"message":"old steer rejected"}}"#.utf8),
                responseDelay: 1)
        }

        session.draft = "old redirect"
        session.steer()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/steer").count == 1 }
        session.load(messages: [], conversationID: "other_conversation")
        try await Task.sleep(for: .milliseconds(1200))

        #expect(session.conversationID == "other_conversation")
        #expect(session.connectionError == nil)
        #expect(
            session.draft.isEmpty, "an old request must not restore text into the new conversation")
        #expect(!session.runControlInFlight)
        session.reset()
    }

    @Test("a delayed successful control acknowledgement cannot erase a newer visible error")
    func delayedSuccessRetainsNewerError() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/steer" else {
                return .init()
            }
            return .init(status: 200, responseDelay: 1)
        }

        session.draft = "steer after this"
        session.steer()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/steer").count == 1 }
        session.connectionError = "newer connection error"
        try await wait { !session.runControlInFlight }
        #expect(session.connectionError == "newer connection error")
        session.reset()
    }
}
