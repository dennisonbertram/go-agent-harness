import Foundation
import HarnessKit
import Observation

/// Locates the `harnessd` binary to supervise.
public enum HarnessBinary {
    /// Resolution order: explicit override, then `PATH`, then a repo-local
    /// build — what a developer running from source has. A `PATH` entry
    /// with no `prompts/catalog.yaml` resolvable above it cannot actually
    /// boot (harnessd exits at startup with no prompt engine); a bootable
    /// candidate is preferred over one that isn't, wherever it was found
    /// (#951 finding 11 — this is exactly the failure a PATH-installed
    /// harnessd caused in practice).
    public static func locate(fileManager: FileManager = .default) -> URL? {
        if let override = ProcessInfo.processInfo.environment["HARNESS_BINARY"] {
            let url = URL(fileURLWithPath: override)
            if fileManager.isExecutableFile(atPath: url.path) { return url }
        }

        let candidates =
            pathCandidates(fileManager: fileManager)
            + repoLocalCandidates(
                startingPoints: [
                    URL(fileURLWithPath: fileManager.currentDirectoryPath), Bundle.main.bundleURL,
                ], fileManager: fileManager)
        return candidates.first(where: { canBoot($0, fileManager: fileManager) })
            ?? candidates.first
    }

    private static func pathCandidates(fileManager: FileManager) -> [URL] {
        (ProcessInfo.processInfo.environment["PATH"]?.split(separator: ":") ?? [])
            .map { URL(fileURLWithPath: String($0)).appending(path: "harnessd") }
            .filter { fileManager.isExecutableFile(atPath: $0.path) }
    }

    /// Walks up from each starting point for `.harnessd-bin/harnessd` — what
    /// `scripts/live-test.sh` builds — or a bare `harnessd`.
    static func repoLocalCandidates(startingPoints: [URL], fileManager: FileManager) -> [URL] {
        var found: [URL] = []
        for start in startingPoints {
            var directory = start
            for _ in 0..<8 {
                let inBinDir = directory.appending(path: ".harnessd-bin").appending(
                    path: "harnessd")
                let bare = directory.appending(path: "harnessd")
                for candidate in [inBinDir, bare]
                where fileManager.isExecutableFile(atPath: candidate.path) {
                    found.append(candidate)
                }
                let parent = directory.deletingLastPathComponent()
                if parent.path == directory.path { break }
                directory = parent
            }
        }
        return found
    }

    /// A binary with no `prompts/catalog.yaml` above it exits immediately at
    /// startup — harnessd resolves its prompt engine relative to its
    /// installation directory (mirrors `HarnessSupervisor.findInstallationRoot`).
    static func canBoot(_ binary: URL, fileManager: FileManager) -> Bool {
        var directory = binary.deletingLastPathComponent()
        for _ in 0..<8 {
            let catalog = directory.appending(path: "prompts").appending(path: "catalog.yaml")
            if fileManager.fileExists(atPath: catalog.path) { return true }
            let parent = directory.deletingLastPathComponent()
            if parent.path == directory.path { break }
            directory = parent
        }
        return false
    }
}

public enum ProjectPhase: Sendable, Equatable {
    case idle
    case starting
    case ready
    case failed(String)
}

/// Structural representation of the server's `409 rewind_refused` safety
/// refusal (KTD-6): a file changed outside the harness since the checkpoint,
/// so a restore was declined. Matched on `HarnessError.code`, not the HTTP
/// status the server happens to send it with, because the code string is the
/// stable part of the contract. Carries the point the refusal was for so the
/// UI can offer a distinct, more severe "restore anyway" confirmation that
/// calls `rewind(to:force:)` on the same point without the caller having to
/// look it back up.
public struct RewindRefusal: Sendable, Equatable {
    public let point: RewindPoint
    public let message: String

    public var pointID: String { point.id }
}

