import HarnessKit
import SwiftUI

/// Settings → Models.
///
/// Replaces the read-only model list with the loop the picker actually needs:
/// add a provider, pull its real model list from the provider itself, and tick
/// which of those models the picker offers. Nothing is offered until it is
/// ticked — that is what keeps a 341-model provider out of the menu.
@MainActor
@Observable
final class ModelSettingsModel {
    var providers: [ModelSettingsProvider] = []
    var selectedProvider: String?
    var search = ""
    var status: String?
    var busy = false

    private let client: HarnessClient
    /// Called after anything that changes what the picker should offer, so the
    /// model chip reflects the new selection without needing a relaunch.
    private let onSelectionChanged: () async -> Void

    init(client: HarnessClient, onSelectionChanged: @escaping () async -> Void = {}) {
        self.client = client
        self.onSelectionChanged = onSelectionChanged
    }

    var current: ModelSettingsProvider? {
        providers.first { $0.name == selectedProvider } ?? providers.first
    }

    var visibleModels: [ModelSettingsEntry] {
        let all = current?.models ?? []
        guard !search.isEmpty else { return all }
        return all.filter { $0.modelID.localizedCaseInsensitiveContains(search) }
    }

    func load() async {
        do {
            providers = try await client.modelSettings()
            if selectedProvider == nil { selectedProvider = providers.first?.name }
            status = nil
        } catch {
            status = "Could not load model settings: \(error.localizedDescription)"
        }
    }

    func fetch(_ provider: String) async {
        busy = true
        defer { busy = false }
        do {
            let count = try await client.fetchProviderModels(name: provider)
            status = "Fetched \(count) models from \(provider)."
            await load()
            await onSelectionChanged()
        } catch {
            // The provider's own reason is the useful part — a bad key, an
            // unreachable host — so it is shown verbatim rather than summarised.
            status = "Fetch failed: \(error.localizedDescription)"
            await load()
        }
    }

    func setExposed(_ provider: String, _ model: String, _ exposed: Bool) async {
        do {
            try await client.setExposedModels(provider: provider, exposed: [model: exposed])
            await load()
            await onSelectionChanged()
        } catch {
            status = "Could not save: \(error.localizedDescription)"
        }
    }

    /// Applies one decision to everything currently listed, so curating a large
    /// provider does not mean hundreds of individual clicks.
    func setAllVisible(_ provider: String, exposed: Bool) async {
        let wanted = visibleModels.reduce(into: [String: Bool]()) { $0[$1.modelID] = exposed }
        guard !wanted.isEmpty else { return }
        do {
            try await client.setExposedModels(provider: provider, exposed: wanted)
            await load()
            await onSelectionChanged()
        } catch {
            status = "Could not save: \(error.localizedDescription)"
        }
    }

    func saveProvider(
        name: String, baseURL: String, protocolName: String, authKind: String, apiKey: String
    ) async -> Bool {
        do {
            try await client.saveProvider(
                name: name, baseURL: baseURL, protocolName: protocolName,
                authKind: authKind, keyRef: nil, apiKey: apiKey)
            await load()
            selectedProvider = name
            status = "Saved \(name)."
            return true
        } catch {
            status = "Could not save provider: \(error.localizedDescription)"
            return false
        }
    }

    func delete(_ provider: String) async {
        do {
            try await client.deleteProvider(name: provider)
            if selectedProvider == provider { selectedProvider = nil }
            await load()
            // Removing a provider drops its exposed models, so the picker has to
            // be rebuilt or it keeps offering models the daemon can no longer run.
            await onSelectionChanged()
        } catch {
            status = "Could not remove: \(error.localizedDescription)"
        }
    }

    func setCost(_ provider: String, _ model: String, input: Double, output: Double) async {
        do {
            try await client.setModelCost(
                provider: provider, model: model, input: input, output: output)
            await load()
        } catch {
            status = "Could not save cost: \(error.localizedDescription)"
        }
    }
}

struct ModelSettingsView: View {
    @Bindable var model: ModelSettingsModel
    @State private var addingProvider = false

    var body: some View {
        HSplitView {
            providerList.frame(minWidth: 240, idealWidth: 280)
            modelList.frame(minWidth: 380)
        }
        .task { await model.load() }
        .sheet(isPresented: $addingProvider) {
            AddProviderSheet(model: model, isPresented: $addingProvider)
        }
        .safeAreaInset(edge: .bottom) {
            if let status = model.status {
                HStack(spacing: 8) {
                    Text(status).font(.caption).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Button("Dismiss") { model.status = nil }
                        .buttonStyle(.plain).font(.caption).foregroundStyle(.secondary)
                }
                .padding(.horizontal, 14).padding(.vertical, 8)
                .background(.quaternary.opacity(0.4))
            }
        }
    }

