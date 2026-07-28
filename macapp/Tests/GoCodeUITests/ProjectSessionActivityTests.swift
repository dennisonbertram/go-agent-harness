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
                        body: Data(#"{"error":{"code":"not_configured","message":"no store"}}"#.utf8))
                default:
                    return .init(
                        status: 500, body: Data(#"{"error":{"code":"boom","message":"boom"}}"#.utf8))
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
}
