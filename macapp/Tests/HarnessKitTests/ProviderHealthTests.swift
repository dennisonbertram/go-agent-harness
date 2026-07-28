import Foundation
import Testing

@testable import HarnessKit

@Suite("Provider usability")
struct ProviderHealthTests {

    private func decode(_ json: String) throws -> ProviderInfo {
        try JSONDecoder().decode(ProviderInfo.self, from: Data(json.utf8))
    }

    // The case that started this: a subscription token that expired overnight
    // still reports configured:true. Only `health` tells it apart, and the
    // picker must not offer its models.
    @Test("a configured provider whose credential failed is not usable")
    func failedHealthIsNotUsable() throws {
        let provider = try decode(
            #"{"name":"kimi-subscription","configured":true,"health":"failed","health_error":"Kimi token refresh returned HTTP 401"}"#
        )
        #expect(provider.configured)
        #expect(!provider.isUsable)
        #expect(provider.healthError?.contains("401") == true)
    }

    @Test("an unconfigured provider is not usable")
    func unconfiguredIsNotUsable() throws {
        let provider = try decode(
            #"{"name":"deepseek","configured":false,"health":"unconfigured"}"#)
        #expect(!provider.isUsable)
    }

    @Test("a working credential is usable")
    func okIsUsable() throws {
        let provider = try decode(#"{"name":"codex-subscription","configured":true,"health":"ok"}"#)
        #expect(provider.isUsable)
    }

    // An API key cannot be checked without spending a request. Treating
    // unproven as unusable would empty the picker on a fresh install.
    @Test("an unverified credential stays usable")
    func unverifiedIsUsable() throws {
        let provider = try decode(#"{"name":"openai","configured":true,"health":"unverified"}"#)
        #expect(provider.isUsable)
    }

    // Older daemons do not send the field at all; they must not go dark.
    @Test("a missing health field falls back to configured")
    func missingHealthFallsBack() throws {
        let provider = try decode(#"{"name":"openai","configured":true}"#)
        #expect(provider.health == nil)
        #expect(provider.isUsable)
    }
}