    private var providerList: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Providers").font(.callout.weight(.medium))
                Spacer()
                Button {
                    addingProvider = true
                } label: {
                    Label("Add Provider", systemImage: "plus")
                        .labelStyle(.iconOnly)
                }
                .buttonStyle(.plain)
                .help("Add any OpenAI-compatible endpoint")
            }
            .padding(.horizontal, 12).padding(.vertical, 9)
            Divider()

            List(model.providers, selection: $model.selectedProvider) { provider in
                VStack(alignment: .leading, spacing: 3) {
                    HStack(spacing: 6) {
                        Text(provider.name).font(.callout.weight(.medium))
                        if !provider.builtin {
                            Text("custom").font(.caption2)
                                .padding(.horizontal, 5).padding(.vertical, 1)
                                .background(.quaternary, in: .rect(cornerRadius: 4))
                        }
                        Spacer()
                        Text("\(provider.exposedCount)/\(provider.modelCount)")
                            .font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                            .help("models exposed of models fetched")
                    }
                    Text(provider.baseURL)
                        .font(.caption).foregroundStyle(.tertiary)
                        .lineLimit(1).truncationMode(.middle)
                    if let error = provider.fetchError, !error.isEmpty {
                        // Say when it happened. A stored error survives until
                        // the next successful fetch, so an unlabelled one reads
                        // as current and sends you chasing a fixed problem.
                        Label(
                            provider.fetchedAt.map { "\(error)  (last tried \($0))" } ?? error,
                            systemImage: "exclamationmark.triangle.fill")
                            .font(.caption2).foregroundStyle(.orange)
                            .lineLimit(3)
                    }
                }
                .padding(.vertical, 2)
                .tag(provider.name)
            }
        }
    }

    @ViewBuilder
    private var modelList: some View {
        if let provider = model.current {
            VStack(spacing: 0) {
                header(for: provider)
                Divider()
                if provider.modelCount == 0 {
                    ContentUnavailableView {
                        Label("No models yet", systemImage: "tray")
                    } description: {
                        Text("Fetch \(provider.name)'s model list to choose which models appear in the picker.")
                    } actions: {
                        Button("Fetch Models") {
                            Task { await model.fetch(provider.name) }
                        }
                        .disabled(model.busy)
                    }
                } else {
                    modelRows(for: provider)
                }
            }
        } else {
            ContentUnavailableView(
                "No provider selected", systemImage: "square.stack.3d.up")
        }
    }

    private func header(for provider: ModelSettingsProvider) -> some View {
        VStack(alignment: .leading, spacing: 7) {
            HStack(spacing: 8) {
                Text(provider.name).font(.headline)
                Spacer()
                if model.busy { ProgressView().controlSize(.small) }
                Button("Fetch Models") { Task { await model.fetch(provider.name) } }
                    .disabled(model.busy)
                if !provider.builtin {
                    Button("Remove", role: .destructive) {
                        Task { await model.delete(provider.name) }
                    }
                }
            }
            HStack(spacing: 10) {
                if let at = provider.fetchedAt {
                    Text("Fetched \(at)").font(.caption).foregroundStyle(.secondary)
                } else {
                    Text("Never fetched").font(.caption).foregroundStyle(.secondary)
                }
                // Where the credential lives, never the credential.
                //
                // hasCredential is the server actually resolving the reference,
                // so it must be checked before the reference is displayed. A
                // provider can hold "env:CEREBRAS_API_KEY" while that variable
                // is unset, and showing the reference alone reported a working
                // key for a provider the server had just refused to fetch for.
                if provider.needsNoCredential {
                    Text("no credential needed").font(.caption).foregroundStyle(.tertiary)
                } else if !provider.hasCredential {
                    Label("no working credential", systemImage: "exclamationmark.triangle")
                        .font(.caption).foregroundStyle(.orange)
                        .help(provider.usesSubscription
                            ? "Sign in with this provider's CLI on the machine running the daemon, then Fetch Models"
                            : "Add a key for this provider in Settings › Providers")
                } else if let ref = provider.keyRef, !ref.isEmpty {
                    Label(ref, systemImage: ref.hasPrefix("keychain:") ? "key.fill" : "doc.text")
                        .font(.caption).foregroundStyle(.tertiary).lineLimit(1)
                } else if provider.usesSubscription {
                    Text("vendor login").font(.caption).foregroundStyle(.tertiary)
                }
            }

            // Keys are entered in Settings › Providers, which owns credentials
            // for every provider. A second key field here was a second place to
            // get it wrong: this page's field belonged to the window rather than
            // the selected row, so a key typed for one provider could be saved
            // against another.

            HStack(spacing: 8) {
                TextField("Filter models", text: $model.search)
                    .textFieldStyle(.roundedBorder)
                Button("Expose All") {
                    Task { await model.setAllVisible(provider.name, exposed: true) }
                }
                .help("Expose every model currently listed")
                Button("Expose None") {
                    Task { await model.setAllVisible(provider.name, exposed: false) }
                }
            }
        }
        .padding(12)
    }

    private func modelRows(for provider: ModelSettingsProvider) -> some View {
        List(model.visibleModels) { entry in
            HStack(spacing: 10) {
                Toggle("", isOn: Binding(
                    get: { entry.exposed },
                    set: { want in
                        Task { await model.setExposed(provider.name, entry.modelID, want) }
                    }))
                    .labelsHidden()
                    .help("Show this model in the picker")

                VStack(alignment: .leading, spacing: 2) {
                    Text(entry.displayName ?? entry.modelID)
                        .font(.callout).lineLimit(1)
                    HStack(spacing: 8) {
                        if entry.displayName != nil {
                            Text(entry.modelID).font(.caption.monospaced())
                                .foregroundStyle(.tertiary).lineLimit(1)
                        }
                        if let ctx = entry.contextSummary {
                            Text(ctx).font(.caption).foregroundStyle(.tertiary)
                        }
                    }
                }
                Spacer()
                CostCell(provider: provider.name, entry: entry, model: model)
            }
            .padding(.vertical, 2)
        }
    }
}

