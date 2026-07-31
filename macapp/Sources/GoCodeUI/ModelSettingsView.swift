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
    var loadState: CollectionLoadState = .idle

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

    /// `clearingStatus` defaults to `true` for the page's own initial load,
    /// where there is no prior action's message to protect. Every action
    /// below (fetch/setExposed/setAllVisible/saveProvider/delete) reloads to
    /// pick up the server's new state but passes `false`, so the message it
    /// just set two lines earlier survives the reload it triggers instead of
    /// being erased before the operator can read it (#999 finding 8).
    func load(clearingStatus: Bool = true) async {
        loadState = .loading
        do {
            providers = try await client.modelSettings()
            if selectedProvider == nil { selectedProvider = providers.first?.name }
            if clearingStatus { status = nil }
            loadState = .loaded
        } catch {
            loadState = .failed(error.localizedDescription)
            status = "Could not load model settings: \(error.localizedDescription)"
        }
    }

    func fetch(_ provider: String) async {
        busy = true
        defer { busy = false }
        do {
            let count = try await client.fetchProviderModels(name: provider)
            status = "Fetched \(count) models from \(provider)."
            await load(clearingStatus: false)
            await onSelectionChanged()
        } catch {
            // The provider's own reason is the useful part — a bad key, an
            // unreachable host — so it is shown verbatim rather than summarised.
            status = "Fetch failed: \(error.localizedDescription)"
            await load(clearingStatus: false)
        }
    }

    func setExposed(_ provider: String, _ model: String, _ exposed: Bool) async {
        do {
            try await client.setExposedModels(provider: provider, exposed: [model: exposed])
            await load(clearingStatus: false)
            await onSelectionChanged()
        } catch {
            status = "Could not save: \(error.localizedDescription)"
            await load(clearingStatus: false)
        }
    }

    /// Applies one decision to everything currently listed, so curating a large
    /// provider does not mean hundreds of individual clicks.
    func setAllVisible(_ provider: String, exposed: Bool) async {
        let wanted = visibleModels.reduce(into: [String: Bool]()) { $0[$1.modelID] = exposed }
        guard !wanted.isEmpty else { return }
        do {
            try await client.setExposedModels(provider: provider, exposed: wanted)
            await load(clearingStatus: false)
            await onSelectionChanged()
        } catch {
            status = "Could not save: \(error.localizedDescription)"
            await load(clearingStatus: false)
        }
    }

    func saveProvider(
        name: String, baseURL: String, protocolName: String, authKind: String, apiKey: String
    ) async -> Bool {
        do {
            try await client.saveProvider(
                name: name, baseURL: baseURL, protocolName: protocolName,
                authKind: authKind, keyRef: nil, apiKey: apiKey)
            await load(clearingStatus: false)
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
            await load(clearingStatus: false)
            // Removing a provider drops its exposed models, so the picker has to
            // be rebuilt or it keeps offering models the daemon can no longer run.
            await onSelectionChanged()
        } catch {
            status = "Could not remove: \(error.localizedDescription)"
            await load(clearingStatus: false)
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
    @State private var removeConfirmation: DestructiveConfirmation?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        HSplitView {
            providerList.frame(
                minWidth: Layout.providerMinimumWidth, idealWidth: Layout.providerIdealWidth)
            modelList.frame(minWidth: Layout.modelMinimumWidth)
        }
        .task { await model.load() }
        .sheet(isPresented: $addingProvider) {
            AddProviderSheet(model: model, isPresented: $addingProvider)
        }
        .destructiveConfirmation($removeConfirmation)
        .safeAreaInset(edge: .bottom) {
            if let status = model.status {
                HStack(spacing: Spacing.standard) {
                    Text(status).font(Typography.caption).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Button("Dismiss") { model.status = nil }
                        .buttonStyle(.plain).font(Typography.caption).foregroundStyle(
                            Theme.foregroundTertiary)
                }
                .padding(.horizontal, 14).padding(.vertical, Spacing.standard)
                .background(Theme.surface)
            }
        }
    }

    private var providerList: some View {
        VStack(spacing: Spacing.none) {
            HStack {
                Text("Providers").font(Typography.body.weight(.medium))
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
            .padding(.horizontal, Spacing.inset).padding(.vertical, 9)
            Divider()

            List(selection: $model.selectedProvider) {
                if model.loadState.showsPlaceholder(itemCount: model.providers.count) {
                    ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                        LoadingPlaceholder(height: Layout.modelProviderRowHeight)
                    }
                } else if model.loadState.showsBlockingError(
                    itemCount: model.providers.count)
                {
                    CollectionErrorState(message: model.loadState.errorMessage ?? "") {
                        Task { await model.load() }
                    }
                } else {
                    if model.loadState.showsRefreshError(itemCount: model.providers.count) {
                        CollectionRefreshErrorState(message: model.loadState.errorMessage ?? "") {
                            Task { await model.load() }
                        }
                    }
                    ForEach(model.providers) { provider in
                        VStack(alignment: .leading, spacing: 3) {
                            HStack(spacing: Spacing.small) {
                                Text(provider.name).font(Typography.body.weight(.medium))
                                if !provider.builtin {
                                    Text("custom").font(Typography.detail)
                                        .padding(.horizontal, 5).padding(
                                            .vertical, Spacing.hairline
                                        )
                                        .background(
                                            Theme.surfaceHighest,
                                            in: .rect(cornerRadius: CornerRadius.tag))
                                }
                                Spacer()
                                Text("\(provider.exposedCount)/\(provider.modelCount)")
                                    .font(Typography.numericCaption).foregroundStyle(
                                        Theme.foregroundTertiary
                                    )
                                    .help("models exposed of models fetched")
                            }
                            Text(provider.baseURL)
                                .font(Typography.caption).foregroundStyle(
                                    Theme.foregroundQuaternary
                                )
                                .lineLimit(1).truncationMode(.middle)
                            if let error = provider.fetchError, !error.isEmpty {
                                // Say when it happened. A stored error survives until
                                // the next successful fetch, so an unlabelled one reads
                                // as current and sends you chasing a fixed problem.
                                Label(
                                    provider.fetchedAt.map { "\(error)  (last tried \($0))" }
                                        ?? error,
                                    systemImage: "exclamationmark.triangle.fill"
                                )
                                .font(Typography.detail).foregroundStyle(.orange)
                                .lineLimit(3)
                            }
                        }
                        .padding(.vertical, Spacing.tight)
                        .tag(provider.name)
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var modelList: some View {
        if model.loadState.showsPlaceholder(itemCount: model.providers.count) {
            VStack(spacing: Spacing.none) {
                LoadingPlaceholder(height: Layout.loadingRowHeight)
                    .padding(Spacing.inset)
                Divider()
                VStack(spacing: Spacing.standard) {
                    ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                        LoadingPlaceholder(height: Layout.modelSettingsRowHeight)
                    }
                }
                .padding(Spacing.inset)
                Spacer()
            }
        } else if model.loadState.showsBlockingError(itemCount: model.providers.count) {
            CollectionErrorState(message: model.loadState.errorMessage ?? "") {
                Task { await model.load() }
            }
            .frame(maxHeight: .infinity)
        } else if let provider = model.current {
            VStack(spacing: Spacing.none) {
                if model.loadState.showsRefreshError(itemCount: model.providers.count) {
                    CollectionRefreshErrorState(message: model.loadState.errorMessage ?? "") {
                        Task { await model.load() }
                    }
                    .padding(.horizontal, Spacing.inset)
                    Divider()
                }
                header(for: provider)
                Divider()
                if provider.modelCount == 0 {
                    ContentUnavailableView {
                        Label("No models yet", systemImage: "tray")
                    } description: {
                        Text(
                            "Fetch \(provider.name)'s model list to choose which models appear in the picker."
                        )
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
            HStack(spacing: Spacing.standard) {
                Text(provider.name).font(Typography.heading)
                Spacer()
                ProgressView()
                    .controlSize(.small)
                    .frame(width: Layout.inlineActivitySlot, height: Layout.inlineActivitySlot)
                    .opacity(model.busy ? StateOpacity.visible : StateOpacity.hidden)
                    // The spinner shares a permanent slot with the fetch
                    // controls, so refreshing a provider cannot shift them.
                    .animation(
                        reduceMotion ? nil : .easeInOut(duration: Motion.loadingFadeDuration),
                        value: model.busy)
                Button("Fetch Models") { Task { await model.fetch(provider.name) } }
                    .disabled(model.busy)
                if !provider.builtin {
                    Button("Remove", role: .destructive) {
                        confirmRemove(provider)
                    }
                }
            }
            HStack(spacing: Spacing.comfortable) {
                if let at = provider.fetchedAt {
                    Text("Fetched \(at)").font(Typography.caption).foregroundStyle(
                        Theme.foregroundTertiary)
                } else {
                    Text("Never fetched").font(Typography.caption).foregroundStyle(
                        Theme.foregroundTertiary)
                }
                // Where the credential lives, never the credential.
                //
                // hasCredential is the server actually resolving the reference,
                // so it must be checked before the reference is displayed. A
                // provider can hold "env:CEREBRAS_API_KEY" while that variable
                // is unset, and showing the reference alone reported a working
                // key for a provider the server had just refused to fetch for.
                if provider.needsNoCredential {
                    Text("no credential needed").font(Typography.caption).foregroundStyle(
                        Theme.foregroundQuaternary)
                } else if !provider.hasCredential {
                    Label("no working credential", systemImage: "exclamationmark.triangle")
                        .font(Typography.caption).foregroundStyle(.orange)
                        .help(
                            provider.usesSubscription
                                ? "Sign in with this provider's CLI on the machine running the daemon, then Fetch Models"
                                : "Add a key for this provider in Settings › Providers")
                } else if let ref = provider.keyRef, !ref.isEmpty {
                    Label(ref, systemImage: ref.hasPrefix("keychain:") ? "key.fill" : "doc.text")
                        .font(Typography.caption).foregroundStyle(Theme.foregroundQuaternary)
                        .lineLimit(1)
                } else if provider.usesSubscription {
                    Text("vendor login").font(Typography.caption).foregroundStyle(
                        Theme.foregroundQuaternary)
                }
            }

            // Keys are entered in Settings › Providers, which owns credentials
            // for every provider. A second key field here was a second place to
            // get it wrong: this page's field belonged to the window rather than
            // the selected row, so a key typed for one provider could be saved
            // against another.

            HStack(spacing: Spacing.standard) {
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
        .padding(Spacing.inset)
    }

    /// States what removing a provider drops -- its exposed models, already
    /// documented above at the credential-status help text -- before it
    /// happens (R10), rather than deleting the moment the button is tapped.
    private func confirmRemove(_ provider: ModelSettingsProvider) {
        removeConfirmation = DestructiveConfirmation(
            title: "Remove \(provider.name)?",
            message:
                "\"\(provider.name)\" and its exposed models will no longer be offered in the picker. It cannot be undone.",
            confirmLabel: "Remove"
        ) {
            Task { await model.delete(provider.name) }
        }
    }

    private func modelRows(for provider: ModelSettingsProvider) -> some View {
        List(model.visibleModels) { entry in
            HStack(spacing: Spacing.comfortable) {
                Toggle(
                    "",
                    isOn: Binding(
                        get: { entry.exposed },
                        set: { want in
                            Task { await model.setExposed(provider.name, entry.modelID, want) }
                        })
                )
                .labelsHidden()
                .help("Show this model in the picker")
                .accessibilityLabel("Show \(entry.modelID) in the picker")

                VStack(alignment: .leading, spacing: Spacing.tight) {
                    Text(entry.displayName ?? entry.modelID)
                        .font(Typography.body).lineLimit(1)
                    HStack(spacing: Spacing.standard) {
                        if entry.displayName != nil {
                            Text(entry.modelID).font(Typography.codeCaption)
                                .foregroundStyle(Theme.foregroundQuaternary).lineLimit(1)
                        }
                        if let ctx = entry.contextSummary {
                            Text(ctx).font(Typography.caption).foregroundStyle(
                                Theme.foregroundQuaternary)
                        }
                    }
                }
                Spacer()
                CostCell(provider: provider.name, entry: entry, model: model)
            }
            .padding(.vertical, Spacing.tight)
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
                HStack(spacing: Spacing.compact) {
                    TextField("in", text: $inputDraft).frame(width: Layout.costFieldWidth)
                    TextField("out", text: $outputDraft).frame(width: Layout.costFieldWidth)
                    Button("Save") { save() }.controlSize(.small)
                    Button("Cancel") { editing = false }
                        .buttonStyle(.plain).font(Typography.caption).foregroundStyle(
                            Theme.foregroundTertiary)
                }
                .textFieldStyle(.roundedBorder)
                .font(Typography.caption)
            } else {
                Button {
                    inputDraft = entry.inputCost.map { String($0) } ?? ""
                    outputDraft = entry.outputCost.map { String($0) } ?? ""
                    editing = true
                } label: {
                    HStack(spacing: Spacing.compact) {
                        Text(entry.priceSummary)
                            .font(Typography.caption)
                            .foregroundStyle(
                                entry.inputCost == nil ? .orange : Theme.foregroundTertiary)
                        if entry.costSource == "user" {
                            Image(systemName: "pencil").font(Typography.detail).foregroundStyle(
                                Theme.foregroundQuaternary
                            )
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
        VStack(alignment: .leading, spacing: Spacing.inset) {
            Text("Add Provider").font(Typography.heading)

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
                        .font(Typography.caption).foregroundStyle(Theme.foregroundTertiary)
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
        .padding(Spacing.large)
        .frame(width: Layout.providerSheetWidth)
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
