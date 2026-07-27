import Foundation
import HarnessKit
import Observation

/// Locates the `harnessd` binary to supervise.
public enum HarnessBinary {
    /// Resolution order: explicit override, then `PATH`, then a repo-local
    /// build — which is what a developer running from source has.
    public static func locate(fileManager: FileManager = .default) -> URL? {
        if let override = ProcessInfo.processInfo.environment["HARNESS_BINARY"] {
            let url = URL(fileURLWithPath: override)
            if fileManager.isExecutableFile(atPath: url.path) { return url }
        }
        for directory in ProcessInfo.processInfo.environment["PATH"]?.split(separator: ":") ?? [] {
            let candidate = URL(fileURLWithPath: String(directory)).appending(path: "harnessd")
            if fileManager.isExecutableFile(atPath: candidate.path) { return candidate }
        }
        return nil
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
    public private(set) var models: [ModelInfo] = []
    public private(set) var providers: [ProviderInfo] = []
    public private(set) var rewindPoints: [RewindPoint] = []
    public private(set) var statusMessage: String?

    /// Model applied to the next run; nil uses the server's default.
    public var selectedModel: String?
    public var planMode = false

    private var supervisor: HarnessSupervisor?
    private var client: HarnessClient?
    /// Set when attaching to an externally-managed harnessd instead of spawning one.
    private let externalBaseURL: URL?

    public init(workspace: URL, externalBaseURL: URL? = nil) {
        self.workspace = workspace
        self.externalBaseURL = externalBaseURL
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

        let supervisor = HarnessSupervisor(binary: binary, workspace: workspace)
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

    public func refreshCatalog() async {
        guard let client else { return }
        async let models = try? await client.models()
        async let providers = try? await client.providers()
        self.models = await models ?? []
        self.providers = await providers ?? []
    }

    public func refreshConversations() async {
        guard let client else { return }
        // Conversation persistence is optional server-side; an unconfigured
        // store is a normal state, not an error worth showing.
        conversations = (try? await client.conversations(limit: 100)) ?? []
    }

    public func refreshRewindPoints() async {
        guard let client, let conversationID = run?.conversationID else {
            rewindPoints = []
            return
        }
        rewindPoints = (try? await client.rewindPoints(conversationID: conversationID)) ?? []
    }

    // MARK: - Actions

    public func submit() {
        run?.model = selectedModel
        run?.planMode = planMode
        run?.submit()
        Task {
            // Give the run a moment to register before listing.
            try? await Task.sleep(for: .seconds(1))
            await refreshConversations()
        }
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
