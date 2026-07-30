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

    /// Exercises the fix for #995 (F4): `deleteConversation` used to call the
    /// server's `DELETE` *before* consulting busyness at all, relying on its
    /// own internal `newConversation()` call (`:349`) to refuse the local
    /// reset afterwards -- which left the conversation actually deleted on
    /// the server while the app stayed bound to it. The guard must now refuse
    /// up front, before the server is ever contacted.
    @Test(
        "deleteConversation refuses up front for its own busy conversation -- the DELETE must never reach the server -- core regression"
    )
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
            LifecycleGuardStub.requests(matching: "/v1/conversations/conv_1").filter {
                $0.httpMethod == "DELETE"
            }.isEmpty,
            "a busy conversation's delete must be refused before the server is ever contacted, not deleted and then locally refused"
        )
        #expect(
            project.run?.conversationID == "conv_1",
            "the guard refuses the reset; the deleted conversation's still-busy run must not be silently dropped"
        )
        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)

        project.run?.reset()
    }

    /// Exercises the fix for #995 (F6): `openConversation` had no busy guard
    /// at all, so switching conversations mid-run would load a different
    /// conversation's messages into the transcript out from under an active
    /// run's stream tracking.
    @Test("openConversation refuses while a run is active and never reaches the server")
    func openConversationRefusesWhileBusy() async throws {
        LifecycleGuardStub.reset()
        let project = await makeBusyProject()
        let other = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"conv_2"}"#.utf8))

        await project.openConversation(other)

        #expect(LifecycleGuardStub.requests(matching: "/v1/conversations/conv_2/messages").isEmpty)
        #expect(project.run?.conversationID == "conv_1")
        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)

        project.run?.reset()
    }

    @Test("openConversation succeeds when idle")
    func openConversationSucceedsWhenIdle() async throws {
        LifecycleGuardStub.reset()
        let project = makeProject()
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/v1/conversations/conv_2/messages"):
                return .init(status: 200, body: Data(#"{"messages":[]}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        await project.start()
        let other = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"conv_2"}"#.utf8))

        await project.openConversation(other)

        #expect(!LifecycleGuardStub.requests(matching: "/v1/conversations/conv_2/messages").isEmpty)
        #expect(project.run?.conversationID == "conv_2")
    }

    /// Exercises the fix for #995 (F5): `fork`/`undo` only checked busyness
    /// *before* their server call, not after -- if a run started on this
    /// same conversation while the request was in flight, the result was
    /// applied anyway, retargeting (`fork`) or reloading (`undo`) the
    /// conversation out from under the run that just started.
    @Test(
        "fork re-checks busy after the server call: a run started mid-flight is not clobbered -- core regression"
    )
    func forkReCheckusBusyAfterAwait() async throws {
        LifecycleGuardStub.reset()
        let project = makeProject()
        let forkArrived = Flag()
        let releaseFork = DispatchSemaphore(value: 0)
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/conversations/conv_1/fork"):
                forkArrived.set()
                releaseFork.wait()
                return .init(status: 200, body: Data(#"{"conversation_id":"conv_2"}"#.utf8))
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

        let forkTask = Task { await project.fork() }
        // `LifecycleGuardStub` serializes every request through one global
        // lock held for the handler's duration -- polling
        // `.requests(matching:)` here would contend for that same lock
        // while fork's handler is still blocked holding it. `forkArrived`
        // observes "the request reached the stub" without touching it. A
        // blocking wait would be just as wrong here: it would stall the
        // MainActor executor that `forkTask` itself needs in order to run
        // far enough to send the request in the first place.
        try await wait { forkArrived.isSet }

        // A run starts on the original conversation while fork's request to
        // the server is still in flight. `RunSession.submit()` marks
        // `isBusy` synchronously (before its own `POST /v1/runs` even
        // reaches this same stub, where it would itself queue behind
        // fork's still-held lock) -- so this does not depend on that
        // request completing.
        project.run?.draft = "keep going"
        project.run?.submit()
        try await wait { project.run?.isBusy == true }

        releaseFork.signal()
        await forkTask.value

        #expect(
            project.run?.conversationID == "conv_1",
            "fork must not rebind onto the forked conversation while a run started mid-flight is still active"
        )

        project.run?.reset()
    }

    @Test(
        "undo re-checks busy after the server call: a run started mid-flight is not clobbered -- core regression"
    )
    func undoReCheckusBusyAfterAwait() async throws {
        LifecycleGuardStub.reset()
        let project = makeProject()
        let undoArrived = Flag()
        let releaseUndo = DispatchSemaphore(value: 0)
        LifecycleGuardStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/conversations/conv_1/undo"):
                undoArrived.set()
                releaseUndo.wait()
                return .init(status: 200)
            case ("GET", "/v1/conversations/conv_1/messages"):
                return .init(status: 200, body: Data(#"{"messages":[{"role":"user"}]}"#.utf8))
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

        let undoTask = Task { await project.undo() }
        try await wait { undoArrived.isSet }

        project.run?.draft = "keep going"
        project.run?.submit()
        try await wait { project.run?.isBusy == true }

        releaseUndo.signal()
        await undoTask.value

        #expect(
            LifecycleGuardStub.requests(matching: "/v1/conversations/conv_1/messages").isEmpty,
            "undo must not reload the conversation's messages while a run started mid-flight is still active"
        )

        project.run?.reset()
    }

    /// Regression angle distinct from the busy-state tests above: those all
    /// exercise `run?.isBusy == true`. This proves the guard's nil-coalescing
    /// reads the *other* direction correctly too -- before a project ever
    /// connects, `run` itself is nil, and nil must never be mistaken for
    /// busy. A guard written as `run?.isBusy != false` (instead of
    /// `run?.isBusy == true`) would pass every test above and still wrongly
    /// refuse every lifecycle action on a session that has no run at all.
    @Test("the guard does not fire when there is no run at all -- nil is not busy")
    func guardDoesNotFireWithNoRun() async throws {
        let project = ProjectSession(workspace: URL(fileURLWithPath: NSTemporaryDirectory()))
        #expect(project.run == nil)

        project.newConversation()
        #expect(project.statusMessage == nil)

        await project.fork()
        #expect(project.statusMessage == nil)

        await project.undo()
        #expect(project.statusMessage == nil)
    }

    /// Regression angle distinct from the busy-state tests above: those only
    /// assert each message contains "running". A revert that collapses all
    /// three refusals to one shared generic string (e.g. "Action refused --
    /// a run is active") would still satisfy that assertion while losing
    /// R5's "refuse (with an explanation)" naming the specific action --
    /// this pins that each of the three messages is distinct and names its
    /// own action.
    @Test("each guarded action's refusal message names that specific action")
    func refusalMessagesNameTheirOwnAction() async throws {
        LifecycleGuardStub.reset()
        let newConversationProject = await makeBusyProject()
        newConversationProject.newConversation()
        let newConversationMessage = try #require(newConversationProject.statusMessage)
        newConversationProject.run?.reset()

        LifecycleGuardStub.reset()
        let forkProject = await makeBusyProject()
        await forkProject.fork()
        let forkMessage = try #require(forkProject.statusMessage)
        forkProject.run?.reset()

        LifecycleGuardStub.reset()
        let undoProject = await makeBusyProject()
        await undoProject.undo()
        let undoMessage = try #require(undoProject.statusMessage)
        undoProject.run?.reset()

        #expect(newConversationMessage != forkMessage)
        #expect(forkMessage != undoMessage)
        #expect(newConversationMessage != undoMessage)
        #expect(newConversationMessage.localizedCaseInsensitiveContains("new conversation"))
        #expect(forkMessage.localizedCaseInsensitiveContains("fork"))
        #expect(undoMessage.localizedCaseInsensitiveContains("undo"))
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
