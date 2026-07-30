import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP stub scoped to this file's tests. Mirrors `RunControlStub`
/// (`RunControlAckTests.swift`) rather than the path-only
/// `ActivityStubProtocol`/`LoadStateStubProtocol` stubs, because these tests
/// must assert *no* fork/undo request ever reached the server -- which
/// requires recording every request, not just answering the
/// last-registered handler. Scoped to its own fixed loopback port so it
/// never intercepts another suite's traffic.
private final class LifecycleGuardStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body: Data = Data()
    }

    static let port = 18915
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

    override class func canInit(with request: URLRequest) -> Bool {
        request.url?.port == port
    }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response = Self.lock.withLock {
            Self.recorded.append(request)
            return Self.handler?(request) ?? Response()
        }
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}

/// Exercises the fix for #995 (F4): `newConversation`/`fork`/`undo` never
/// consulted `run?.isBusy`, so any of the six call sites listed in KTD-9
/// could race a running turn and silently reset or replace the conversation
/// underneath it. The guard lives once inside `ProjectSession` rather than
/// at each call site, so these tests drive the session methods directly --
/// the same shape `ProjectSessionActivityTests` uses.
@Suite("ProjectSession lifecycle guard", .serialized)
@MainActor
struct ProjectSessionLifecycleGuardTests {

    private static let baseURL = URL(string: "http://127.0.0.1:\(LifecycleGuardStub.port)")!

    private func makeProject() -> ProjectSession {
        URLProtocol.registerClass(LifecycleGuardStub.self)
        return ProjectSession(
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()),
            externalBaseURL: Self.baseURL)
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

    /// Puts `project.run` into a busy state on conversation `conv_1`.
    /// `RunSession.submit()` sets `transcript.runState = .queued`
    /// synchronously -- before its background task even reaches the server
    /// -- so `isBusy` is already true the moment this returns. The run's
    /// events stream answers with an empty body (no terminal event), so the
    /// run stays busy for the rest of a test unless it explicitly waits for
    /// completion.
    private func makeBusyProject() async -> ProjectSession {
        let project = makeProject()
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(
                    status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_1/events"):
                return .init(status: 200, body: Data())
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        await project.start()
        project.run?.rebind(conversationID: "conv_1")
        project.run?.draft = "keep going"
        project.run?.submit()
        return project
    }

    @Test("newConversation refuses while a run is active -- core regression")
    func newConversationRefusesWhileBusy() async throws {
        LifecycleGuardStub.reset()
        let project = await makeBusyProject()

        project.newConversation()

        #expect(project.run?.conversationID == "conv_1")
        #expect(project.run?.isBusy == true)
        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)

        project.run?.reset()
    }

    @Test("fork refuses while a run is active and never reaches the server")
    func forkRefusesWhileBusy() async throws {
        LifecycleGuardStub.reset()
        let project = await makeBusyProject()

        await project.fork()

        #expect(LifecycleGuardStub.requests(matching: "/v1/conversations/conv_1/fork").isEmpty)
        #expect(project.run?.conversationID == "conv_1")
        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)

        project.run?.reset()
    }

    @Test("undo refuses while a run is active and never reaches the server")
    func undoRefusesWhileBusy() async throws {
        LifecycleGuardStub.reset()
        let project = await makeBusyProject()

        await project.undo()

        #expect(LifecycleGuardStub.requests(matching: "/v1/conversations/conv_1/undo").isEmpty)
        #expect(project.run?.conversationID == "conv_1")
        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)

        project.run?.reset()
    }

    @Test("with no active run, fork, undo, and newConversation behave exactly as before")
    func idlePathUnaffected() async throws {
        LifecycleGuardStub.reset()
        let project = makeProject()
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/conversations/conv_1/fork"):
                return .init(status: 200, body: Data(#"{"conversation_id":"conv_2"}"#.utf8))
            case ("POST", "/v1/conversations/conv_2/undo"):
                return .init(status: 200)
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        await project.start()
        project.run?.rebind(conversationID: "conv_1")
        #expect(project.run?.isBusy == false)

        await project.fork()
        #expect(project.run?.conversationID == "conv_2", "fork rebinds to the server's new id")
        #expect(project.statusMessage == "Forked into a new conversation")

        await project.undo()
        #expect(
            !LifecycleGuardStub.requests(matching: "/v1/conversations/conv_2/undo").isEmpty,
            "undo reloads the conversation it was called on")

        project.newConversation()
        #expect(project.run?.conversationID == nil, "new resets the conversation")
    }

    @Test(
        "a previously refused fork succeeds once the run completes -- guard is state-based, not sticky"
    )
    func guardClearsWhenRunCompletes() async throws {
        LifecycleGuardStub.reset()
        let project = makeProject()
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(
                    status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.completed
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.completed","payload":{}}


                    """
                return .init(status: 200, body: Data(frame.utf8))
            case ("POST", "/v1/conversations/conv_1/fork"):
                return .init(status: 200, body: Data(#"{"conversation_id":"conv_2"}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        await project.start()
        project.run?.rebind(conversationID: "conv_1")
        project.run?.draft = "hi"
        project.run?.submit()

        await project.fork()
        #expect(
            LifecycleGuardStub.requests(matching: "/v1/conversations/conv_1/fork").isEmpty,
            "the run has not completed yet -- fork must still be refused")

        try await wait { project.run?.transcript.runState == .completed }

        await project.fork()
        #expect(
            !LifecycleGuardStub.requests(matching: "/v1/conversations/conv_1/fork").isEmpty,
            "the run is no longer busy -- fork must now reach the server")
        #expect(project.run?.conversationID == "conv_2")

        project.run?.reset()
    }

    /// `deleteConversation`'s own call to `newConversation()` (`:349`) only
    /// fires when the deleted conversation is the one currently open. If a
    /// run is still active on that conversation when the delete server-call
    /// succeeds, the inherited guard must refuse the reset explicitly rather
    /// than silently dropping the busy run's local state (U4 Approach step
    /// 2) -- this is the interaction the plan calls out as intentional, not
    /// incidental.
    @Test("deleteConversation's internal reset inherits the guard for its own busy run")
    func deleteConversationInheritsGuardForOwnBusyRun() async throws {
        LifecycleGuardStub.reset()
        let project = await makeBusyProject()
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("DELETE", "/v1/conversations/conv_1"):
                return .init(status: 200)
            case ("POST", "/v1/runs"):
                return .init(
                    status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_1/events"):
                return .init(status: 200, body: Data())
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        let conversation = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"conv_1"}"#.utf8))

        await project.deleteConversation(conversation)

        #expect(
            project.run?.conversationID == "conv_1",
            "the guard refuses the reset; the deleted conversation's still-busy run must not be silently dropped"
        )
        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)

        project.run?.reset()
    }
}
