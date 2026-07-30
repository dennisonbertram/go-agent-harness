import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP stub for exercising `ProjectSession`'s load-state surfaces
/// without a live harnessd. Mirrors `ActivityStubProtocol` in
/// `ProjectSessionActivityTests.swift` (that stub is `private` to its own
/// file, so it cannot be reused directly) — scoped to its own fixed loopback
/// port so it never intercepts another suite's traffic.
private final class LoadStateStubProtocol: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body: Data = Data()
    }

    static let port = 18913
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

/// `.failed` used to carry no message, so an inline error next to a failed
/// list could not truthfully name *that* list's failure — it could only fall
/// back to the single-slot `statusMessage` shared by every collection (#991
/// finding 2). These prove `CollectionLoadState.failed` on `ProjectSession`
/// carries its own per-collection reason, that a retry actually recovers,
/// and that one collection's failure never blanks another's data.
@Suite("ProjectSession load-state messages", .serialized)
@MainActor
struct ProjectSessionLoadStateTests {

    private static let baseURL = URL(string: "http://127.0.0.1:\(LoadStateStubProtocol.port)")!

    private func makeProject() -> ProjectSession {
        URLProtocol.registerClass(LoadStateStubProtocol.self)
        return ProjectSession(
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()),
            externalBaseURL: Self.baseURL)
    }

    private final class Box: @unchecked Sendable {
        private let lock = NSLock()
        private var value: Int
        init(_ value: Int) { self.value = value }
        var current: Int {
            get { lock.withLock { value } }
            set { lock.withLock { value = newValue } }
        }
    }

    @Test("a failed conversations refresh carries the server's own message")
    func failedRefreshCarriesServerMessage() async throws {
        let project = makeProject()
        LoadStateStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/conversations/":
                return .init(
                    status: 500,
                    body: Data(
                        #"{"error":{"code":"boom","message":"conversations exploded"}}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()
        await project.refreshConversations()

        #expect(project.conversationsLoadState.errorMessage?.contains("conversations exploded") == true)
        #expect(project.conversations.isEmpty)
    }

    @Test("retrying a failed conversations refresh recovers to loaded with rows")
    func retryAfterFailureRecovers() async throws {
        let project = makeProject()
        let conversationsStatus = Box(500)
        LoadStateStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/conversations/":
                switch conversationsStatus.current {
                case 200:
                    return .init(
                        status: 200,
                        body: Data(#"{"conversations":[{"id":"c1"}]}"#.utf8))
                default:
                    return .init(
                        status: 500,
                        body: Data(
                            #"{"error":{"code":"boom","message":"conversations exploded"}}"#.utf8
                        ))
                }
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()
        await project.refreshConversations()
        #expect(project.conversationsLoadState.showsError)
        #expect(project.conversations.isEmpty)

        // Retry pressed: same call, now the daemon is healthy.
        conversationsStatus.current = 200
        await project.refreshConversations()
        #expect(project.conversationsLoadState == .loaded)
        #expect(project.conversations.count == 1)
    }

    @Test("a failed refresh after a successful one keeps the previously loaded rows")
    func failedRefreshAfterSuccessKeepsRows() async throws {
        let project = makeProject()
        let conversationsStatus = Box(200)
        LoadStateStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/conversations/":
                switch conversationsStatus.current {
                case 200:
                    return .init(
                        status: 200,
                        body: Data(#"{"conversations":[{"id":"c1"}]}"#.utf8))
                default:
                    return .init(
                        status: 500,
                        body: Data(
                            #"{"error":{"code":"boom","message":"conversations exploded"}}"#.utf8
                        ))
                }
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()
        await project.refreshConversations()
        #expect(project.conversations.count == 1)

        conversationsStatus.current = 500
        await project.refreshConversations()
        #expect(project.conversationsLoadState.showsError)
        #expect(
            project.conversations.count == 1,
            "a failed refresh must not blank rows the prior successful refresh loaded")
    }

    @Test("one collection's failure carries its own message without affecting a healthy sibling")
    func perCollectionIsolation() async throws {
        let project = makeProject()
        LoadStateStubProtocol.set { request in
            switch request.url?.path {
            case "/v1/models":
                return .init(
                    status: 500,
                    body: Data(#"{"error":{"code":"boom","message":"models exploded"}}"#.utf8))
            case "/v1/providers":
                return .init(
                    status: 200,
                    body: Data(#"{"providers":[{"name":"openai","configured":true}]}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.start()
        await project.refreshCatalog()

        #expect(project.modelsLoadState.errorMessage?.contains("models exploded") == true)
        #expect(project.providersLoadState == .loaded)
    }
}

/// Distinct from the state-table tests in `CollectionLoadStateTests`: those
/// prove the type's own logic; this proves the failure UI it enables has a
/// real production call site rather than being a component nobody uses.
@Suite("CollectionErrorState reachability")
struct CollectionErrorStateReachabilityTests {

    @Test("CollectionErrorState has production call sites")
    func hasProductionCallSites() throws {
        let sourceDirectory = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
        let source = try FileManager.default
            .contentsOfDirectory(at: sourceDirectory, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "swift" }
            .map { try String(contentsOf: $0, encoding: .utf8) }
            .joined(separator: "\n")

        #expect(source.contains("CollectionErrorState("))
    }
}
