import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP stub scoped to this file's tests, keyed on HTTP method and
/// path. `ModelSettingsModel` takes a `HarnessClient` directly (no
/// `ProjectSession` in between), so the stub is wired through the client's
/// injectable `URLSessionConfiguration` rather than a global `URLProtocol`
/// registration -- the same shape `RunControlAckTests` uses.
private final class ModelSettingsStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var body: Data = Data()
    }

    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
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

/// Exercises the fix for #999 (F8, R10): `ModelSettingsModel.load()`
/// unconditionally set `status = nil` on success, which erased the message
/// `fetch`/`setExposed`/etc. had just set two lines earlier -- the reload
/// every action already triggers threw its own result away before the
/// operator could read it.
@Suite("ModelSettingsModel status feedback", .serialized)
@MainActor
struct ModelSettingsFeedbackTests {

    private let providerJSON = """
        {"providers":[{"name":"openai","base_url":"https://api.openai.com","protocol":"openai_compat","auth_kind":"api_key","builtin":true,"has_credential":true,"key_ref":null,"model_count":1,"exposed_count":1,"fetched_at":null,"fetch_error":null,"models":null}]}
        """

    private func makeModel() -> ModelSettingsModel {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ModelSettingsStub.self]
        let client = HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:18899")!,
            session: URLSession(configuration: config))
        return ModelSettingsModel(client: client)
    }

    @Test("a successful fetch's status survives the reload that follows it -- core regression")
    func fetchStatusSurvivesReload() async throws {
        let providerJSON = providerJSON
        ModelSettingsStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/v1/model-settings"):
                return .init(status: 200, body: Data(providerJSON.utf8))
            case ("POST", "/v1/model-settings/providers/openai/fetch"):
                return .init(status: 200, body: Data(#"{"model_count":3}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        let model = makeModel()

        await model.fetch("openai")

        #expect(model.status?.contains("Fetched") == true)
        #expect(model.loadState == .loaded)
    }

    @Test("a failed fetch's reason survives the reload that follows it")
    func fetchFailureStatusSurvivesReload() async throws {
        let providerJSON = providerJSON
        ModelSettingsStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/v1/model-settings"):
                return .init(status: 200, body: Data(providerJSON.utf8))
            case ("POST", "/v1/model-settings/providers/openai/fetch"):
                return .init(
                    status: 500,
                    body: Data(#"{"error":{"code":"boom","message":"bad key"}}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        let model = makeModel()

        await model.fetch("openai")

        #expect(model.status?.contains("bad key") == true)
    }

    @Test("the initial load has no prior status to clear, and lands loaded")
    func initialLoadStartsClean() async throws {
        let providerJSON = providerJSON
        ModelSettingsStub.set { request in
            .init(status: 200, body: Data(providerJSON.utf8))
        }
        let model = makeModel()

        await model.load()

        #expect(model.status == nil)
        #expect(model.loadState == .loaded)
    }

    @Test("a failed setExposed's reason survives the reload that follows it")
    func setExposedFailureStatusSurvivesReload() async throws {
        let providerJSON = providerJSON
        ModelSettingsStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/v1/model-settings"):
                return .init(status: 200, body: Data(providerJSON.utf8))
            case ("POST", "/v1/model-settings/providers/openai/expose"):
                return .init(
                    status: 500,
                    body: Data(#"{"error":{"code":"boom","message":"could not save"}}"#.utf8))
            default:
                return .init(status: 200, body: Data("{}".utf8))
            }
        }
        let model = makeModel()

        await model.setExposed("openai", "gpt-5", true)

        #expect(model.status?.contains("could not save") == true)
    }

    /// `clearingStatus` is the escape hatch for the initial `.task` load,
    /// which must still clear a stale status rather than leave a past
    /// failure looking permanent.
    @Test("load(clearingStatus: true) clears a stale status on success")
    func clearingStatusTrueClearsStaleStatus() async throws {
        let providerJSON = providerJSON
        ModelSettingsStub.set { request in
            .init(status: 200, body: Data(providerJSON.utf8))
        }
        let model = makeModel()
        model.status = "some stale message"

        await model.load(clearingStatus: true)

        #expect(model.status == nil)
    }
}
