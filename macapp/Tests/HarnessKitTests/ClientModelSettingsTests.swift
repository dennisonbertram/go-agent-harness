import Foundation
import Testing

@testable import HarnessKit

/// Regression coverage for finding 13: `fetchedAt` used to be `String?` while
/// every other timestamp in this codebase decodes RFC3339 through the shared
/// decoder. If `fetchedAt` ever regressed back to a plain string (or the
/// custom date strategy stopped applying to it), this test fails.
@Suite("ModelSettingsProvider")
struct ClientModelSettingsTests {

    private func provider(json: String) throws -> ModelSettingsProvider {
        try HarnessClient.decoder.decode(ModelSettingsProvider.self, from: Data(json.utf8))
    }

    @Test("fetchedAt decodes an RFC3339 timestamp into a real Date")
    func fetchedAtDecodesToDate() throws {
        let decoded = try provider(
            json: """
                {"name":"openai","base_url":"https://api.openai.com","protocol":"openai",
                 "auth_kind":"api_key","builtin":true,"has_credential":true,"key_ref":null,
                 "model_count":3,"exposed_count":1,"fetched_at":"2026-07-27T12:00:00Z",
                 "fetch_error":null,"models":null}
                """)
        let fetchedAt = try #require(decoded.fetchedAt)
        #expect(abs(fetchedAt.timeIntervalSince1970 - 1_785_153_600) < 1)
    }

    @Test("fetchedAt stays nil when the provider has never fetched")
    func fetchedAtNilWhenAbsent() throws {
        let decoded = try provider(
            json: """
                {"name":"openai","base_url":"https://api.openai.com","protocol":"openai",
                 "auth_kind":"api_key","builtin":true,"has_credential":true,"key_ref":null,
                 "model_count":0,"exposed_count":0,"fetched_at":null,
                 "fetch_error":null,"models":null}
                """)
        #expect(decoded.fetchedAt == nil)
    }
}