/// Everything scoped to one open project: its harnessd, its client, and its
/// current conversation.
///
/// harnessd serves one workspace per process, so a project and a server are
/// one-to-one — see `docs/design/native-macos-app.md` §2.
@MainActor
@Observable
public final class ProjectSession {
    public let workspace: URL
    public private(set) var phase: ProjectPhase = .idle
    public private(set) var run: RunSession?
    public private(set) var conversations: [ConversationInfo] = []
    public private(set) var conversationsLoadState: CollectionLoadState = .idle
    public private(set) var models: [ModelInfo] = []
    public private(set) var modelsLoadState: CollectionLoadState = .idle
    public private(set) var providers: [ProviderInfo] = []
    public private(set) var providersLoadState: CollectionLoadState = .idle
    public private(set) var rewindPoints: [RewindPoint] = []
    public private(set) var rewindPointsLoadState: CollectionLoadState = .idle
    public private(set) var tasks: [TaskInfo] = []
    public private(set) var tasksLoadState: CollectionLoadState = .idle
    public private(set) var todos: [TodoItem] = []
    public private(set) var todosLoadState: CollectionLoadState = .idle
    /// nil when the daemon has no run store — a configuration state, not a fault.
    public private(set) var runs: [RunSummaryInfo]?
    public private(set) var runsLoadState: CollectionLoadState = .idle
    public private(set) var statusMessage: String?
    /// Set only for the server's deliberate `rewind_refused` safety refusal
    /// (KTD-6); every other `rewind` failure still lands in `statusMessage`.
    public private(set) var rewindRefusal: RewindRefusal?

    /// Model applied to the next run; nil uses the server's default.
    public var selectedModel: String?
    public var planMode = false
    /// Directories granted to runs beyond the workspace root (the TUI's
    /// /add-dir). Session-scoped, matching the TUI's behaviour.
    public private(set) var extraDirs: [URL] = []
    public private(set) var profiles: [ProfileInfo] = []
    public private(set) var profilesLoadState: CollectionLoadState = .idle
    public var selectedProfile: String?

    private var supervisor: HarnessSupervisor?
    private var client: HarnessClient?

    // Refreshes overlap in normal SwiftUI use: `.task`, Retry, pull-to-refresh,
    // navigation, and action completion can all ask for the same collection.
    // The daemon has no request ordering guarantee, so this session—not a view—
    // owns the last-request-wins boundary for every mutable result.
    private var connectionGeneration = 0
    private var modelsRequestGeneration = 0
    private var providersRequestGeneration = 0
    private var profilesRequestGeneration = 0
    private var conversationsRequestGeneration = 0
    private var rewindPointsRequestGeneration = 0
    private var tasksRequestGeneration = 0
    private var runsRequestGeneration = 0
    private var todosRequestGeneration = 0
    private var conversationSyncGeneration = 0
    private var conversationSelectionGeneration = 0
    /// Non-nil between initiating a selection and accepting its response.
    /// A durable sync for the old selection must not reconcile during that
    /// window, even though `run.conversationID` still names the old row.
    private var pendingConversationSelectionID: String?

    /// Exposed so the model settings page can talk to this project's daemon.
    /// Read-only: the session still owns the client's lifetime.
    public var harnessClient: HarnessClient? { client }
    /// Set when attaching to an externally-managed harnessd instead of spawning one.
    private let externalBaseURL: URL?
    /// Extra environment for the supervised server — used by tests to script
    /// the fake provider per project.
    private let serverEnvironment: [String: String]

    public init(
        workspace: URL, externalBaseURL: URL? = nil, serverEnvironment: [String: String] = [:]
    ) {
        self.workspace = workspace
        self.externalBaseURL = externalBaseURL
        self.serverEnvironment = serverEnvironment
    }

    public var name: String { workspace.lastPathComponent }
    public var isReady: Bool { phase == .ready }
    /// Shared explanation for controls that would otherwise be refused by
    /// `refuseIfBusy(_:)`. Views own their disabled presentation, while the
    /// session remains the authoritative lifecycle guard.
    public var conversationActionDisabledReason: String? {
        guard run?.isBusy == true else { return nil }
        return "Stop the running task before changing conversations."
    }

    // MARK: - Lifecycle

    public func start() async {
        guard phase == .idle || isFailed else { return }
        phase = .starting

        if let externalBaseURL {
            // Attaching to a server someone else owns: never terminate it.
            connect(to: externalBaseURL)
            return
        }

        guard let binary = HarnessBinary.locate() else {
            phase = .failed(
                "Could not find the harnessd binary. Set HARNESS_BINARY or put harnessd on your PATH."
            )
            return
        }

        let supervisor = HarnessSupervisor(
            binary: binary, workspace: workspace, extraEnvironment: serverEnvironment)
        self.supervisor = supervisor
        do {
            let baseURL = try await supervisor.start()
            connect(to: baseURL)
        } catch {
            phase = .failed(error.localizedDescription)
        }
    }