/// Shows a model's price and lets an unknown or wrong one be corrected.
///
/// Most provider model endpoints report no pricing at all, so "cost unknown" is
/// the common state and typing a figure here is the only way to get an estimate
/// for those models. A typed figure outranks later fetches.
private struct CostCell: View {
    let provider: String
    let entry: ModelSettingsEntry
    @Bindable var model: ModelSettingsModel

    @State private var editing = false
    @State private var inputDraft = ""
    @State private var outputDraft = ""

    var body: some View {
        Group {
            if editing {
                HStack(spacing: 4) {
                    TextField("in", text: $inputDraft).frame(width: 54)
                    TextField("out", text: $outputDraft).frame(width: 54)
                    Button("Save") { save() }.controlSize(.small)
                    Button("Cancel") { editing = false }
                        .buttonStyle(.plain).font(.caption).foregroundStyle(.secondary)
                }
                .textFieldStyle(.roundedBorder)
                .font(.caption)
            } else {
                Button {
                    inputDraft = entry.inputCost.map { String($0) } ?? ""
                    outputDraft = entry.outputCost.map { String($0) } ?? ""
                    editing = true
                } label: {
                    HStack(spacing: 4) {
                        Text(entry.priceSummary)
                            .font(.caption)
                            .foregroundStyle(entry.inputCost == nil ? .orange : .secondary)
                        if entry.costSource == "user" {
                            Image(systemName: "pencil").font(.caption2).foregroundStyle(.tertiary)
                                .help("Price you entered")
                        }
                    }
                }
                .buttonStyle(.plain)
                .help("Click to set the price")
            }
        }
    }

    private func save() {
        guard let input = Double(inputDraft), let output = Double(outputDraft) else {
            model.status = "Costs must be numbers, in dollars per million tokens."
            return
        }
        editing = false
        Task { await model.setCost(provider, entry.modelID, input: input, output: output) }
    }
}

/// Add any OpenAI-compatible endpoint: a gateway, a proxy, a local server.
private struct AddProviderSheet: View {
    @Bindable var model: ModelSettingsModel
    @Binding var isPresented: Bool

    @State private var name = ""
    @State private var baseURL = ""
    @State private var protocolName = "openai_compat"
    @State private var authKind = "api_key"
    @State private var apiKey = ""
    @State private var saving = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Add Provider").font(.headline)

            Form {
                TextField("Name", text: $name)
                    .help("How it appears in the picker, e.g. my-proxy")
                TextField("Endpoint", text: $baseURL)
                    .help("Base URL, e.g. https://gateway.example.com/v1")

                Picker("Protocol", selection: $protocolName) {
                    Text("OpenAI-compatible").tag("openai_compat")
                    Text("Anthropic").tag("anthropic")
                }

                Picker("Auth", selection: $authKind) {
                    Text("API key").tag("api_key")
                    Text("None (local server)").tag("none")
                }

                if authKind == "api_key" {
                    SecureField("API key", text: $apiKey)
                    Text("Stored in your macOS Keychain. It is never written to the settings file.")
                        .font(.caption).foregroundStyle(.secondary)
                }
            }
            .formStyle(.grouped)

            HStack {
                Spacer()
                Button("Cancel") { isPresented = false }
                Button("Add") { add() }
                    .buttonStyle(.borderedProminent)
                    .disabled(name.trimmed.isEmpty || baseURL.trimmed.isEmpty || saving)
            }
        }
        .padding(16)
        .frame(width: 460)
    }

    private func add() {
        saving = true
        Task {
            let ok = await model.saveProvider(
                name: name.trimmed, baseURL: baseURL.trimmed,
                protocolName: protocolName, authKind: authKind, apiKey: apiKey)
            saving = false
            if ok { isPresented = false }
        }
    }
}
