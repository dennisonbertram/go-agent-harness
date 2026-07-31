import HarnessKit
import SwiftUI

struct SettingsView: View {
    @Bindable var project: ProjectSession
    @State private var tab = Tab.providers

    enum Tab: String, CaseIterable, Identifiable {
        case providers, models, project, access
        var id: String { rawValue }
        var title: String {
            switch self {
            case .providers: return "Providers"
            case .models: return "Models"
            case .project: return "Project"
            case .access: return "Access"
            }
        }
    }

    var body: some View {
        VStack(spacing: Spacing.none) {
            Picker("", selection: $tab) {
                ForEach(Tab.allCases) { Text($0.title).tag($0) }
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .padding(Spacing.inset)
            Divider()

            switch tab {
            case .providers: ProvidersTab(project: project)
            case .models: ModelSettingsTab(project: project)
            case .project: ProjectTab(project: project)
            case .access: AccessTab(project: project)
            }
        }
        .task { await project.refreshCatalog() }
    }
}

private struct ProvidersTab: View {
    @Bindable var project: ProjectSession
    @State private var editing: String?
    @State private var keyDraft = ""

    var body: some View {
        List {
            if project.providersLoadState.showsPlaceholder(itemCount: project.providers.count) {
                ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                    LoadingPlaceholder(height: Layout.modelProviderRowHeight)
                }
            } else if project.providersLoadState.showsBlockingError(
                itemCount: project.providers.count)
            {
                CollectionErrorState(message: project.providersLoadState.errorMessage ?? "") {
                    Task { await project.refreshCatalog() }
                }
            } else {
                if project.providersLoadState.showsRefreshError(
                    itemCount: project.providers.count)
                {
                    CollectionRefreshErrorState(
                        message: project.providersLoadState.errorMessage ?? ""
                    ) {
                        Task { await project.refreshCatalog() }
                    }
                }
                ForEach(project.providers) { provider in
                    VStack(alignment: .leading, spacing: 7) {
                        HStack {
                            Image(
                                systemName: provider.configured
                                    ? "checkmark.seal.fill" : "exclamationmark.circle"
                            )
                            .foregroundStyle(
                                provider.configured ? .green : Theme.foregroundTertiary)
                            Text(provider.name).font(Typography.body.weight(.medium))
                            if let count = provider.modelCount {
                                Text("\(count) models").font(Typography.caption).foregroundStyle(
                                    Theme.foregroundQuaternary)
                            }
                            Spacer()
                            if provider.supportsSubscriptionImport {
                                Button("Import Login") {
                                    Task {
                                        await project.importSubscription(provider: provider.name)
                                    }
                                }
                                .help(
                                    "Reads the vendor CLI credential from the machine running the harness server"
                                )
                            } else {
                                Button(editing == provider.name ? "Cancel" : "Set Key…") {
                                    editing = editing == provider.name ? nil : provider.name
                                    keyDraft = ""
                                }
                            }
                        }

                        if let env = provider.apiKeyEnv, !provider.configured {
                            Text("Reads \(env), or set a key here.")
                                .font(Typography.caption).foregroundStyle(Theme.foregroundTertiary)
                        }

                        if editing == provider.name {
                            HStack {
                                // SecureField so the key is never shown in the clear.
                                SecureField("API key", text: $keyDraft)
                                Button("Save") {
                                    let key = keyDraft
                                    keyDraft = ""
                                    editing = nil
                                    Task {
                                        await project.setProviderKey(
                                            provider: provider.name, key: key)
                                    }
                                }
                                .disabled(keyDraft.isEmpty)
                            }
                        }
                    }
                    .padding(.vertical, Spacing.compact)
                }
            }
        }
        .listStyle(.inset)
        .overlay(alignment: .bottom) { StatusToast(message: project.statusMessage) }
    }
}

private struct ModelsTab: View {
    @Bindable var project: ProjectSession
    @State private var search = ""

