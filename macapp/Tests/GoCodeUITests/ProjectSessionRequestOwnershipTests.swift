import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// A deliberately blocking URL protocol for request-ownership regressions.
/// Each test arms selected paths after `ProjectSession.start()` has finished
/// its connection-time refreshes. The first armed request waits until the
/// test releases it; a later request receives a newer payload immediately.
/// This makes response ordering, rather than request ordering, observable.
private final class RequestOwnershipStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body = Data()
        /// The response is delivered asynchronously after this gate opens.
        /// `URLProtocol.startLoading()` itself must return immediately so a
        /// newer URLSession request can race the older response.
        var completionGate: DispatchSemaphore?
    }

    static let port = 18921
    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    override class func canInit(with request: URLRequest) -> Bool {
        request.url?.port == port
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let response = Self.lock.withLock { Self.handler }?(request) ?? Response()
        if let gate = response.completionGate {
            DispatchQueue.global().async { [self] in
                gate.wait()
                finishLoading(response)
            }
            return
        }
        finishLoading(response)
    }

    private func finishLoading(_ response: Response) {
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}

/// Thread-safe response sequencer. Never holds its lock while waiting: the
/// test needs a newer URLSession request to make progress while the older one
/// is deliberately paused.
private final class RequestOwnershipResponses: @unchecked Sendable {
    private let lock = NSLock()
    private var armedPaths: Set<String> = []
    private var requestsByPath: [String: Int] = [:]
    private var reachedPaths: Set<String> = []
    private var releases: [String: DispatchSemaphore] = [:]
    private let oldBodies: [String: Data]
    private let newBodies: [String: Data]

    init(oldBodies: [String: Data], newBodies: [String: Data]) {
        self.oldBodies = oldBodies
        self.newBodies = newBodies
    }

    func arm(_ paths: Set<String>) {
        lock.withLock {
            armedPaths = paths
            requestsByPath = [:]
            reachedPaths = []
            releases = Dictionary(
                uniqueKeysWithValues: paths.map { ($0, DispatchSemaphore(value: 0)) })
        }
    }

    func response(for request: URLRequest) -> RequestOwnershipStub.Response {
        guard let path = request.url?.path else { return .init() }
        guard armedPathsSnapshotContains(path) else {
            return .init(body: newBodies[path] ?? baseline(path: path).body)
        }
        let firstRequest: Bool = lock.withLock {
            let count = requestsByPath[path, default: 0] + 1
            requestsByPath[path] = count
            if count == 1 {
                reachedPaths.insert(path)
                return true
            }
            return false
        }

        if firstRequest {
            return .init(body: oldBodies[path] ?? Data(), completionGate: releasesForPath(path))
        }
        return .init(body: newBodies[path] ?? Data())
    }

    func reached(_ paths: Set<String>) -> Bool {
        lock.withLock { paths.isSubset(of: reachedPaths) }
    }

    func release(_ paths: Set<String>) {
        let semaphores = lock.withLock { paths.compactMap { releases[$0] } }
        for semaphore in semaphores { semaphore.signal() }
    }

    private func armedPathsSnapshotContains(_ path: String) -> Bool {
        lock.withLock { armedPaths.contains(path) }
    }

    private func releasesForPath(_ path: String) -> DispatchSemaphore {
        lock.withLock { releases[path]! }
    }