    /// Terminates this project's server. Called on window close so a day of
    /// opening projects does not leave a trail of orphaned servers.
    public func shutdown() async {
        connectionGeneration &+= 1
        invalidateConversationSelection()
        run?.cancel()
        run?.stopConversationStream()
        await supervisor?.stop()
        supervisor = nil
        client = nil
        phase = .idle
    }

    private func connect(to baseURL: URL) {
        connectionGeneration &+= 1
        invalidateConversationSelection()
        let client = HarnessClient(baseURL: baseURL)
        self.client = client
        self.run = RunSession(client: client)
        phase = .ready
        // Reserve the initial request generations before scheduling their
        // unstructured work. A caller that refreshes immediately after
        // `start()` must own the newer generation; the delayed startup task
        // is not allowed to begin later and replace that explicit refresh.
        modelsRequestGeneration &+= 1
        providersRequestGeneration &+= 1
        profilesRequestGeneration &+= 1
        let modelsGeneration = modelsRequestGeneration
        let providersGeneration = providersRequestGeneration
        let profilesGeneration = profilesRequestGeneration
        let initialConnection = connectionGeneration
        Task {
            await refreshCatalog(
                modelsGeneration: modelsGeneration,
                providersGeneration: providersGeneration,
                profilesGeneration: profilesGeneration,
                requestedConnection: initialConnection)
        }

        conversationsRequestGeneration &+= 1
        let conversationsGeneration = conversationsRequestGeneration
        Task {
            await refreshConversations(
                generation: conversationsGeneration, requestedConnection: initialConnection)
        }
    }

    private var isFailed: Bool {
        if case .failed = phase { return true }
        return false
    }

    // MARK: - Data

    /// `try?` here used to discard every failure, so a transport error looked
    /// identical to "nothing to show" — the daemon being briefly unreachable
    /// read the same as an empty catalog. Failures now surface via
    /// `statusMessage`; a failed refresh leaves the previous data in place
    /// rather than blanking a working catalog over one bad request (#951
    /// finding 3).
    public func refreshCatalog() async {
        modelsRequestGeneration &+= 1
        providersRequestGeneration &+= 1
        profilesRequestGeneration &+= 1
        let modelsGeneration = modelsRequestGeneration
        let providersGeneration = providersRequestGeneration
        let profilesGeneration = profilesRequestGeneration
        let requestedConnection = connectionGeneration
        await refreshCatalog(
            modelsGeneration: modelsGeneration,
            providersGeneration: providersGeneration,
            profilesGeneration: profilesGeneration,
            requestedConnection: requestedConnection)
    }

