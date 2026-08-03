import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI
@testable import ToolWalk

/// Exercises `RunSession.submit()` and ToolWalk's wait policy through a real
/// URLSession transport. Gates establish the ownership boundary (B selected)
/// before the A response/stream is allowed to advance; sleeps would make these
/// exact ordering proofs flaky and could accidentally test the opposite order.
private final class PassiveOutcomeProtocol: URLProtocol, @unchecked Sendable {
    struct Response {
        var status = 200
        var headers = ["Content-Type": "application/json"]
        var body = Data()
        var neverFinishes = false
        var waitForGate: String?
    }

    private nonisolated(unsafe) static var handler: (@Sendable (URLRequest) -> Response)?
    private nonisolated(unsafe) static var requests: [URLRequest] = []
    private nonisolated(unsafe) static var stoppedPaths: Set<String> = []
    private nonisolated(unsafe) static var startRunResponses = 0
    private static let lock = NSLock()
    private static let gateLock = NSCondition()
    private nonisolated(unsafe) static var openGates: Set<String> = []

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    static func reset() {
        lock.withLock {
            handler = nil
            requests = []
            stoppedPaths = []
            startRunResponses = 0
        }
        gateLock.lock()
        openGates = []
        gateLock.unlock()
    }

    static func paths() -> [String] {
        lock.withLock { requests.compactMap(\.url?.path) }
    }

    static func stopped(_ path: String) -> Bool {
        lock.withLock { stoppedPaths.contains(path) }
    }

    /// URLProtocol does not guarantee `httpBody` remains materialized when the
    /// request is recreated by URLSession. The integration fixture uses a
    /// request-order counter rather than relying on that transport detail.
    static func nextStartRunID() -> String {
        lock.withLock {
            startRunResponses += 1
            return startRunResponses == 1 ? "run_a" : "run_c"
        }
    }

    static func openGate(_ gate: String) {
        gateLock.lock()
        openGates.insert(gate)
        gateLock.broadcast()
        gateLock.unlock()
    }

    override class func canInit(with _: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        let request = request
        let handler = Self.lock.withLock {
            Self.requests.append(request)
            return Self.handler
        }
        // The handler may advance a fixture counter protected by the same
        // lock. Invoke it after recording the request, not recursively inside
        // the lock, so a multi-submit ordering test cannot deadlock itself.
        let response = handler?(request) ?? Response()
        DispatchQueue.global().async {
            if let gate = response.waitForGate { Self.waitForGate(gate) }
            let http = HTTPURLResponse(
                url: request.url!, statusCode: response.status, httpVersion: "HTTP/1.1",
                headerFields: response.headers
            )!
            self.client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
            self.client?.urlProtocol(self, didLoad: response.body)
            if !response.neverFinishes { self.client?.urlProtocolDidFinishLoading(self) }
        }
    }

    override func stopLoading() {
        _ = Self.lock.withLock { Self.stoppedPaths.insert(request.url?.path ?? "") }
    }

    private static func waitForGate(_ gate: String) {
        gateLock.lock()
        defer { gateLock.unlock() }
        let deadline = Date().addingTimeInterval(5)
        while !openGates.contains(gate), gateLock.wait(until: deadline) {}
    }
}

@Suite("ToolWalk displaced submission outcomes", .serialized)
@MainActor
struct PassiveSubmissionOutcomeIntegrationTests {
    private func session() -> RunSession {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [PassiveOutcomeProtocol.self]
        return RunSession(
            client: HarnessClient(
                baseURL: URL(string: "http://127.0.0.1:8897")!,
                session: URLSession(configuration: configuration)
            ))
    }

    private func event(_ id: String, _ runID: String, _ type: String, timestamp: String) throws
        -> HarnessEvent
    {
        try HarnessEvent(
            frame: SSEFrame(
                id: id, event: type,
                data:
                    #"{"id":"\#(id)","run_id":"\#(runID)","type":"\#(type)","timestamp":"\#(timestamp)","payload":{}}"#
            ))
    }

    private nonisolated static func terminalStream(for runID: String) -> Data {
        let id = "\(runID):terminal"
        return Data(
            "id: \(id)\nevent: run.completed\ndata: {\"id\":\"\(id)\",\"run_id\":\"\(runID)\",\"type\":\"run.completed\",\"payload\":{}}\n\n"
                .utf8)
    }

