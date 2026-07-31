import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP stub scoped to this file's tests, mirroring
/// `LifecycleGuardStub` (`ProjectSessionLifecycleGuardTests.swift`): it
/// records every request (and, unlike that stub, the request body) on a
/// fixed loopback port so it never intercepts another suite's traffic and so
/// these tests can assert the second, forced request actually carries
/// `"force":true` rather than merely trusting the client's own unit tests.
private final class RewindStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body: Data = Data()
        var completionGate: DispatchSemaphore?
    }

    static let port = 18917
    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    nonisolated(unsafe) private static var recordedBodies: [String: [Data]] = [:]
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    static func reset() {
        lock.withLock {
            handler = nil
            recordedBodies = [:]
        }
    }

    static func bodies(matching path: String) -> [Data] {
        lock.withLock { recordedBodies[path] ?? [] }
    }

    override class func canInit(with request: URLRequest) -> Bool {
        request.url?.port == port
    }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        // The handler itself may call `bodies(matching:)` (to count prior
        // attempts) -- invoke it after releasing the lock, or a handler doing
        // so would deadlock re-entering this file's non-reentrant `NSLock`.
        let handler = Self.lock.withLock { () -> (@Sendable (URLRequest) -> Response)? in
            if let path = request.url?.path {
                Self.recordedBodies[path, default: []].append(request.httpBodyData ?? Data())
            }
            return Self.handler
        }
        let response = handler?(request) ?? Response()
        if let gate = response.completionGate {
            DispatchQueue.global().async { [self] in
                gate.wait()
                finishLoading(response)
            }
        } else {
            finishLoading(response)
        }
    }
    override func stopLoading() {}

    private func finishLoading(_ response: Response) {
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        client?.urlProtocolDidFinishLoading(self)
    }
}

/// A `409 rewind_refused` envelope, matching the server's actual wire shape
/// (`internal/server/http_conversations.go:370`). A free function, not a
/// method on the `@MainActor` test suite, so it can be called from the
/// stub's non-isolated `@Sendable` handler closures without hopping actors.
private let rewindPath = "/v1/conversations/conv_1/rewind"

