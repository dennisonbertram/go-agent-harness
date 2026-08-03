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
        RunSession(client: client())
    }

    private func session(
        submissionTimeoutNow: @escaping @MainActor () -> ContinuousClock.Instant
    ) -> RunSession {
        RunSession(client: client(), submissionTimeoutNow: submissionTimeoutNow)
    }

    private func client() -> HarnessClient {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [PassiveOutcomeProtocol.self]
        return HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:8897")!,
            session: URLSession(configuration: configuration)
        )
    }

    private func project() -> ProjectSession {
        ProjectSession(workspace: URL(fileURLWithPath: "/tmp"), client: client())
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

    private nonisolated static func startedStream(for runID: String, timestamp: String) -> Data {
        let id = "\(runID):started"
        return Data(
            "id: \(id)\nevent: run.started\ndata: {\"id\":\"\(id)\",\"run_id\":\"\(runID)\",\"type\":\"run.started\",\"timestamp\":\"\(timestamp)\",\"payload\":{}}\n\n"
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

    /// The only way test code receives timeout authority mirrors production:
    /// Runner mints the opaque ticket at the actual wait deadline. Holding a
    /// submission before this helper returns never exposes a cancel API.
    private func waitForTimeoutTicket(
        _ run: RunSession, submission: RunSubmission
    ) async -> (Runner.SubmissionWaitOutcome, TimedOutSubmissionTicket?) {
        var ticket: TimedOutSubmissionTicket?
        let outcome = await Runner.waitForTerminal(
            run: run, submission: submission,
            pollInterval: .milliseconds(5)
        ) { ticket = $0 }
        return (outcome, ticket)
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
        let now = ContinuousClock.now
        let run = session(submissionTimeoutNow: { now })
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(100)))
        try await displaceA(run, submission: submission)
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/events") }
        let waitTask = Task {
            await Runner.waitForTerminal(
                run: run, submission: submission,
                pollInterval: .milliseconds(5))
        }
        PassiveOutcomeProtocol.openGate("a-terminal")
        try await wait { submission.isTerminal }
        #expect(await waitTask.value == .terminal)
        #expect(run.currentRunID == "run_b")
        assertNoAction(for: ["run_a", "run_b", "run_c"])
        run.reset()
    }

    @Test("Runner.walk judges displaced A terminal without B control")
    func walkObservesTerminalAAfterBSelection() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                .init(
                    headers: ["Content-Type": "text/event-stream"],
                    body: Self.terminalStream(for: "run_a"), waitForGate: "walk-terminal"
                )
            case ("GET", "/v1/conversations/run_a/events"):
                .init(
                    headers: ["Content-Type": "text/event-stream"],
                    body: Self.startedStream(for: "run_a", timestamp: "2026-08-03T22:00:00Z")
                        + Self.startedStream(for: "run_b", timestamp: "2026-08-03T22:00:01Z"),
                    neverFinishes: true
                )
            case ("GET", "/v1/conversations"):
                .init(body: Data(#"{"conversations":[]}"#.utf8))
            default: .init()
            }
        }
        let project = project()
        let task = Task {
            await Runner.walk(
                project: project, specs: [.init(name: "x", prompt: "A")],
                config: .init(timeoutPerTool: .seconds(1), pollInterval: .milliseconds(5))
            )
        }
        try await wait { project.run?.currentRunID == "run_b" }
        PassiveOutcomeProtocol.openGate("walk-terminal")
        let results = await task.value
        #expect(results.count == 1)
        #expect(results[0].verdict == "fail")
        #expect(results[0].reply == "the tool 'x' was never invoked")
        assertNoAction(for: ["run_b"])
    }

    @Test("Runner.walk timeout cancels only A after B then C selection")
    func walkTimeoutCancelsOnlyAAfterBThenC() async throws {
        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            case ("GET", "/v1/conversations/run_a/events"):
                .init(
                    headers: ["Content-Type": "text/event-stream"],
                    body: Self.startedStream(for: "run_a", timestamp: "2026-08-03T22:00:00Z")
                        + Self.startedStream(for: "run_b", timestamp: "2026-08-03T22:00:01Z")
                        + Self.startedStream(for: "run_c", timestamp: "2026-08-03T22:00:02Z"),
                    neverFinishes: true
                )
            case ("POST", "/v1/runs/run_a/cancel"):
                .init(status: 204)
            case ("GET", "/v1/conversations"):
                .init(body: Data(#"{"conversations":[]}"#.utf8))
            default: .init()
            }
        }
        let project = project()
        let task = Task {
            await Runner.walk(
                project: project, specs: [.init(name: "x", prompt: "A")],
                config: .init(timeoutPerTool: .milliseconds(80), pollInterval: .milliseconds(5))
            )
        }
        try await wait { project.run?.currentRunID == "run_c" }
        let results = await task.value
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel") }
        #expect(results[0].reply.contains("timed out"))
        #expect(project.run?.currentRunID == "run_c")
        #expect(PassiveOutcomeProtocol.paths().filter { $0 == "/v1/runs/run_a/cancel" }.count == 1)
        assertNoAction(for: ["run_b", "run_c"])
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
        let now = ContinuousClock.now
        let run = session(submissionTimeoutNow: { now })
        run.load(messages: [], conversationID: "conversation")
        run.draft = "A"
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await displaceA(run, submission: submission)
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/events") }
        let waitTask = Task {
            await Runner.waitForTerminal(
                run: run, submission: submission,
                pollInterval: .milliseconds(5))
        }
        PassiveOutcomeProtocol.openGate("a-eof")
        try await wait {
            if case .failed = submission.lifecycle { return true }
            return false
        }
        #expect(await waitTask.value == .failed("run event stream ended before a terminal event"))
        #expect(run.currentRunID == "run_b")
        #expect(run.transcript.runState != .failed)
        assertNoAction(for: ["run_a", "run_b", "run_c"])
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
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await displaceA(run, submission: submission)
        let (outcome, ticket) = await waitForTimeoutTicket(run, submission: submission)
        #expect(outcome == .timedOut)
        #expect(ticket?.consume() == true)
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
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(80)))
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
        // Only the deadline wait may mint A's authority. C cannot replace
        // that ticket even though it now owns selected shared state.
        let (outcome, ticket) = await waitForTimeoutTicket(run, submission: submission)
        #expect(outcome == .timedOut)
        #expect(ticket?.consume() == true)
        #expect(ticket?.consume() == false)
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
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await wait { submission.runID == "run_a" }
        let (outcome, ticket) = await waitForTimeoutTicket(run, submission: submission)
        #expect(outcome == .timedOut)
        #expect(ticket?.consume() == true)
        #expect(ticket?.consume() == false)
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
        let terminal = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await wait { terminal.runID == "run_a" }
        PassiveOutcomeProtocol.openGate("a-terminal")
        try await wait { terminal.isTerminal }
        // A terminal run can never mint a deadline ticket.
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
        // Reset detaches the stream before another deadline can mint a ticket.
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
        // A failed run can never mint a deadline ticket.
        #expect(!PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel"))
    }

    @Test("deadline ticket is absent before expiry and reset revokes it after expiry")
    func ticketCannotExistBeforeDeadlineAndResetRevokesIt() async throws {
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
        var now = ContinuousClock.now
        let run = session(submissionTimeoutNow: { now })
        run.draft = "A"
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(100)))
        try await wait { submission.runID == "run_a" }

        let gateA = try #require(run.submissionTimeoutGate(for: submission))
        let gateB = try #require(run.submissionTimeoutGate(for: submission))
        #expect(gateA === gateB)
        #expect(gateA.ticketIfExpired() == nil)
        now = now.advanced(by: .milliseconds(99))
        #expect(gateB.ticketIfExpired() == nil)
        assertNoAction(for: ["run_a", "run_b", "run_c"])

        now = now.advanced(by: .milliseconds(1))
        let minted = try #require(gateA.ticketIfExpired())
        #expect(gateB.ticketIfExpired() == nil)
        #expect(minted.consume())
        #expect(!minted.consume())
        try await wait { PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel") }
        #expect(PassiveOutcomeProtocol.paths().filter { $0 == "/v1/runs/run_a/cancel" }.count == 1)
        assertNoAction(for: ["run_b", "run_c"])
        run.reset()
        try await wait { PassiveOutcomeProtocol.stopped("/v1/runs/run_a/events") }
        #expect(!minted.consume())
    }

    @Test("GUI submissions have no timeout gate and delayed start binds its own deadline")
    func guiSubmissionHasNoTimeoutAndDeadlineStartsAtAcknowledgement() async throws {
        var now = ContinuousClock.now
        let delayed = RunSubmission(
            prompt: "A", timeoutOwner: UUID(), timeoutGeneration: 0,
            timeoutAfter: .milliseconds(60), timeoutNow: { now }
        )
        #expect(delayed.timeoutDeadlineIfStarted() == nil)
        now = now.advanced(by: .seconds(1))
        delayed.markStarted(runID: "run_delayed")
        let deadline = try #require(delayed.timeoutDeadlineIfStarted())
        #expect(now < deadline)

        PassiveOutcomeProtocol.reset()
        PassiveOutcomeProtocol.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                .init(status: 202, body: Data(#"{"run_id":"run_gui","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_gui/events"):
                .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            default: .init()
            }
        }
        let run = session()
        run.draft = "GUI"
        let submission = try #require(run.submit())
        try await wait { submission.runID == "run_gui" }
        #expect(run.submissionTimeoutGate(for: submission) == nil)
        assertNoAction(for: ["run_gui"])
        run.reset()
    }

    @Test("terminal and failure after deadline revoke an already-minted ticket")
    func terminalAndFailureAfterTicketRevokeTransport() async throws {
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
        let terminal = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await wait { terminal.runID == "run_a" }
        let (outcome, ticket) = await waitForTimeoutTicket(run, submission: terminal)
        #expect(outcome == .timedOut)
        try terminal.apply(
            event(
                "run_a:terminal", "run_a", "run.completed",
                timestamp: "2026-08-03T22:00:04Z"
            )
        )
        let minted = try #require(ticket)
        #expect(!minted.consume())

        // Failure uses the same RunSession-owned submission path; it must
        // revoke a ticket minted just before its stream reports EOF/failure.
        run.reset()
        run.draft = "failed A"
        let failed = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await wait { failed.runID == "run_a" }
        let (failureOutcome, failureTicket) = await waitForTimeoutTicket(run, submission: failed)
        #expect(failureOutcome == .timedOut)
        failed.markFailed("stream ended")
        let mintedFailure = try #require(failureTicket)
        #expect(!mintedFailure.consume())
        #expect(!PassiveOutcomeProtocol.paths().contains("/v1/runs/run_a/cancel"))
        run.reset()
    }

    @Test("loading another conversation revokes an already-minted A ticket")
    func loadRevokesMintedTicketWithoutActingOnReplacement() async throws {
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
        let submission = try #require(run.submit(timeoutAfter: .milliseconds(80)))
        try await wait { submission.runID == "run_a" }
        let (outcome, ticket) = await waitForTimeoutTicket(run, submission: submission)
        #expect(outcome == .timedOut)
        let minted = try #require(ticket)

        run.load(messages: [], conversationID: "replacement")
        try await wait { PassiveOutcomeProtocol.stopped("/v1/runs/run_a/events") }
        #expect(!minted.consume())
        #expect(run.currentRunID == nil)
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
