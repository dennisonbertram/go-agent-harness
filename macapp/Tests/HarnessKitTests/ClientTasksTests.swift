import Foundation
import Testing

@testable import HarnessKit

/// Isolated transport stub for this suite. `StubURLProtocol` is intentionally
/// process-global for the older HarnessClient suite, so reusing it here would
/// let independent Swift Testing suites overwrite each other's handler.
private final class TaskStubURLProtocol: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body = Data()
    }

    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    nonisolated(unsafe) private static var recorded: [URLRequest] = []
    nonisolated(unsafe) private static var recordedBodies: [Data?] = []
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock {
            self.handler = handler
            self.recorded = []
            self.recordedBodies = []
        }
    }

    static var requests: [URLRequest] { lock.withLock { recorded } }
    static var bodies: [Data?] { lock.withLock { recordedBodies } }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        Self.lock.withLock {
            Self.recorded.append(request)
            Self.recordedBodies.append(Self.bodyData(request))
        }
        let response = Self.lock.withLock { Self.handler }?(request) ?? Response()
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}

    private static func bodyData(_ request: URLRequest) -> Data? {
        if let body = request.httpBody { return body }
        guard let stream = request.httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        while stream.hasBytesAvailable {
            let read = stream.read(&buffer, maxLength: buffer.count)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

@Suite("HarnessClient task lifecycle", .serialized)
struct ClientTasksTests {

    private func makeClient() -> HarnessClient {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [TaskStubURLProtocol.self]
        return HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:8899")!,
            session: URLSession(configuration: configuration))
    }

    @Test("decodes additive lifecycle fields and preserves unknown task values")
    func decodesLifecycleAndUnknownValues() async throws {
        TaskStubURLProtocol.set { request in
            #expect(request.httpMethod == "GET")
            #expect(request.url?.path == "/v1/tasks")
            return .init(
                body: Data(
                    #"""
                    {"tasks":[{
                      "id":"cron-1","type":"cron","status":"active","label":"watch deploy",
                      "started_at":"2026-08-04T12:00:00Z","age_seconds":1,
                      "actions":["pause","delete","future_action"],
                      "conversation_id":"conv-1","next_run_at":"2026-08-04T12:05:00Z",
                      "last_run_at":"2026-08-04T11:55:00Z","last_execution_status":"completed",
                      "run_id":"run-1","updated_at":"2026-08-04T12:00:00.123456789Z"
                    },{
                      "id":"future-1","type":"future_kind","status":"future_state","label":"future",
                      "started_at":"2026-08-04T12:00:00Z","age_seconds":0,"actions":["future_action"]
                    }]}
                    """#.utf8))
        }

        let tasks = try await makeClient().tasks()
        let cron = try #require(tasks.first)
        #expect(cron.type == .cron)
        #expect(cron.status == .active)
        #expect(cron.conversationID == "conv-1")
        #expect(cron.nextRunAt != nil)
        #expect(cron.lastRunAt != nil)
        #expect(cron.lastExecutionStatus == .completed)
        #expect(cron.runID == "run-1")
        #expect(cron.updatedAtVersion == "2026-08-04T12:00:00.123456789Z")
        #expect(cron.actions?.contains(.pause) == true)
        #expect(cron.actions?.contains(.unknown("future_action")) == true)
        #expect(tasks[1].type == .unknown("future_kind"))
        #expect(tasks[1].status == .unknown("future_state"))
    }

    @Test("uses scoped task-control endpoints with the row version when present")
    func sendsTaskControlRequests() async throws {
        TaskStubURLProtocol.set { request in
            #expect(request.httpMethod == "POST" || request.httpMethod == "DELETE")
            return .init(status: request.httpMethod == "DELETE" ? 204 : 200, body: Data("{}".utf8))
        }
        let client = makeClient()
        let updatedAt = "2026-08-04T12:00:00.123456789Z"
        try await client.pauseCron(id: "cron-1", expectedUpdatedAt: updatedAt)
        try await client.resumeCron(id: "cron-1", expectedUpdatedAt: updatedAt)
        try await client.deleteCron(id: "cron-1", expectedUpdatedAt: updatedAt)
        try await client.cancelCallback(id: "callback-1")

        #expect(
            TaskStubURLProtocol.requests.map(\.url?.path) == [
                "/v1/cron/jobs/cron-1/pause", "/v1/cron/jobs/cron-1/resume",
                "/v1/cron/jobs/cron-1", "/v1/callbacks/callback-1/cancel",
            ])
        for body in TaskStubURLProtocol.bodies.prefix(3) {
            let body = try #require(body)
            let payload = try #require(
                JSONSerialization.jsonObject(with: body) as? [String: String])
            #expect(payload["expected_updated_at"] == updatedAt)
        }
    }

    @Test("preserves empty action bodies when no task version is available")
    func preservesEmptyActionBodiesWithoutVersion() async throws {
        TaskStubURLProtocol.set { _ in .init(body: Data("{}".utf8)) }
        try await makeClient().pauseCron(id: "cron-1")
        #expect(TaskStubURLProtocol.bodies == [nil])
    }
}