private func refused(
    message: String, completionGate: DispatchSemaphore? = nil
) -> RewindStub.Response {
    .init(
        status: 409,
        body: Data(#"{"error":{"code":"rewind_refused","message":"\#(message)"}}"#.utf8),
        completionGate: completionGate)
}

extension URLRequest {
    /// `URLProtocol` may strip `httpBody` onto a stream; read either.
    fileprivate var httpBodyData: Data? {
        if let body = httpBody { return body }
        guard let stream = httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        let size = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: size)
        defer { buffer.deallocate() }
        while stream.hasBytesAvailable {
            let read = stream.read(buffer, maxLength: size)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

/// Exercises the fix for #997 (F6, R7, KTD-6): `rewind` used to collapse
/// every `HarnessError` -- including the server's deliberate
/// `409 rewind_refused` safety refusal -- into `statusMessage` prose, so no
/// distinct "restore anyway" path could exist. `rewind` now branches on
/// `HarnessError.code == "rewind_refused"` (not the HTTP status, which is
/// merely the transport for it) and records a structural `RewindRefusal`
/// instead.
@Suite("ProjectSession rewind refusal", .serialized)
@MainActor
struct ProjectSessionRewindTests {

    private static let baseURL = URL(string: "http://127.0.0.1:\(RewindStub.port)")!

    private func makeProject() -> ProjectSession {
        URLProtocol.registerClass(RewindStub.self)
        return ProjectSession(
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()),
            externalBaseURL: Self.baseURL)
    }

    private func makeReadyProject() async -> ProjectSession {
        let project = makeProject()
        await project.start()
        project.run?.rebind(conversationID: "conv_1")
        return project
    }

    private func makePoint(id: String = "point_1") throws -> RewindPoint {
        try JSONDecoder().decode(RewindPoint.self, from: Data(#"{"id":"\#(id)"}"#.utf8))
    }

    private func wait(
        timeout: Duration = .seconds(3), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("timed out waiting for rewind request")
    }

    // MARK: - Behavioral

    @Test(
        "a rewind_refused refusal is captured structurally, matched on HarnessError.code -- core regression"
    )
    func refusalCapturedStructurally() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return refused(message: "README.md changed outside the harness")
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)

        #expect(project.rewindRefusal?.pointID == point.id)
        #expect(
            project.rewindRefusal?.message.contains("README.md changed outside the harness")
                == true)
        #expect(project.rewindRefusal?.conversationID == "conv_1")
    }

    @Test("a rewind refusal that completes after conversation switch is discarded")
    func refusalValidatesConversationBeforePresentation() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        let responseGate = DispatchSemaphore(value: 0)
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return refused(
                    message: "README.md changed outside the harness",
                    completionGate: responseGate)
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        let pending = Task { await project.rewind(to: point) }
        try await wait { RewindStub.bodies(matching: rewindPath).count == 1 }
        project.run?.rebind(conversationID: "conv_2")
        responseGate.signal()
        await pending.value

        #expect(project.rewindRefusal == nil)
    }

    @Test("a captured refusal cannot force-rewind a newly selected conversation")
    func forceRetryValidatesRefusalConversation() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return refused(message: "README.md changed outside the harness")
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)
        let refusal = try #require(project.rewindRefusal)
        let requestsBeforeSwitch = RewindStub.bodies(matching: rewindPath).count
        project.run?.rebind(conversationID: "conv_2")

        await project.forceRewind(refusal)

        #expect(RewindStub.bodies(matching: rewindPath).count == requestsBeforeSwitch)
        #expect(project.rewindRefusal == nil)
    }

    @Test("a generic failure sets statusMessage and offers no force path")
    func genericFailureDoesNotOfferForce() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return .init(
                    status: 500,
                    body: Data(#"{"error":{"code":"internal_error","message":"boom"}}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)

        #expect(project.rewindRefusal == nil)
        #expect(project.statusMessage == "boom")
    }

    @Test(
        "confirming the refusal sends force:true, and the refusal clears once the forced call succeeds"
    )
    func forceRewindSendsForceTrueAndClears() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                let attempt = RewindStub.bodies(matching: rewindPath).count
                if attempt <= 1 {
                    return refused(message: "README.md changed outside the harness")
                }
                return .init(
                    status: 200,
                    body: Data(#"{"files_restored":2,"messages_truncated":3}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)
        #expect(project.rewindRefusal != nil)

        await project.rewind(to: point, force: true)

        let bodies = RewindStub.bodies(matching: rewindPath)
        #expect(bodies.count == 2)
        let secondBody = try #require(bodies.last)
        let decoded = try JSONSerialization.jsonObject(with: secondBody) as? [String: Any]
        #expect(decoded?["force"] as? Bool == true)
        #expect(project.rewindRefusal == nil)
        #expect(project.statusMessage == "Restored 2 file(s), removed 3 message(s)")
    }

    @Test(
        "a second refusal on the forced call sets rewindRefusal again rather than looping or clearing silently"
    )
    func secondRefusalOnForcedCallSetsAgain() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return refused(message: "still changed outside the harness")
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)
        #expect(project.rewindRefusal != nil)

        await project.rewind(to: point, force: true)

        #expect(project.rewindRefusal?.pointID == point.id)
        #expect(
            project.rewindRefusal?.message.contains("still changed outside the harness") == true)
    }

    @Test(
        "a successful rewind reports the restore counts with no refusal -- existing behaviour, pinned"
    )
    func successfulRewindReportsCounts() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return .init(
                    status: 200,
                    body: Data(#"{"files_restored":4,"messages_truncated":1}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)

        #expect(project.rewindRefusal == nil)
        #expect(project.statusMessage == "Restored 4 file(s), removed 1 message(s)")
    }

    // MARK: - Regression

    /// Distinct from the behavioral tests above, which only exercise the
    /// clear-on-a-fresh-call and clear-on-success paths: this pins that
    /// declining the confirmation (`dismissRewindRefusal()`, the binding's
    /// Cancel path in `SessionsView`) clears the refusal on its own, with no
    /// server call at all. "Never auto-retried with force" would still read
    /// true if Cancel silently issued `rewind(force: false)` instead of doing
    /// nothing, and only asserting on the request count -- not merely that
    /// `rewindRefusal` became nil -- catches that.
    @Test("dismissing a refusal clears it without contacting the server")
    func dismissingRefusalContactsNoServer() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let point = try makePoint()
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return refused(message: "README.md changed outside the harness")
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }

        await project.rewind(to: point)
        #expect(project.rewindRefusal != nil)
        let requestsBeforeDismiss = RewindStub.bodies(matching: rewindPath).count

        project.dismissRewindRefusal()

        #expect(project.rewindRefusal == nil)
        #expect(RewindStub.bodies(matching: rewindPath).count == requestsBeforeDismiss)
    }

    /// Distinct regression angle from `refusalCapturedStructurally` and
    /// `successfulRewindReportsCounts` above, which both only ever act on a
    /// single point: this proves the guarantee is "cleared at the start of
    /// every call", not merely "cleared on success" -- a stale refusal for a
    /// prior checkpoint must not still read as current once the operator
    /// moves on to a different one, even when the new attempt itself fails
    /// for an unrelated reason.
    @Test(
        "attempting a rewind on a different checkpoint clears a stale refusal from a prior one"
    )
    func newRewindAttemptClearsStaleRefusalFromPriorPoint() async throws {
        RewindStub.reset()
        let project = await makeReadyProject()
        let pointA = try makePoint(id: "point_a")
        let pointB = try makePoint(id: "point_b")
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return refused(message: "point A changed outside the harness")
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        await project.rewind(to: pointA)
        #expect(project.rewindRefusal?.pointID == pointA.id)

        // A fresh attempt on a *different* point fails for an unrelated
        // reason. The stale point-A refusal must not survive into this call.
        RewindStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", rewindPath):
                return .init(
                    status: 500,
                    body: Data(#"{"error":{"code":"internal_error","message":"boom"}}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        await project.rewind(to: pointB)

        #expect(project.rewindRefusal == nil)
        #expect(project.statusMessage == "boom")
    }

    /// Distinct from `sessionsViewWiresForceOnlyInRefusalBranch` below, which
    /// only pins that the force call and the stale NOTE are handled -- this
    /// pins the specific decline-path symbol by name, so a revert that leaves
    /// the second confirmation presented but wires its Cancel button to
    /// nothing (a refusal that can never be dismissed) is caught, not just a
    /// revert of the force call itself.
    @Test("the force-rewind confirmation's decline path calls dismissRewindRefusal")
    func declinePathCallsDismissRewindRefusal() throws {
        let contents = try fileContents("SessionsView.swift")
        #expect(contents.contains("project.dismissRewindRefusal()"))
    }

    // MARK: - Reachability (KTD-6 wiring)

    /// #951 finding 9's NOTE said a force path could not exist without this
    /// unit's model-level change; that NOTE must be retired along with the
    /// dead-toggle prose it explained, and the production force call must
    /// exist only inside the refusal-confirmation branch -- never as a
    /// second, independent call site that could auto-retry with force.
    @Test(
        "SessionsView retries only through the conversation-bound refusal, and the stale NOTE is gone"
    )
    func sessionsViewWiresForceOnlyInRefusalBranch() throws {
        let contents = try fileContents("SessionsView.swift")
        #expect(!contents.contains("finding 9"))
        #expect(!contents.contains("forceNext"))
        #expect(occurrences(of: "forceRewind(", in: contents) == 1)
        #expect(occurrences(of: "rewind(to:", in: contents) == 1)
    }

    // MARK: - Helpers

    private func occurrences(of needle: String, in haystack: String) -> Int {
        haystack.components(separatedBy: needle).count - 1
    }

    private func fileContents(_ name: String) throws -> String {
        let url = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
            .appending(path: name)
        return try String(contentsOf: url, encoding: .utf8)
    }
}
