import Foundation

/// One provider row on the model settings page.
public struct ModelSettingsProvider: Sendable, Decodable, Identifiable, Hashable {
    public var id: String { name }
    public let name: String
    public let baseURL: String
    public let protocolName: String
    public let authKind: String
    public let builtin: Bool
    public let hasCredential: Bool
    /// Where the credential lives — a location, never the secret itself.
    public let keyRef: String?
    public let modelCount: Int
    public let exposedCount: Int
    public let fetchedAt: String?
    public let fetchError: String?
    public let models: [ModelSettingsEntry]?

    enum CodingKeys: String, CodingKey {
        case name, builtin, models
        case baseURL = "base_url"
        case protocolName = "protocol"
        case authKind = "auth_kind"
        case hasCredential = "has_credential"
        case keyRef = "key_ref"
        case modelCount = "model_count"
        case exposedCount = "exposed_count"
        case fetchedAt = "fetched_at"
        case fetchError = "fetch_error"
    }

    /// Subscription providers import a vendor login rather than take a key.
    public var usesSubscription: Bool { authKind == "subscription" }
    /// Local servers need no credential at all.
    public var needsNoCredential: Bool { authKind == "none" }
}

/// One model as fetched from its provider.
public struct ModelSettingsEntry: Sendable, Decodable, Identifiable, Hashable {
    public var id: String { modelID }
    public let modelID: String
    public let displayName: String?
    public let contextWindow: Int?
    public let inputCost: Double?
    public let outputCost: Double?
    /// "provider", "catalog", or "user" — where the price came from.
    public let costSource: String?
    public let modalities: [String]?
    public let exposed: Bool

    enum CodingKeys: String, CodingKey {
        case modelID = "id"
        case displayName = "display_name"
        case contextWindow = "context_window"
        case inputCost = "input_per_1m_tokens_usd"
        case outputCost = "output_per_1m_tokens_usd"
        case costSource = "cost_source"
        case modalities, exposed
    }

    /// Most provider model endpoints report no pricing, so "unknown" is the
    /// common case and must read differently from "free".
    public var priceSummary: String {
        guard let inputCost, let outputCost else { return "cost unknown" }
        return "$\(trimCost(inputCost)) in · $\(trimCost(outputCost)) out per Mtok"
    }

    public var contextSummary: String? {
        guard let contextWindow, contextWindow > 0 else { return nil }
        if contextWindow >= 1_000_000 {
            return "\(contextWindow / 1_000_000)M ctx"
        }
        return "\(contextWindow / 1000)K ctx"
    }
}

private func trimCost(_ value: Double) -> String {
    value == value.rounded()
        ? String(Int(value))
        : String(format: "%g", value)
}

private struct SaveProviderBody: Encodable {
    let name: String
    let baseURL: String
    let protocolName: String
    let authKind: String
    let keyRef: String?
    /// Write-only: stored at the credential location, never read back.
    let apiKey: String?

    enum CodingKeys: String, CodingKey {
        case name
        case baseURL = "base_url"
        case protocolName = "protocol"
        case authKind = "auth_kind"
        case keyRef = "key_ref"
        case apiKey = "api_key"
    }
}

private struct ExposeBody: Encodable { let exposed: [String: Bool] }

private struct CostBody: Encodable {
    let model: String
    let input: Double
    let output: Double

    enum CodingKeys: String, CodingKey {
        case model
        case input = "input_per_1m_tokens_usd"
        case output = "output_per_1m_tokens_usd"
    }
}

private struct EmptyBody: Encodable {}

private struct OKResponse: Decodable { let ok: Bool }
private struct FetchResponse: Decodable {
    let modelCount: Int
    enum CodingKeys: String, CodingKey { case modelCount = "model_count" }
}

extension HarnessClient {
    /// Everything the settings page needs, including models not yet exposed.
    public func modelSettings() async throws -> [ModelSettingsProvider] {
        struct Response: Decodable { let providers: [ModelSettingsProvider] }
        let response: Response = try await get("/v1/model-settings")
        return response.providers
    }

    /// Adds or updates a provider. `apiKey` is stored at the credential
    /// location and is never returned by any later read.
    public func saveProvider(
        name: String,
        baseURL: String,
        protocolName: String,
        authKind: String,
        keyRef: String?,
        apiKey: String?
    ) async throws {
        let body = SaveProviderBody(
            name: name,
            baseURL: baseURL,
            protocolName: protocolName,
            authKind: authKind,
            keyRef: (keyRef?.isEmpty ?? true) ? nil : keyRef,
            apiKey: (apiKey?.isEmpty ?? true) ? nil : apiKey
        )
        try await sendVoid(.post, "/v1/model-settings/providers", body: body)
    }

    public func deleteProvider(name: String) async throws {
        try await sendVoid(.delete, providerPath(name))
    }

    /// Pulls the provider's current model list from the provider itself.
    @discardableResult
    public func fetchProviderModels(name: String) async throws -> Int {
        let response: FetchResponse = try await send(
            .post, providerPath(name) + "/fetch", body: EmptyBody())
        return response.modelCount
    }

    /// Records which models the picker should offer.
    public func setExposedModels(provider: String, exposed: [String: Bool]) async throws {
        try await sendVoid(
            .post, providerPath(provider) + "/expose", body: ExposeBody(exposed: exposed))
    }

    /// Records a hand-entered price, which later fetches will not overwrite.
    public func setModelCost(
        provider: String, model: String, input: Double, output: Double
    ) async throws {
        try await sendVoid(
            .post, providerPath(provider) + "/cost",
            body: CostBody(
                model: model,
                input: input,
                output: output))
    }

    private func providerPath(_ name: String) -> String {
        let escaped =
            name.addingPercentEncoding(
                withAllowedCharacters: .urlPathAllowed) ?? name
        return "/v1/model-settings/providers/\(escaped)"
    }
}