    var body: some View {
        VStack(spacing: Spacing.none) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.foregroundTertiary)
                TextField("Filter models", text: $search).textFieldStyle(.plain)
            }
            .padding(Spacing.comfortable)
            Divider()

            List {
                if project.modelsLoadState.showsPlaceholder(itemCount: project.models.count) {
                    ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                        LoadingPlaceholder(height: Layout.loadingRowHeight)
                    }
                } else if project.modelsLoadState.showsBlockingError(
                    itemCount: project.models.count)
                {
                    CollectionErrorState(message: project.modelsLoadState.errorMessage ?? "") {
                        Task { await project.refreshCatalog() }
                    }
                } else {
                    if project.modelsLoadState.showsRefreshError(itemCount: project.models.count) {
                        CollectionRefreshErrorState(
                            message: project.modelsLoadState.errorMessage ?? ""
                        ) {
                            Task { await project.refreshCatalog() }
                        }
                    }
                    ForEach(filtered) { model in
                        Button {
                            project.selectedModel = model.id
                        } label: {
                            HStack {
                                VStack(alignment: .leading, spacing: Spacing.tight) {
                                    Text(model.id).font(Typography.body)
                                    HStack(spacing: Spacing.standard) {
                                        Text(model.provider)
                                        // Price and image support are the two facts that
                                        // actually drive model choice; the TUI shows neither.
                                        if let price = model.priceSummary { Text(price) }
                                        if model.supportsImages {
                                            Label("images", systemImage: "photo").labelStyle(
                                                .titleAndIcon)
                                        }
                                    }
                                    .font(Typography.caption).foregroundStyle(
                                        Theme.foregroundTertiary)
                                }
                                Spacer()
                                if project.selectedModel == model.id {
                                    Image(systemName: "checkmark").foregroundStyle(.tint)
                                }
                            }
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel(accessibilityLabel(for: model))
                    }
                }
            }
            .listStyle(.inset)
        }
    }

    private var filtered: [ModelInfo] {
        let all = project.models.sorted { $0.id < $1.id }
        guard !search.isEmpty else { return all }
        return all.filter {
            $0.id.localizedCaseInsensitiveContains(search)
                || $0.provider.localizedCaseInsensitiveContains(search)
        }
    }

    /// Names the model and its provider, plus whether it is the current
    /// selection, so VoiceOver reads more than a bare model id (R9).
    private func accessibilityLabel(for model: ModelInfo) -> String {
        var label = "\(model.id), \(model.provider)"
        if project.selectedModel == model.id { label += ", selected" }
        return label
    }
}

private struct ProjectTab: View {
    @Bindable var project: ProjectSession
    @State private var undoConfirmation: DestructiveConfirmation?

    var body: some View {
        Form {
            LabeledContent("Workspace") {
                Text(project.workspace.path).textSelection(.enabled).font(Typography.code)
            }
            LabeledContent("Model") { Text(project.selectedModel ?? "Server default") }
            Picker("Profile", selection: $project.selectedProfile) {
                Text("None").tag(String?.none)
                ForEach(project.profiles) { profile in
                    Text(profile.name).tag(String?.some(profile.name))
                }
            }
            LabeledContent("Plan mode") { Text(project.planMode ? "On" : "Off") }
            LabeledContent("Conversation") {
                Text(project.run?.conversationID ?? "None yet").font(Typography.code)
            }
            LabeledContent("Conversation actions") {
                HStack {
                    Button("Fork") { Task { await project.fork() } }
                        .disabled(project.conversationActionDisabledReason != nil)
                        .help(project.conversationActionDisabledReason ?? "Fork conversation")
                        .accessibilityHint(project.conversationActionDisabledReason ?? "")
                    Button("Undo Last Prompt") { confirmUndo() }
                        .disabled(project.conversationActionDisabledReason != nil)
                        .help(project.conversationActionDisabledReason ?? "Undo last prompt")
                        .accessibilityHint(project.conversationActionDisabledReason ?? "")
                }
            }
        }
        .formStyle(.grouped)
        .destructiveConfirmation($undoConfirmation)
    }

