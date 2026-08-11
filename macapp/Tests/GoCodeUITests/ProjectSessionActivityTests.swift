import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP stub for exercising `ProjectSession` without a live harnessd.
///
/// `ProjectSession.connect(to:)` always builds its `HarnessClient` with the
/// default `URLSession.shared` (there is no injection seam — see #951 finding
/// 2, out of scope here), so the stub has to register globally rather than
/// through a per-session configuration. Scoped to one fixed loopback port so
/// it never intercepts another suite's traffic.
private final class ActivityStubProtocol: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body: Data = Data()
    }

    static let port = 18912
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
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}

/// `refreshActivity` used `runs = try? await client.runs()`, which conflates
/// a transport failure with `client.runs()`'s deliberate `nil` for a 501 "no
/// run store configured" response — so a network blip displayed the same
/// message as a daemon that was never configured to persist runs at all
/// (#951 finding 3).
@Suite("ProjectSession activity refresh", .serialized)
@MainActor
struct ProjectSessionActivityTests {

    private static let baseURL = URL(string: "http://127.0.0.1:\(ActivityStubProtocol.port)")!

    private func makeProject() -> ProjectSession {
        URLProtocol.registerClass(ActivityStubProtocol.self)
        return ProjectSession(
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()),
            externalBaseURL: Self.baseURL)
    }

    /// Boxes the mutable status code so the stub's `@Sendable` handler
    /// closure can read a value the test changes between calls.
    private final class Box: @unchecked Sendable {
        private let lock = NSLock()
        private var value: Int
        init(_ value: Int) { self.value = value }
        var current: Int {
            get { lock.withLock { value } }
            set { lock.withLock { value = newValue } }
        }
    }

    private final class DataBox: @unchecked Sendable {
        private let lock = NSLock()
        private var value: Data
        init(_ value: Data) { self.value = value }
        var current: Data {
            get { lock.withLock { value } }
            set { lock.withLock { value = newValue } }
        }
    }

    private final class Paths: @unchecked Sendable {
        private let lock = NSLock()
        private var values: [String] = []
        func append(_ value: String) { lock.withLock { values.append(value) } }
        var snapshot: [String] { lock.withLock { values } }
    }

    @Test("a genuine 501 stays nil with no error, but a real failure surfaces via statusMessage")
    func distinguishesTransportErrorFromNoStoreSignal() async throws {
        let project = makeProject()
        let runsStatus = Box(501)
        ActivityStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/runs":
                switch runsStatus.current {
                case 200:
                    return .init(status: 200, body: Data(#"{"runs":[{"id":"run_1"}]}"#.utf8))
                case 501:
                    return .init(
                        status: 501,
                        body: Data(
                            #"{"error":{"code":"not_configured","message":"no store"}}"#.utf8))
                default:
                    return .init(
                        status: 500, body: Data(#"{"error":{"code":"boom","message":"boom"}}"#.utf8)
                    )
                }
            default:
                // Every other endpoint the connect-time background refreshes
                // hit — keep them boring so only /v1/runs drives the test.
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()

        // Daemon genuinely has no run store: normal state, no error.
        await project.refreshActivity()
        #expect(project.runs == nil)
        #expect(project.statusMessage == nil)

        // A real dataset now exists.
        runsStatus.current = 200
        await project.refreshActivity()
        #expect(project.runs?.count == 1)

        // Now a genuine server error — must surface, and must not silently
        // read as "no run store configured" by wiping the last-known data.
        runsStatus.current = 500
        await project.refreshActivity()
        #expect(project.statusMessage != nil)
        #expect(project.runs?.count == 1)
    }

    /// Regression angle distinct from the `runs`-specific test above: the
    /// fix applies to every `refresh*` method, not just `refreshActivity`.
    /// `refreshCatalog` must also surface a failure via `statusMessage`
    /// instead of swallowing it, and must not blank out a catalog that
    /// already loaded successfully.
    @Test("refreshCatalog surfaces a failure without discarding an already-loaded catalog")
    func refreshCatalogSurfacesFailureWithoutDiscardingData() async throws {
        let project = makeProject()
        let modelsStatus = Box(200)
        ActivityStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/models":
                switch modelsStatus.current {
                case 200:
                    return .init(
                        status: 200,
                        body: Data(
                            #"{"models":[{"id":"gpt-5","provider":"openai"}]}"#.utf8))
                default:
                    return .init(
                        status: 500, body: Data(#"{"error":{"code":"boom","message":"boom"}}"#.utf8)
                    )
                }
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()

        await project.refreshCatalog()
        #expect(project.models.count == 1)
        #expect(project.statusMessage == nil)

        modelsStatus.current = 500
        await project.refreshCatalog()
        #expect(project.statusMessage != nil)
        #expect(project.models.count == 1, "a failed refresh must not blank the working catalog")
    }

    /// Regression for #1008: a cron/callback run may finish while Activity is
    /// visible. Returning to Chat must reconcile the durable message log even
    /// if no later SSE event arrives to shake the in-memory transcript forward.
    @Test("Chat re-entry sync restores a completed scheduled message")
    func syncCurrentConversationRestoresScheduledMessage() async throws {
        let project = makeProject()
        let messages = DataBox(
            Data(
                #"{"messages":[{"role":"user","content":"watch deployment","step":0},{"role":"assistant","content":"watching","step":0}]}"#
                    .utf8))
        ActivityStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/conversations/conv-scheduled/messages":
                return .init(status: 200, body: messages.current)
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()
        let run = try #require(project.run)
        run.rebind(conversationID: "conv-scheduled")

        await project.syncCurrentConversation()
        #expect(
            run.transcript.items.contains {
                if case .assistantMessage(let message) = $0.kind {
                    return message.text == "watching"
                }
                return false
            })

        messages.current = Data(
            #"{"messages":[{"role":"user","content":"watch deployment","step":0},{"role":"assistant","content":"watching","step":0},{"role":"user","content":"scheduled monitor","step":1},{"role":"assistant","content":"deployment passed","step":1}]}"#
                .utf8)
        await project.syncCurrentConversation()

        #expect(
            run.transcript.items.contains {
                if case .assistantMessage(let message) = $0.kind {
                    return message.text == "deployment passed"
                }
                return false
            },
            "persisted scheduled reply did not appear when Chat re-entered")
    }

    @Test("a stale task action surfaces the server error and reconciles its authoritative row")
    func staleTaskActionReconcilesAuthoritativeRow() async throws {
        let project = makeProject()
        let active = Data(
            #"{"tasks":[{"id":"cron-1","type":"cron","status":"active","label":"watch","started_at":"2026-08-04T12:00:00Z","age_seconds":0,"actions":["pause","delete"]}]}"#
                .utf8)
        let paused = Data(
            #"{"tasks":[{"id":"cron-1","type":"cron","status":"paused","label":"watch","started_at":"2026-08-04T12:00:00Z","age_seconds":0,"actions":["resume","delete"]}]}"#
                .utf8)
        ActivityStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/tasks": return .init(body: active)
            default: return .init(body: Data("{}".utf8))
            }
        }
        await project.start()
        await project.refreshActivity()
        let task = try #require(project.tasks.first)
        let paths = Paths()
        ActivityStubProtocol.set { request in
            let path = request.url?.path ?? ""
            paths.append(path)
            switch path {
            case "/v1/cron/jobs/cron-1/pause":
                return .init(
                    status: 409,
                    body: Data(#"{"error":{"code":"conflict","message":"already paused"}}"#.utf8))
            case "/v1/tasks": return .init(body: paused)
            default: return .init(body: Data("{}".utf8))
            }
        }

        await project.performTaskAction(.pause, for: task)

        #expect(paths.snapshot.contains("/v1/cron/jobs/cron-1/pause"))
        #expect(paths.snapshot.contains("/v1/tasks"))
        #expect(project.tasks.first?.status == .paused)
        #expect(project.statusMessage == "already paused")
    }

    @Test("scheduled lifecycle labels expose result, timing, run link, and safe error")
    func lifecycleLabelsAreAccessible() throws {
        let task = TaskInfo(
            id: "cron-1", type: .cron, status: .active, label: "watch deploy",
            lastExecutionStatus: .failed, runID: "run-1", lastError: "deployment unavailable")
        let label = TaskLifecycleText.accessibilityLabel(for: task)
        #expect(label.contains("cron"))
        #expect(label.contains("watch deploy"))
        #expect(label.contains("Last result: failed"))
        #expect(label.contains("Run: run-1"))
        #expect(label.contains("Error: deployment unavailable"))
    }

    @Test("opening a linked scheduled task loads its chat and attaches only active run evidence")
    func opensLinkedScheduledTaskWithActiveRun() async throws {
        let project = makeProject()
        ActivityStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/conversations/conv-linked/messages":
                return .init(
                    body: Data(
                        #"{"messages":[{"role":"user","content":"watch deploy","step":0}]}"#.utf8))
            case "/v1/runs/run-linked/events":
                return .init(
                    body: Data(
                        "event: run.started\ndata: {\"id\":\"run-linked:0\",\"run_id\":\"run-linked\",\"type\":\"run.started\",\"payload\":{}}\n\n"
                            .utf8))
            default:
                return .init(body: Data("{}".utf8))
            }
        }
        await project.start()
        let opened = await project.openScheduledTask(
            TaskInfo(
                id: "cron-1", type: .cron, status: .active, label: "watch deploy",
                conversationID: "conv-linked", runID: "run-linked"))
        #expect(opened)
        #expect(project.run?.conversationID == "conv-linked")
        #expect(project.run?.currentRunID == "run-linked")
    }

    @Test("terminal or missing task links never manufacture live run controls")
    func terminalAndMissingTaskLinksDoNotCreateLiveControls() async throws {
        let project = makeProject()
        ActivityStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/conversations/conv-terminal/messages":
                return .init(
                    body: Data(
                        #"{"messages":[{"role":"assistant","content":"done","step":0}]}"#.utf8))
            case "/v1/runs/run-terminal/events":
                return .init(
                    body: Data(
                        "event: run.completed\ndata: {\"id\":\"run-terminal:1\",\"run_id\":\"run-terminal\",\"type\":\"run.completed\",\"payload\":{}}\n\n"
                            .utf8))
            default:
                return .init(body: Data("{}".utf8))
            }
        }
        await project.start()
        let terminalOpened = await project.openScheduledTask(
            TaskInfo(
                id: "cron-1", type: .cron, status: .active, label: "watch deploy",
                conversationID: "conv-terminal", runID: "run-terminal"))
        #expect(terminalOpened)
        #expect(project.run?.conversationID == "conv-terminal")
        #expect(project.run?.currentRunID == nil)

        let missingOpened = await project.openScheduledTask(
            TaskInfo(
                id: "orphan", type: .cron, status: .active, label: "orphan", runID: "run-orphan"))
        #expect(!missingOpened)
        #expect(project.run?.currentRunID == nil)
    }
}