    private func baseline(path: String) -> RequestOwnershipStub.Response {
        switch path {
        case "/v1/models":
            return .init(
                body: Data(#"{"models":[{"id":"baseline-model","provider":"openai"}]}"#.utf8))
        case "/v1/providers":
            return .init(
                body: Data(#"{"providers":[{"name":"baseline-provider","configured":true}]}"#.utf8))
        case "/v1/profiles":
            return .init(body: Data(#"{"profiles":[{"name":"baseline-profile"}]}"#.utf8))
        case "/v1/conversations/":
            return .init(body: Data(#"{"conversations":[]}"#.utf8))
        case "/v1/tasks":
            return .init(body: Data(#"{"tasks":[]}"#.utf8))
        case "/v1/runs":
            return .init(body: Data(#"{"runs":[]}"#.utf8))
        default:
            return .init(body: Data(#"{}"#.utf8))
        }
    }
}

@Suite("ProjectSession request ownership", .serialized)
@MainActor
struct ProjectSessionRequestOwnershipTests {
    private static let baseURL = URL(string: "http://127.0.0.1:\(RequestOwnershipStub.port)")!

    private func makeProject(_ responses: RequestOwnershipResponses) -> ProjectSession {
        URLProtocol.registerClass(RequestOwnershipStub.self)
        RequestOwnershipStub.set { responses.response(for: $0) }
        return ProjectSession(
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()), externalBaseURL: Self.baseURL)
    }

    private func wait(
        timeout: Duration = .seconds(3), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("timed out waiting for a request")
    }

    private func start(_ project: ProjectSession) async throws {
        await project.start()
        // `connect(to:)` starts two unstructured baseline refreshes. Let their
        // non-blocking stub responses finish before arming this test's race.
        try await Task.sleep(for: .milliseconds(50))
    }

    @Test("a late conversations refresh cannot replace a newer list")
    func conversationsKeepLatestResponse() async throws {
        let path = "/v1/conversations/"
        let responses = RequestOwnershipResponses(
            oldBodies: [path: Data(#"{"conversations":[{"id":"old-conversation"}]}"#.utf8)],
            newBodies: [path: Data(#"{"conversations":[{"id":"new-conversation"}]}"#.utf8)])
        let project = makeProject(responses)
        try await start(project)
        responses.arm([path])

        let older = Task { await project.refreshConversations() }
        try await wait { responses.reached([path]) }
        await project.refreshConversations()
        responses.release([path])
        await older.value

        #expect(project.conversations.map(\.id) == ["new-conversation"])
    }

    @Test("each catalog collection keeps the newest independently refreshed value")
    func catalogCollectionsKeepLatestResponsesIndependently() async throws {
        let modelPath = "/v1/models"
        let providerPath = "/v1/providers"
        let profilePath = "/v1/profiles"
        let paths: Set<String> = [modelPath, providerPath, profilePath]
        let responses = RequestOwnershipResponses(
            oldBodies: [
                modelPath: Data(#"{"models":[{"id":"old-model","provider":"openai"}]}"#.utf8),
                providerPath: Data(
                    #"{"providers":[{"name":"old-provider","configured":true}]}"#.utf8),
                profilePath: Data(#"{"profiles":[{"name":"old-profile"}]}"#.utf8),
            ],
            newBodies: [
                modelPath: Data(#"{"models":[{"id":"new-model","provider":"openai"}]}"#.utf8),
                providerPath: Data(
                    #"{"providers":[{"name":"new-provider","configured":true}]}"#.utf8),
                profilePath: Data(#"{"profiles":[{"name":"new-profile"}]}"#.utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        responses.arm(paths)

        let older = Task { await project.refreshCatalog() }
        try await wait { responses.reached(paths) }
        await project.refreshCatalog()
        responses.release(paths)
        await older.value

        #expect(project.models.map(\.id) == ["new-model"])
        #expect(project.providers.map(\.name) == ["new-provider"])
        #expect(project.profiles.map(\.name) == ["new-profile"])
    }

    @Test("a late activity refresh cannot replace newer tasks, runs, or current-run todos")
    func activityCollectionsKeepLatestResponsesIndependently() async throws {
        let taskPath = "/v1/tasks"
        let runPath = "/v1/runs"
        let todoPath = "/v1/runs/run-current/todos"
        let paths: Set<String> = [taskPath, runPath, todoPath]
        let responses = RequestOwnershipResponses(
            oldBodies: [
                taskPath: Data(
                    #"{"tasks":[{"id":"old-task","type":"cron","status":"running","label":"old"}]}"#
                        .utf8),
                runPath: Data(#"{"runs":[{"id":"old-run"}]}"#.utf8),
                todoPath: Data(
                    #"{"todos":[{"id":"old-todo","text":"old","status":"pending"}]}"#.utf8),
            ],
            newBodies: [
                taskPath: Data(
                    #"{"tasks":[{"id":"new-task","type":"cron","status":"running","label":"new"}]}"#
                        .utf8),
                runPath: Data(#"{"runs":[{"id":"new-run"}]}"#.utf8),
                todoPath: Data(
                    #"{"todos":[{"id":"new-todo","text":"new","status":"pending"}]}"#.utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        let eventRelease = DispatchSemaphore(value: 0)
        RequestOwnershipStub.set { request in
            if request.httpMethod == "POST", request.url?.path == "/v1/runs" {
                return .init(
                    status: 202, body: Data(#"{"run_id":"run-current","status":"queued"}"#.utf8))
            }
            if request.url?.path == "/v1/runs/run-current/events" {
                return .init(completionGate: eventRelease)
            }
            return responses.response(for: request)
        }
        project.run?.draft = "make activity current"
        project.run?.submit()
        try await wait { project.run?.currentRunID == "run-current" }
        responses.arm(paths)

        let older = Task { await project.refreshActivity() }
        try await wait { responses.reached(paths) }
        await project.refreshActivity()
        responses.release(paths)
        await older.value

        #expect(project.tasks.map(\.id) == ["new-task"])
        #expect(project.runs?.map(\.id) == ["new-run"])
        #expect(project.todos.map(\.text) == ["new"])
        eventRelease.signal()
        project.run?.reset()
    }

    @Test("a run ending during todo fetch still applies independently valid tasks and runs")
    func terminalRunOnlyDiscardsItsTodos() async throws {
        let taskPath = "/v1/tasks"
        let runPath = "/v1/runs"
        let todoPath = "/v1/runs/run-current/todos"
        let responses = RequestOwnershipResponses(
            oldBodies: [
                todoPath: Data(
                    #"{"todos":[{"id":"stale-todo","text":"stale","status":"pending"}]}"#.utf8)
            ],
            newBodies: [
                taskPath: Data(
                    #"{"tasks":[{"id":"current-task","type":"cron","status":"running","label":"current"}]}"#
                        .utf8),
                runPath: Data(#"{"runs":[{"id":"current-summary"}]}"#.utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        let eventRelease = DispatchSemaphore(value: 0)
        let terminalEvent = Data(
            """
            id: run-current:1
            event: run.completed
            data: {"id":"run-current:1","run_id":"run-current","type":"run.completed","payload":{}}


            """.utf8)
        RequestOwnershipStub.set { request in
            if request.httpMethod == "POST", request.url?.path == "/v1/runs" {
                return .init(
                    status: 202, body: Data(#"{"run_id":"run-current","status":"queued"}"#.utf8))
            }
            if request.url?.path == "/v1/runs/run-current/events" {
                return .init(body: terminalEvent, completionGate: eventRelease)
            }
            return responses.response(for: request)
        }
        project.run?.draft = "make activity current"
        project.run?.submit()
        try await wait { project.run?.currentRunID == "run-current" }
        responses.arm([todoPath])

        let refresh = Task { await project.refreshActivity() }
        try await wait { responses.reached([todoPath]) }
        eventRelease.signal()
        try await wait { project.run?.currentRunID == nil }
        responses.release([todoPath])
        await refresh.value

        #expect(project.tasks.map(\.id) == ["current-task"])
        #expect(project.runs?.map(\.id) == ["current-summary"])
        #expect(project.tasksLoadState == .loaded)
        #expect(project.runsLoadState == .loaded)
        #expect(project.todos.isEmpty)
        #expect(project.todosLoadState == .loaded)
        project.run?.reset()
    }

    @Test("ready tasks and runs apply while current-run todos are still loading")
    func activityDoesNotBlockIndependentCollectionsOnTodos() async throws {
        let taskPath = "/v1/tasks"
        let runPath = "/v1/runs"
        let todoPath = "/v1/runs/run-current/todos"
        let responses = RequestOwnershipResponses(
            oldBodies: [
                todoPath: Data(
                    #"{"todos":[{"id":"old-todo","text":"old","status":"pending"}]}"#.utf8)
            ],
            newBodies: [
                taskPath: Data(
                    #"{"tasks":[{"id":"ready-task","type":"cron","status":"running","label":"ready"}]}"#
                        .utf8),
                runPath: Data(#"{"runs":[{"id":"ready-run"}]}"#.utf8),
                todoPath: Data(
                    #"{"todos":[{"id":"ready-todo","text":"ready","status":"pending"}]}"#.utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        let eventRelease = DispatchSemaphore(value: 0)
        RequestOwnershipStub.set { request in
            if request.httpMethod == "POST", request.url?.path == "/v1/runs" {
                return .init(
                    status: 202,
                    body: Data(#"{"run_id":"run-current","status":"queued"}"#.utf8))
            }
            if request.url?.path == "/v1/runs/run-current/events" {
                return .init(completionGate: eventRelease)
            }
            return responses.response(for: request)
        }
        project.run?.draft = "keep todos loading"
        project.run?.submit()
        try await wait { project.run?.currentRunID == "run-current" }
        responses.arm([todoPath])

        let refresh = Task { await project.refreshActivity() }
        try await wait { responses.reached([todoPath]) }
        try await wait {
            project.tasks.map(\.id) == ["ready-task"]
                && project.runs?.map(\.id) == ["ready-run"]
        }
        #expect(project.todosLoadState == .loading)

        responses.release([todoPath])
        await refresh.value
        eventRelease.signal()
        project.run?.reset()
    }

    @Test("a late rewind-point response for an old conversation is discarded")
    func rewindPointsValidateTheirConversationTarget() async throws {
        let oldPath = "/v1/conversations/old-conversation/rewind-points"
        let newPath = "/v1/conversations/new-conversation/rewind-points"
        let responses = RequestOwnershipResponses(
            oldBodies: [oldPath: Data(#"{"points":[{"id":"old-point"}]}"#.utf8)],
            newBodies: [
                oldPath: Data(#"{"points":[{"id":"ignored-old-point"}]}"#.utf8),
                newPath: Data(#"{"points":[{"id":"new-point"}]}"#.utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        project.run?.rebind(conversationID: "old-conversation")
        responses.arm([oldPath])

        let older = Task { await project.refreshRewindPoints() }
        try await wait { responses.reached([oldPath]) }
        project.run?.rebind(conversationID: "new-conversation")
        await project.refreshRewindPoints()
        responses.release([oldPath])
        await older.value

        #expect(project.rewindPoints.map(\.id) == ["new-point"])
    }

    @Test("a pending open of an older conversation cannot win after a newer selection")
    func openConversationKeepsLatestSelection() async throws {
        let oldPath = "/v1/conversations/old-conversation/messages"
        let newPath = "/v1/conversations/new-conversation/messages"
        let responses = RequestOwnershipResponses(
            oldBodies: [
                oldPath: Data(
                    #"{"messages":[{"role":"assistant","content":"old reply","step":0}]}"#.utf8)
            ],
            newBodies: [
                oldPath: Data(#"{"messages":[]}"#.utf8),
                newPath: Data(
                    #"{"messages":[{"role":"assistant","content":"new reply","step":0}]}"#.utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        let old = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"old-conversation"}"#.utf8))
        let new = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"new-conversation"}"#.utf8))
        responses.arm([oldPath])

        let older = Task { await project.openConversation(old) }
        try await wait { responses.reached([oldPath]) }
        await project.openConversation(new)
        responses.release([oldPath])
        await older.value

        #expect(project.run?.conversationID == "new-conversation")
        #expect(
            project.run?.transcript.items.contains {
                if case .assistantMessage(let message) = $0.kind {
                    return message.text == "new reply"
                }
                return false
            } == true)
    }

    @Test("an open refused by a run starting in flight releases selection ownership")
    func busyRefusalDoesNotLeaveConversationSyncBlocked() async throws {
        let blockedPath = "/v1/conversations/blocked/messages"
        let currentPath = "/v1/conversations/current/messages"
        let responses = RequestOwnershipResponses(
            oldBodies: [blockedPath: Data(#"{"messages":[]}"#.utf8)],
            newBodies: [
                blockedPath: Data(#"{"messages":[]}"#.utf8),
                currentPath: Data(
                    #"{"messages":[{"role":"assistant","content":"sync recovered","step":0}]}"#
                        .utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        project.run?.rebind(conversationID: "current")
        let blocked = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"blocked"}"#.utf8))
        responses.arm([blockedPath])

        let opening = Task { await project.openConversation(blocked) }
        try await wait { responses.reached([blockedPath]) }
        project.run?.draft = "start while open is pending"
        project.run?.submit()
        #expect(project.run?.isBusy == true)
        responses.release([blockedPath])
        await opening.value

        project.run?.reset()
        project.run?.rebind(conversationID: "current")
        await project.syncCurrentConversation()

        #expect(
            project.run?.transcript.items.contains {
                if case .assistantMessage(let message) = $0.kind {
                    return message.text == "sync recovered"
                }
                return false
            } == true,
            "a busy refusal must not leave pending selection ownership blocking later syncs")
    }

    @Test("a late durable sync cannot reconcile messages after a newer selection starts")
    func syncValidatesSelectionBeforeReconciling() async throws {
        let currentPath = "/v1/conversations/current/messages"
        let nextPath = "/v1/conversations/next/messages"
        let responses = RequestOwnershipResponses(
            oldBodies: [
                currentPath: Data(
                    #"{"messages":[{"role":"assistant","content":"stale durable reply","step":0}]}"#
                        .utf8)
            ],
            newBodies: [
                currentPath: Data(#"{"messages":[]}"#.utf8),
                nextPath: Data(
                    #"{"messages":[{"role":"assistant","content":"new durable reply","step":0}]}"#
                        .utf8),
            ])
        let project = makeProject(responses)
        try await start(project)
        project.run?.rebind(conversationID: "current")
        let next = try JSONDecoder().decode(
            ConversationInfo.self, from: Data(#"{"id":"next"}"#.utf8))
        responses.arm([currentPath])

        let sync = Task { await project.syncCurrentConversation() }
        try await wait { responses.reached([currentPath]) }
        await project.openConversation(next)
        responses.release([currentPath])
        await sync.value

        #expect(project.run?.conversationID == "next")
        #expect(
            project.run?.transcript.items.contains {
                if case .assistantMessage(let message) = $0.kind {
                    return message.text == "stale durable reply"
                }
                return false
            } == false)
    }

    @Test("rewind refuses at the session boundary while a run is active")
    func rewindRefusesWhileBusy() async throws {
        let rewindPath = "/v1/conversations/busy-conversation/rewind"
        let responses = RequestOwnershipResponses(oldBodies: [:], newBodies: [:])
        let project = makeProject(responses)
        try await start(project)
        RequestOwnershipStub.set { request in
            if request.httpMethod == "POST", request.url?.path == "/v1/runs" {
                return .init(
                    status: 202, body: Data(#"{"run_id":"busy-run","status":"queued"}"#.utf8))
            }
            if request.url?.path == "/v1/runs/busy-run/events" { return .init() }
            if request.url?.path == rewindPath {
                Issue.record("rewind reached the server while a run was active")
            }
            return responses.response(for: request)
        }
        project.run?.rebind(conversationID: "busy-conversation")
        project.run?.draft = "stay busy"
        project.run?.submit()
        try await wait { project.run?.isBusy == true }
        let point = try JSONDecoder().decode(RewindPoint.self, from: Data(#"{"id":"point"}"#.utf8))

        await project.rewind(to: point)

        #expect(project.statusMessage?.localizedCaseInsensitiveContains("running") == true)
        project.run?.reset()
    }
}