    private func wait(timeout: Duration = .seconds(2), _ condition: () -> Bool) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("timed out waiting for gated integration condition")
    }

    private func displaceA(_ session: RunSession, submission: RunSubmission) async throws {
        try await wait { submission.runID == "run_a" && session.currentRunID == "run_a" }
        try await session.applyConversationEvent(
            event("run_a:started", "run_a", "run.started", timestamp: "2026-08-03T22:00:00Z"),
            conversationID: "conversation"
        )
        try await session.applyConversationEvent(
            event("run_b:started", "run_b", "run.started", timestamp: "2026-08-03T22:00:01Z"),
            conversationID: "conversation"
        )
        #expect(session.currentRunID == "run_b")
        #expect(submission.isDisplaced)
    }

    private func assertNoAction(for runIDs: [String]) {
        let actions = Set(
            runIDs.flatMap { runID in
                ["cancel", "approve", "deny", "input", "steer"].map { "/v1/runs/\(runID)/\($0)" }
            }
        ).intersection(PassiveOutcomeProtocol.paths())
        #expect(actions.isEmpty)
    }

    @Test("B before A terminal is judged as A terminal without a B action")
    func terminalAfterDisplacementRemainsObservable() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                .init(
                    headers: ["Content-Type": "text/event-stream"],
                    body: Self.terminalStream(for: "run_a"), waitForGate: "a-terminal")
            case ("GET", "/v1/conversations/conversation/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            default: .init()
            }
        }
        let run = session()
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit())
        try await displaceA(run, submission: submission)
        let waitTask = Task {
            await Runner.waitForTerminal(
                run: run, submission: submission,
                config: .init(timeoutPerTool: .seconds(1), pollInterval: .milliseconds(5)))
        }
        PassiveOutcomeProtocol.openGate("a-terminal")
        #expect(await waitTask.value == .terminal)
        #expect(run.currentRunID == "run_b")
        assertNoAction(for: ["run_b"])
        run.reset()
    }

    @Test("B before A EOF is judged as A failure without failing B")
    func eofAfterDisplacementRemainsObservable() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                .init(headers: ["Content-Type": "text/event-stream"], waitForGate: "a-eof")
            case ("GET", "/v1/conversations/conversation/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            default: .init()
            }
        }
        let run = session()
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit())
        try await displaceA(run, submission: submission)
        let waitTask = Task {
            await Runner.waitForTerminal(
                run: run, submission: submission,
                config: .init(timeoutPerTool: .seconds(1), pollInterval: .milliseconds(5)))
        }
        PassiveOutcomeProtocol.openGate("a-eof")
        #expect(await waitTask.value == .failed("run event stream ended before a terminal event"))
        #expect(run.currentRunID == "run_b")
        #expect(run.transcript.runState != .failed)
        assertNoAction(for: ["run_b"])
        run.reset()
    }

    @Test("B before A timeout cancels exactly A without changing B")
    func timeoutAfterDisplacementCancelsOnlyA() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"), ("GET", "/v1/conversations/conversation/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            case ("POST", "/v1/runs/run_a/cancel"):
                .init(status: 204)
            default: .init()
            }
        }
        let run = session()
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit())
        try await displaceA(run, submission: submission)
        let outcome = await Runner.waitForTerminal(
            run: run, submission: submission,
            config: .init(timeoutPerTool: .milliseconds(80), pollInterval: .milliseconds(5))
        )
        #expect(outcome == .timedOut)
        #expect(run.cancelTimedOutSubmission(submission))
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel") }
        #expect(run.currentRunID == "run_b")
        assertNoAction(for: ["run_b"])
        run.reset()
    }

    @Test("a later local C cannot revoke timed-out displaced A cancellation")
    func timeoutRetainsAOwnershipAfterBThenC() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                let runID = PassiveOutcomeProtocol.nextStartRunID()
                return .init(
                    status: 202,
                    body: Data(#"{"run_id":"\#(runID)","status":"queued"}"#.utf8)
                )
            case ("GET", "/v1/runs/run_a/events"),
                 ("GET", "/v1/runs/run_c/events"),
                 ("GET", "/v1/conversations/conversation/events"):
                return .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            case ("POST", "/v1/runs/run_a/cancel"):
                return .init(status: 204)
            default:
                return .init()
            }
        }
        let run = session()
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit())
        try await displaceA(run, submission: submission)
        try await run.applyConversationEvent(
            event("run_b:completed", "run_b", "run.completed", timestamp: "2026-08-03T22:00:02Z"),
            conversationID: "conversation"
        )
        #expect(run.currentRunID == nil)

        // B completion makes the shared view available, but A's per-run
        // stream is deliberately still active. C must not replace A's timeout
        // authority just because session-level mutable pointers now name C.
        run.draft = "C"
        let c = try #require(run.submit())
        try await wait { c.runID == "run_c" && run.currentRunID == "run_c" }
        // Timeout policy is already exercised above. This direct dispatch
        // proves the more important authority condition deterministically:
        // once B is terminal and C owns visible state, A's captured handle
        // still authorizes exactly one A-only cancellation.
        #expect(run.cancelTimedOutSubmission(submission))
        #expect(!run.cancelTimedOutSubmission(submission))
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel") }
        #expect(PassiveOutcomeProtocol.paths().filter { $0 == "/v1/runs/run_a/cancel" }.count == 1)
        #expect(run.currentRunID == "run_c")
        assertNoAction(for: ["run_b", "run_c"])
        run.reset()
        try await wait {
            PassiveOutcomeProtocol.stopped("/v1/runs/run_a/events")
                && PassiveOutcomeProtocol.stopped("/v1/runs/run_c/events")
        }
    }

    @Test("exact timeout capability sends A cancel once and never reuses it")
    func timeoutCapabilityIsSingleUse() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            case ("POST", "/v1/runs/run_a/cancel"):
                .init(status: 204)
            default:
                .init()
            }
        }
        let run = session()
        run.draft = "A"
        let submission = try #require(run.submit())
        try await wait { submission.runID == "run_a" }
        #expect(run.cancelTimedOutSubmission(submission))
        #expect(!run.cancelTimedOutSubmission(submission))
        try await wait {
            PassiveOutcomeProtocol.paths().filter { $0 == "/v1/runs/run_a/cancel" }.count == 1
        }
        run.reset()
    }

    @Test("terminal or reset submission cannot retain timeout cancellation")
    func terminalAndResetRevokeTimeoutCapability() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                .init(
                    headers: ["Content-Type": "text/event-stream"],
                    body: Self.terminalStream(for: "run_a"),
                    waitForGate: "a-terminal"
                )
            case ("POST", "/v1/runs/run_a/cancel"):
                .init(status: 204)
            default:
                .init()
            }
        }
        let run = session()
        run.draft = "A"
        let terminal = try #require(run.submit())
        try await wait { terminal.runID == "run_a" }
        PassiveOutcomeProtocol.openGate("a-terminal")
        try await wait { terminal.isTerminal }
        #expect(!run.cancelTimedOutSubmission(terminal))
        #expect(!PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel"))

        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_b","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_b/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            case ("POST", "/v1/runs/run_b/cancel"):
                .init(status: 204)
            default:
                .init()
            }
        }
        run.draft = "B"
        let reset = try #require(run.submit())
        try await wait { reset.runID == "run_b" }
        run.reset()
        try await wait { PassiveOutcomeProtocol.stopped("/v1/runs/run_b/events") }
        #expect(!run.cancelTimedOutSubmission(reset))
        #expect(!PassiveOutcomeProtocol.paths().contains("/v1/runs/run_b/cancel"))
    }

    @Test("failed A submission revokes its timeout capability before dispatch")
    func failedSubmissionRevokesTimeoutCapability() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                // An empty completed stream is the real RunSession failure
                // path, not a manually forged lifecycle transition.
                .init(headers: ["Content-Type": "text/event-stream"])
            case ("POST", "/v1/runs/run_a/cancel"):
                .init(status: 204)
            default:
                .init()
            }
        }
        let run = session()
        run.draft = "A"
        let submission = try #require(run.submit())
        try await wait {
            if case .failed = submission.lifecycle { return true }
            return false
        }
        #expect(!run.cancelTimedOutSubmission(submission))
        #expect(!PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel"))
    }

    @Test("B before delayed A acknowledgement still waits for A identity")
    func delayedAcknowledgementPreservesDisplacementAndObservation() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(
                    status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8),
                    waitForGate: "a-ack")
            case ("GET", "/v1/runs/run_a/events"), ("GET", "/v1/conversations/conversation/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            default: .init()
            }
        }
        let run = session()
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit())
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs") }
        try await run.applyConversationEvent(
            event("run_b:started", "run_b", "run.started", timestamp: "2026-08-03T22:00:01Z"),
            conversationID: "conversation"
        )
        #expect(run.currentRunID == "run_b")
        PassiveOutcomeProtocol.openGate("a-ack")
        let started = await Runner.waitForStartedSubmission(
            submission, config: .init(timeoutPerTool: .seconds(1), pollInterval: .milliseconds(5))
        )
        #expect(started == .started)
        #expect(submission.runID == "run_a")
        #expect(submission.isDisplaced)
        #expect(run.currentRunID == "run_b")
        assertNoAction(for: ["run_b"])
        run.reset()
    }
}
