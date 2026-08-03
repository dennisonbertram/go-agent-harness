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
        run?.cancel()
        run?.stopConversationStream()
        await supervisor?.stop()
        supervisor = nil
        client = nil
        phase = .idle
    }

    private func connect(to baseURL: URL) {
        let client = HarnessClient(baseURL: baseURL)
        self.client = client
        self.run = RunSession(client: client)
        phase = .ready
        Task { await refreshCatalog() }
        Task { await refreshConversations() }
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
        guard let client else { return }
        modelsLoadState = .loading
        providersLoadState = .loading
        profilesLoadState = .loading
        async let models = try await client.models()
        async let providers = try await client.providers()
        async let profiles = try await client.profiles()
        do {
            self.models = try await models
            modelsLoadState = .loaded
        } catch {
            modelsLoadState = .failed
            statusMessage = error.localizedDescription
        }
        do {
            self.providers = try await providers
            providersLoadState = .loaded
        } catch {
            providersLoadState = .failed
            statusMessage = error.localizedDescription
        }
        do {
            self.profiles = try await profiles
            profilesLoadState = .loaded
        } catch {
            profilesLoadState = .failed
            statusMessage = error.localizedDescription
        }
    }

    public func refreshConversations() async {
        guard let client else { return }
        conversationsLoadState = .loading
        do {
            conversations = try await client.conversations(limit: 100)
            conversationsLoadState = .loaded
        } catch {
            conversationsLoadState = .failed
            statusMessage = error.localizedDescription
        }
    }

    public func refreshRewindPoints() async {
        guard let client, let conversationID = run?.conversationID else {
            rewindPoints = []
            rewindPointsLoadState = .loaded
            return
        }
        rewindPointsLoadState = .loading
        do {
            rewindPoints = try await client.rewindPoints(conversationID: conversationID)
            rewindPointsLoadState = .loaded
        } catch {
            rewindPointsLoadState = .failed
            statusMessage = error.localizedDescription
        }
    }

    public func refreshActivity() async {
        guard let client else { return }
        tasksLoadState = .loading
        runsLoadState = .loading
        do {
            tasks = try await client.tasks()
            tasksLoadState = .loaded
        } catch {
            tasksLoadState = .failed
            statusMessage = error.localizedDescription
        }
        do {
            // `client.runs()` already turns the deliberate "no run store
            // configured" 501 into `nil`; anything thrown here is a genuine
            // transport/server failure and must not be folded into that same
            // nil, or a network blip reads as "no run store configured" — a
            // lie about the daemon's configuration (#951 finding 3).
            runs = try await client.runs()
            runsLoadState = .loaded
        } catch {
            runsLoadState = .failed
            statusMessage = error.localizedDescription
        }
        if let runID = run?.currentRunID {
            todosLoadState = .loading
            do {
                todos = try await client.todos(runID: runID)
                todosLoadState = .loaded
            } catch {
                todosLoadState = .failed
                statusMessage = error.localizedDescription
            }
        } else {
            todos = []
            todosLoadState = .loaded
        }
    }

    /// Rehydrates the selected conversation from durable messages when Chat
    /// becomes visible. Conversation SSE remains the low-latency path; this is
    /// the durability safety net for a completed scheduled run that happened
    /// across a dropped/recreated stream or while the view was elsewhere.
    public func syncCurrentConversation() async {
        guard
            let client,
            let run,
            let conversationID = run.conversationID,
            !run.isBusy
        else { return }
        do {
            let messages = try await client.messages(conversationID: conversationID)
            run.reconcilePersistedMessages(messages)
        } catch {
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

    @discardableResult
    public func submit() -> RunSubmission? {
        run?.model = selectedModel
        run?.planMode = planMode
        run?.extraDirs = extraDirs.map(\.path)
        run?.profile = selectedProfile
        let submission = run?.submit()
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
        return submission
    }

    public func openConversation(_ conversation: ConversationInfo) async {
        guard let client else { return }
        do {
            let messages = try await client.messages(conversationID: conversation.id)
            run?.load(messages: messages, conversationID: conversation.id)
            await refreshRewindPoints()
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    public func deleteConversation(_ conversation: ConversationInfo) async {
        guard let client else { return }
        do {
            try await client.deleteConversation(id: conversation.id)
            if run?.conversationID == conversation.id { newConversation() }
            await refreshConversations()
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    public func newConversation() {
        run?.reset()
        rewindPoints = []
        rewindPointsLoadState = .loaded
    }

    public func fork() async {
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            let result = try await client.fork(conversationID: conversationID)
            run?.rebind(conversationID: result.conversationID)
            statusMessage = "Forked into a new conversation"
            await refreshConversations()
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    public func undo(count: Int = 1) async {
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            try await client.undo(conversationID: conversationID, count: count)
            await openConversationByID(conversationID)
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    /// Restores files and truncates history. Destructive; `force` overrides the
    /// server's refusal when a file changed outside the harness.
    public func rewind(to point: RewindPoint, force: Bool = false) async {
        guard let client, let conversationID = run?.conversationID else { return }
        do {
            let result = try await client.rewind(
                conversationID: conversationID, pointID: point.id, force: force)
            statusMessage =
                "Restored \(result.filesRestored) file(s), removed \(result.messagesTruncated) message(s)"
            await openConversationByID(conversationID)
        } catch let error as HarnessError {
            statusMessage = error.message
        } catch {
            statusMessage = error.localizedDescription
        }
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
        guard let client else { return }
        if let messages = try? await client.messages(conversationID: id) {
            run?.load(messages: messages, conversationID: id)
        }
        await refreshRewindPoints()
    }
}