    /// States what turn will be lost before it is lost (R6).
    private func confirmUndo() {
        let lastPrompt = UndoPreview.lastUserPrompt(in: project.run?.transcript.items ?? [])
        undoConfirmation = DestructiveConfirmation(
            title: "Undo last turn?",
            message: UndoPreview.message(lastPrompt: lastPrompt),
            confirmLabel: "Undo"
        ) {
            Task { await project.undo() }
        }
    }
}

private struct StatusToast: View {
    let message: String?
    var body: some View {
        if let message {
            Text(message)
                .font(Typography.caption)
                .padding(.horizontal, Spacing.inset).padding(.vertical, 7)
                .background(Theme.surfaceElevated, in: .capsule)
                .padding(.bottom, Spacing.inset)
        }
    }
}

/// Extra directories a run may touch beyond the workspace root — the TUI's
/// `/add-dir`. Session-scoped, matching the TUI, so nothing is persisted.
private struct AccessTab: View {
    @Bindable var project: ProjectSession

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.none) {
            HStack {
                Text("Extra directories").font(Typography.body.weight(.medium))
                Spacer()
                Button("Add…", action: add)
            }
            .padding(Spacing.inset)
            Text(
                "Runs can read and write inside the workspace. Add a directory to grant access beyond it for this session."
            )
            .font(Typography.caption).foregroundStyle(Theme.foregroundTertiary)
            .padding(.horizontal, Spacing.inset)
            Divider().padding(.top, Spacing.comfortable)

            if project.extraDirs.isEmpty {
                EmptyState(
                    icon: "folder.badge.plus",
                    title: "No extra directories",
                    detail: "The agent is limited to \(project.workspace.lastPathComponent).")
            } else {
                List(project.extraDirs, id: \.self) { url in
                    HStack {
                        Image(systemName: "folder").foregroundStyle(Theme.foregroundTertiary)
                        Text(url.path).font(Typography.code).lineLimit(1)
                            .truncationMode(.head)
                        Spacer()
                        Button("Remove") { project.removeDirectory(url) }
                            .buttonStyle(.plain).font(Typography.caption).foregroundStyle(
                                Theme.foregroundTertiary)
                    }
                }
                .listStyle(.inset)
            }
        }
    }

    private func add() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.prompt = "Grant Access"
        if panel.runModal() == .OK, let url = panel.url {
            project.addDirectory(url)
        }
    }
}

/// Hosts the model settings page, holding its model across tab switches so a
/// fetch in progress is not thrown away by flipping tabs.
private struct ModelSettingsTab: View {
    @Bindable var project: ProjectSession
    @State private var model: ModelSettingsModel?

    var body: some View {
        Group {
            if let model {
                ModelSettingsView(model: model)
            } else {
                HSplitView {
                    VStack(spacing: Spacing.none) {
                        LoadingPlaceholder(height: Layout.loadingRowHeight)
                            .padding(Spacing.inset)
                        Divider()
                        VStack(spacing: Spacing.standard) {
                            ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                                LoadingPlaceholder(height: Layout.modelProviderRowHeight)
                            }
                        }
                        .padding(Spacing.inset)
                        Spacer()
                    }
                    .frame(
                        minWidth: Layout.providerMinimumWidth,
                        idealWidth: Layout.providerIdealWidth)
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
                    .frame(minWidth: Layout.modelMinimumWidth)
                }
            }
        }
        .onAppear {
            if model == nil, let client = project.harnessClient {
                // Refreshing the project catalog is what makes the model chip
                // reflect a new selection without relaunching the app.
                model = ModelSettingsModel(client: client) {
                    await project.refreshCatalog()
                }
            }
        }
    }
}
