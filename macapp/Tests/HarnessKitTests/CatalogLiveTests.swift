import Foundation
import Testing

@testable import HarnessKit

private let liveBaseURL: URL? = ProcessInfo.processInfo
    .environment["HARNESS_TEST_BASE_URL"].flatMap(URL.init(string:))

/// Decoding tests for the catalog, session, and config endpoints.
///
/// These run against a real harnessd rather than fixtures because the failure
/// they guard against is a *mismatch with the server* — a wrong `CodingKey` or
/// a missing date strategy decodes fine against a stub and fails in production.
@Suite("catalog and session endpoints (live)", .serialized)
struct CatalogLiveTests {

    private func client() throws -> HarnessClient {
        HarnessClient(baseURL: try #require(liveBaseURL))
    }

    @Test("decodes the model catalog with pricing and modalities", .enabled(if: liveBaseURL != nil))
    func decodesModels() async throws {
        let models = try await client().models()
        // Not a count threshold: live model discovery adds entries only when
        // providers are configured, so a developer machine sees far more models
        // than CI. What must hold is that the catalog is populated and decodes.
        #expect(!models.isEmpty, "empty model catalog")

        let priced = models.filter { $0.inputCostPerMTok != nil }
        #expect(!priced.isEmpty, "no model decoded pricing — check the CodingKeys")
        #expect(priced.first?.priceSummary != nil)

        // Modalities gate image paste, so they must survive decoding.
        #expect(models.contains { $0.modalities?.isEmpty == false })
        #expect(models.allSatisfy { !$0.id.isEmpty && !$0.provider.isEmpty })
    }

    @Test("decodes providers with their auth type", .enabled(if: liveBaseURL != nil))
    func decodesProviders() async throws {
        let providers = try await client().providers()
        #expect(!providers.isEmpty)
        #expect(providers.allSatisfy { !$0.name.isEmpty })
        // Both auth kinds exist in the catalog; the UI branches on this.
        #expect(providers.contains { $0.authType == "api_key" })
    }

    @Test("decodes profiles", .enabled(if: liveBaseURL != nil))
    func decodesProfiles() async throws {
        let profiles = try await client().profiles()
        #expect(profiles.allSatisfy { !$0.name.isEmpty })
    }

    /// Dates are the most fragile part of these payloads: a missing date
    /// strategy fails only once a real timestamp arrives.
    @Test("decodes conversations including their timestamps", .enabled(if: liveBaseURL != nil))
    func decodesConversations() async throws {
        let client = try client()

        // Produce a conversation to list.
        var request = HarnessClient.StartRunRequest(prompt: "list the workspace")
        request.allowFallback = true
        let started = try await client.startRun(request)
        for try await _ in client.events(runID: started.runID) {}

        let conversations = try await client.conversations(limit: 10)
        #expect(!conversations.isEmpty, "expected the run to persist a conversation")

        let conversation = try #require(conversations.first)
        #expect(!conversation.id.isEmpty)
        #expect(conversation.createdAt != nil, "timestamp failed to decode")
        #expect(!conversation.displayTitle.isEmpty)

        let messages = try await client.messages(conversationID: started.runID)
        #expect(!messages.isEmpty)
        #expect(messages.contains { $0.role == "user" })
    }

    @Test("forks a conversation", .enabled(if: liveBaseURL != nil))
    func forksConversation() async throws {
        let client = try client()
        var request = HarnessClient.StartRunRequest(prompt: "list the workspace")
        request.allowFallback = true
        let started = try await client.startRun(request)
        for try await _ in client.events(runID: started.runID) {}

        let fork = try await client.fork(conversationID: started.runID)
        #expect(!fork.conversationID.isEmpty)
        #expect(fork.conversationID != started.runID, "fork must mint a new id")
    }

    @Test("lists rewind points without error", .enabled(if: liveBaseURL != nil))
    func listsRewindPoints() async throws {
        let client = try client()
        var request = HarnessClient.StartRunRequest(prompt: "list the workspace")
        request.allowFallback = true
        let started = try await client.startRun(request)
        for try await _ in client.events(runID: started.runID) {}

        // A read-only run creates no file snapshots; decoding an empty list
        // cleanly is still the contract under test.
        let points = try await client.rewindPoints(conversationID: started.runID)
        #expect(points.allSatisfy { !$0.id.isEmpty })
    }
}
