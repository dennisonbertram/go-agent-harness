import Foundation

/// A model the server can route to. Field names mirror the live
/// `GET /v1/models` payload.
public struct ModelInfo: Sendable, Decodable, Identifiable, Hashable {
    public let id: String
    public let provider: String
    public let aliases: [String]?
    public let inputCostPerMTok: Double?
    public let outputCostPerMTok: Double?
    public let modalities: [String]?

    enum CodingKeys: String, CodingKey {
        case id, provider, aliases, modalities
        case inputCostPerMTok = "input_cost_per_mtok"
        case outputCostPerMTok = "output_cost_per_mtok"
    }

    /// The TUI's model picker shows neither price nor modality; both drive real
    /// choices, so the native picker surfaces them.
    public var supportsImages: Bool { modalities?.contains("image") ?? false }

    public var priceSummary: String? {
        guard let input = inputCostPerMTok, let output = outputCostPerMTok else { return nil }
        return "$\(trim(input)) in · $\(trim(output)) out per Mtok"
    }

    private func trim(_ value: Double) -> String {
        value == value.rounded() ? String(Int(value)) : String(format: "%.2f", value)
    }
}

public struct ProviderInfo: Sendable, Decodable, Identifiable, Hashable {
    public var id: String { name }
    public let name: String
    public let configured: Bool
    public let apiKeyEnv: String?
    public let authType: String?
    public let baseURL: String?
    public let modelCount: Int?

    enum CodingKeys: String, CodingKey {
        case name, configured
        case apiKeyEnv = "api_key_env"
        case authType = "auth_type"
        case baseURL = "base_url"
        case modelCount = "model_count"
    }

    /// Only subscription providers can import a vendor CLI credential.
    public var supportsSubscriptionImport: Bool { authType == "subscription" }
}

public struct ProfileInfo: Sendable, Decodable, Identifiable, Hashable {
    public var id: String { name }
    public let name: String
    public let description: String?
    public let model: String?
    public let allowedToolCount: Int?
    public let sourceTier: String?

    enum CodingKeys: String, CodingKey {
        case name, description, model
        case allowedToolCount = "allowed_tool_count"
        case sourceTier = "source_tier"
    }
}

extension HarnessClient {
    public func models() async throws -> [ModelInfo] {
        struct Response: Decodable { let models: [ModelInfo]? }
        let response: Response = try await get("/v1/models")
        return response.models ?? []
    }

    public func providers() async throws -> [ProviderInfo] {
        struct Response: Decodable { let providers: [ProviderInfo]? }
        let response: Response = try await get("/v1/providers")
        return response.providers ?? []
    }

    public func profiles() async throws -> [ProfileInfo] {
        struct Response: Decodable { let profiles: [ProfileInfo]? }
        let response: Response = try await get("/v1/profiles")
        return response.profiles ?? []
    }

    /// Sets a provider's API key. The value is held in the server's memory only.
    public func setProviderKey(provider: String, key: String) async throws {
        struct Body: Encodable { let key: String }
        try await sendVoid(.put, "/v1/providers/\(provider)/key", body: Body(key: key))
    }

    /// Imports a vendor CLI subscription credential.
    ///
    /// The request carries no credential: harnessd reads its **own host's**
    /// vendor files, so this cannot transfer a local login to a remote daemon.
    public func importSubscription(provider: String) async throws {
        try await sendVoid(.post, "/v1/providers/\(provider)/import-subscription")
    }
}