    private func refreshCatalog(
        modelsGeneration: Int,
        providersGeneration: Int,
        profilesGeneration: Int,
        requestedConnection: Int
    ) async {
        guard let client else { return }
        modelsLoadState = .loading
        providersLoadState = .loading
        profilesLoadState = .loading
        async let models = try await client.models()
        async let providers = try await client.providers()
        async let profiles = try await client.profiles()
        do {
            let fetchedModels = try await models
            guard connectionGeneration == requestedConnection,
                modelsRequestGeneration == modelsGeneration
            else { return }
            self.models = fetchedModels
            modelsLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                modelsRequestGeneration == modelsGeneration
            else { return }
            modelsLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
        do {
            let fetchedProviders = try await providers
            guard connectionGeneration == requestedConnection,
                providersRequestGeneration == providersGeneration
            else { return }
            self.providers = fetchedProviders
            providersLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                providersRequestGeneration == providersGeneration
            else { return }
            providersLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
        do {
            let fetchedProfiles = try await profiles
            guard connectionGeneration == requestedConnection,
                profilesRequestGeneration == profilesGeneration
            else { return }
            self.profiles = fetchedProfiles
            profilesLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                profilesRequestGeneration == profilesGeneration
            else { return }
            profilesLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
    }

    public func refreshConversations() async {
        conversationsRequestGeneration &+= 1
        let generation = conversationsRequestGeneration
        let requestedConnection = connectionGeneration
        await refreshConversations(generation: generation, requestedConnection: requestedConnection)
    }

    private func refreshConversations(generation: Int, requestedConnection: Int) async {
        guard let client else { return }
        conversationsLoadState = .loading
        do {
            let fetchedConversations = try await client.conversations(limit: 100)
            guard connectionGeneration == requestedConnection,
                conversationsRequestGeneration == generation
            else { return }
            conversations = fetchedConversations
            conversationsLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                conversationsRequestGeneration == generation
            else { return }
            conversationsLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
    }

    public func refreshRewindPoints() async {
        rewindPointsRequestGeneration &+= 1
        let generation = rewindPointsRequestGeneration
        let requestedConnection = connectionGeneration
        guard let client, let conversationID = run?.conversationID else {
            rewindPoints = []
            rewindPointsLoadState = .loaded
            return
        }
        rewindPointsLoadState = .loading
        do {
            let fetchedPoints = try await client.rewindPoints(conversationID: conversationID)
            guard connectionGeneration == requestedConnection,
                rewindPointsRequestGeneration == generation,
                run?.conversationID == conversationID
            else { return }
            rewindPoints = fetchedPoints
            rewindPointsLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                rewindPointsRequestGeneration == generation,
                run?.conversationID == conversationID
            else { return }
            rewindPointsLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
    }

    public func refreshActivity() async {
        tasksRequestGeneration &+= 1
        runsRequestGeneration &+= 1
        todosRequestGeneration &+= 1
        let tasksGeneration = tasksRequestGeneration
        let runsGeneration = runsRequestGeneration
        let todosGeneration = todosRequestGeneration
        let requestedConnection = connectionGeneration
        guard let client else { return }
        let runID = run?.currentRunID
        tasksLoadState = .loading
        runsLoadState = .loading
        if runID != nil {
            todosLoadState = .loading
        } else {
            todos = []
            todosLoadState = .loaded
        }
        async let fetchedTasks = try await client.tasks()
        async let fetchedRuns = try await client.runs()
        if let runID {
            async let fetchedTodos = try await client.todos(runID: runID)
            do {
                let latestTodos = try await fetchedTodos
                guard connectionGeneration == requestedConnection,
                    todosRequestGeneration == todosGeneration,
                    run?.currentRunID == runID
                else { return }
                todos = latestTodos
                todosLoadState = .loaded
            } catch {
                guard connectionGeneration == requestedConnection,
                    todosRequestGeneration == todosGeneration,
                    run?.currentRunID == runID
                else { return }
                todosLoadState = .failed(error.localizedDescription)
                statusMessage = error.localizedDescription
            }
        }
        do {
            let latestTasks = try await fetchedTasks
            guard connectionGeneration == requestedConnection,
                tasksRequestGeneration == tasksGeneration
            else { return }
            tasks = latestTasks
            tasksLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                tasksRequestGeneration == tasksGeneration
            else { return }
            tasksLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
        do {
            // `client.runs()` already turns the deliberate "no run store
            // configured" 501 into `nil`; anything thrown here is a genuine
            // transport/server failure and must not be folded into that same
            // nil, or a network blip reads as "no run store configured" — a
            // lie about the daemon's configuration (#951 finding 3).
            let latestRuns = try await fetchedRuns
            guard connectionGeneration == requestedConnection,
                runsRequestGeneration == runsGeneration
            else { return }
            runs = latestRuns
            runsLoadState = .loaded
        } catch {
            guard connectionGeneration == requestedConnection,
                runsRequestGeneration == runsGeneration
            else { return }
            runsLoadState = .failed(error.localizedDescription)
            statusMessage = error.localizedDescription
        }
    }

    /// Rehydrates the selected conversation from durable messages when Chat
    /// becomes visible. Conversation SSE remains the low-latency path; this is
    /// the durability safety net for a completed scheduled run that happened
    /// across a dropped/recreated stream or while the view was elsewhere.
    public func syncCurrentConversation() async {
        conversationSyncGeneration &+= 1
        let syncGeneration = conversationSyncGeneration
        let selectionGeneration = conversationSelectionGeneration
        let requestedConnection = connectionGeneration
        guard
            let client,
            let run,
            let conversationID = run.conversationID,
            !run.isBusy,
            pendingConversationSelectionID == nil
        else { return }
        do {
            let messages = try await client.messages(conversationID: conversationID)
            guard connectionGeneration == requestedConnection,
                conversationSyncGeneration == syncGeneration,
                conversationSelectionGeneration == selectionGeneration,
                pendingConversationSelectionID == nil,
                self.run === run,
                run.conversationID == conversationID,
                !run.isBusy
            else { return }
            run.reconcilePersistedMessages(messages)
        } catch {
            guard connectionGeneration == requestedConnection,
                conversationSyncGeneration == syncGeneration,
                conversationSelectionGeneration == selectionGeneration,
                pendingConversationSelectionID == nil,
                self.run === run,
                run.conversationID == conversationID
            else { return }
            statusMessage = error.localizedDescription
        }
    }

    // MARK: - Actions

    public func addDirectory(_ url: URL) {
        guard !extraDirs.contains(url) else { return }
        extraDirs.append(url)
    }

    public func removeDirectory(_ url: URL) {
        extraDirs.removeAll { $0 == url }
    }

    public func submit() {
        run?.model = selectedModel
        run?.planMode = planMode
        run?.extraDirs = extraDirs.map(\.path)
        run?.profile = selectedProfile
        run?.submit()
        Task {
            // `run.submit()` starts its own unstructured task that only sets
            // `conversationID` once harnessd has actually minted one — a
            // fixed sleep before refreshing was a guess at how long that
            // takes and flaked under load. Poll the observable state instead,
            // bounded so a run that never registers (e.g. it fails
            // immediately) cannot hang this refresh forever.
            for _ in 0..<20 {
                if run?.conversationID != nil { break }
                try? await Task.sleep(for: .milliseconds(100))
            }
            await refreshConversations()
        }
    }

    public func openConversation(_ conversation: ConversationInfo) async {
        guard !refuseIfBusy("switching conversations") else { return }
        await loadConversation(id: conversation.id, reportFailure: true)
    }

    public func deleteConversation(_ conversation: ConversationInfo) async {
        // Refused up front, not deleted-then-locally-refused: this used to
        // call the server's `DELETE` unconditionally and rely on
        // `newConversation()`'s own guard (below) to refuse the *local*
        // reset afterwards -- which actually deleted the conversation on
        // the server while leaving the app still bound to it, since the
        // local refusal only stopped the reset, not the delete that already
        // happened.
        guard run?.conversationID != conversation.id || run?.isBusy != true else {
            statusMessage =
                "Stop the running task before deleting the conversation it's running in."
            return
        }
        guard let client else { return }
        do {
            try await client.deleteConversation(id: conversation.id)
            if run?.conversationID == conversation.id { newConversation() }
            await refreshConversations()
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    /// Guards `newConversation`/`fork`/`undo` (KTD-9): one check here covers
    /// every call site, including `deleteConversation`'s own internal call,
    /// rather than a `.disabled(...)` per call site that a caller added
    /// later could bypass.
    private func refuseIfBusy(_ action: String) -> Bool {
        guard run?.isBusy == true else { return false }
        statusMessage = "Stop the running task before \(action)."
        return true
    }

    public func newConversation() {
        guard !refuseIfBusy("starting a new conversation") else { return }
        invalidateConversationSelection()
        run?.reset()
        rewindPoints = []
        rewindPointsLoadState = .loaded
    }

    public func fork() async {
        guard !refuseIfBusy("forking this conversation") else { return }
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            let result = try await client.fork(conversationID: conversationID)
            // Re-checked after the server call: a run can start on this same
            // conversation while fork's request is in flight, and applying
            // the result anyway would retarget the run's tracked
            // conversation out from under it mid-turn. The server-side fork
            // already happened either way -- only the local rebind is
            // skipped.
            guard !refuseIfBusy("forking this conversation") else { return }
            run?.rebind(conversationID: result.conversationID)
            statusMessage = "Forked into a new conversation"
            await refreshConversations()
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    public func undo(count: Int = 1) async {
        guard !refuseIfBusy("undoing the last turn") else { return }
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            try await client.undo(conversationID: conversationID, count: count)
            // Re-checked after the server call, same reasoning as `fork`
            // above: a run started mid-flight must not have its
            // conversation reloaded out from under it.
            guard !refuseIfBusy("undoing the last turn") else { return }
            await openConversationByID(conversationID)
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    /// Restores files and truncates history. Destructive; `force` overrides the
    /// server's refusal when a file changed outside the harness.
    ///
    /// Cleared at the start of every call -- including this one's own retry --
    /// so a stale refusal from a previous point can never be mistaken for one
    /// on the point this call is now acting on. Never auto-retried with
    /// `force`: setting `rewindRefusal` only records the refusal for the UI to
    /// present a distinct, explicit second confirmation (R7).
    public func rewind(to point: RewindPoint, force: Bool = false) async {
        guard !refuseIfBusy("rewinding this conversation") else { return }
        rewindRefusal = nil
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            let result = try await client.rewind(
                conversationID: conversationID, pointID: point.id, force: force)
            // A run can become active while the destructive request is in
            // flight. Its eventual persisted work must never be replaced by
            // the historical reload below.
            guard !refuseIfBusy("rewinding this conversation") else { return }
            statusMessage =
                "Restored \(result.filesRestored) file(s), removed \(result.messagesTruncated) message(s)"
            await openConversationByID(conversationID)
        } catch let error as HarnessError where error.code == "rewind_refused" {
            rewindRefusal = RewindRefusal(point: point, message: error.message)
        } catch let error as HarnessError {
            statusMessage = error.message
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    /// Dismisses a `rewind_refused` refusal without contacting the server --
    /// the "Cancel" path on the force-rewind confirmation. A refusal is a UI
    /// presentation concern once recorded; declining it performs nothing.
    public func dismissRewindRefusal() {
        rewindRefusal = nil
    }

    public func setProviderKey(provider: String, key: String) async {
        guard let client else { return }
        do {
            try await client.setProviderKey(provider: provider, key: key)
            statusMessage = "Saved key for \(provider)"
            await refreshCatalog()
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    public func importSubscription(provider: String) async {
        guard let client else { return }
        do {
            try await client.importSubscription(provider: provider)
            statusMessage = "Imported \(provider) credential"
            await refreshCatalog()
        } catch let error as HarnessError {
            // The credential is read from the daemon's own host, so this fails
            // whenever the vendor CLI was logged in somewhere else.
            statusMessage =
                "\(error.message) — log in with the vendor CLI on the machine running harnessd."
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    public func exportTranscript(to destination: URL) async {
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            let data = try await client.exportConversation(id: conversationID)
            try data.write(to: destination)
            statusMessage = "Exported to \(destination.lastPathComponent)"
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    private func openConversationByID(_ id: String) async {
        guard !refuseIfBusy("reloading this conversation") else { return }
        await loadConversation(id: id, reportFailure: false)
    }

    private func loadConversation(id: String, reportFailure: Bool) async {
        guard let client else { return }
        let generation = beginConversationSelection(id)
        let requestedConnection = connectionGeneration
        do {
            let messages = try await client.messages(conversationID: id)
            guard
                ownsConversationSelection(
                    generation, id: id, connectionGeneration: requestedConnection)
            else { return }
            // This request still owns the pending-selection slot even when a
            // run began while it was awaiting the server. Release that slot
            // before refusing the switch, or later durable syncs remain
            // blocked after the run ends.
            pendingConversationSelectionID = nil
            guard !refuseIfBusy("switching conversations") else { return }
            run?.load(messages: messages, conversationID: id)
            await refreshRewindPoints()
        } catch {
            guard
                ownsConversationSelection(
                    generation, id: id, connectionGeneration: requestedConnection)
            else { return }
            pendingConversationSelectionID = nil
            if reportFailure { statusMessage = error.localizedDescription }
        }
    }

    private func beginConversationSelection(_ id: String) -> Int {
        conversationSelectionGeneration &+= 1
        pendingConversationSelectionID = id
        return conversationSelectionGeneration
    }

    private func invalidateConversationSelection() {
        conversationSelectionGeneration &+= 1
        pendingConversationSelectionID = nil
    }

    private func ownsConversationSelection(
        _ generation: Int, id: String, connectionGeneration: Int
    ) -> Bool {
        self.connectionGeneration == connectionGeneration
            && conversationSelectionGeneration == generation
            && pendingConversationSelectionID == id
    }
}
