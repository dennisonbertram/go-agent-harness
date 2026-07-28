import Foundation

public struct SupervisorError: Error, Sendable {
    public let message: String
    /// The child's stderr, so a startup failure is diagnosable rather than
    /// presenting as an app that silently does nothing.
    public let details: String
}

extension SupervisorError: LocalizedError {
    public var errorDescription: String? {
        details.isEmpty ? message : "\(message)\n\n\(details)"
    }
}

/// Supervises one harnessd child process for one project directory.
///
/// harnessd serves a single workspace per process (`HARNESS_WORKSPACE`; there is
/// no per-run workspace field), so opening a second project means a second
/// server. An actor because start/stop race against the health poll and the
/// process's own termination handler.
public actor HarnessSupervisor {
    private let binary: URL
    private let workspace: URL
    private let healthTimeout: Duration
    private let extraEnvironment: [String: String]

    private var process: Process?
    private var stderrPipe: Pipe?
    private var baseURL: URL?

    public private(set) var environment: [String: String] = [:]

    public init(
        binary: URL,
        workspace: URL,
        healthTimeout: Duration = .seconds(30),
        extraEnvironment: [String: String] = [:]
    ) {
        self.binary = binary
        self.workspace = workspace
        self.healthTimeout = healthTimeout
        self.extraEnvironment = extraEnvironment
    }

    public var isRunning: Bool { process?.isRunning ?? false }

    /// Starts harnessd and returns its base URL once `/healthz` answers.
    @discardableResult
    public func start() async throws -> URL {
        if let baseURL, isRunning { return baseURL }

        let port = try Self.reserveFreePort()
        let url = try makeURL("http://127.0.0.1:\(port)")

        var env = ProcessInfo.processInfo.environment
        env["HARNESS_ADDR"] = "127.0.0.1:\(port)"
        env["HARNESS_WORKSPACE"] = workspace.path
        // Without a conversation store harnessd answers 501 for every
        // conversation route, so sessions, fork, undo and rewind are all dead.
        // Kept beside the project, matching harnessd's own `.harness/` state.
        if env["HARNESS_CONVERSATION_DB"] == nil {
            let harnessDirectory = workspace.appending(path: ".harness")
            try? FileManager.default.createDirectory(
                at: harnessDirectory, withIntermediateDirectories: true)
            env["HARNESS_CONVERSATION_DB"] =
                harnessDirectory.appending(path: "conversations.db").path
        }
        // Same story for runs: with no run store harnessd answers 501 on
        // /v1/runs, so the Activity screen could only ever report "this server
        // has no run store configured" — a dead panel in every app-supervised
        // daemon, since nothing else was ever going to set this.
        if env["HARNESS_RUN_DB"] == nil {
            let harnessDirectory = workspace.appending(path: ".harness")
            try? FileManager.default.createDirectory(
                at: harnessDirectory, withIntermediateDirectories: true)
            env["HARNESS_RUN_DB"] = harnessDirectory.appending(path: "runs.db").path
        }
        // harnessd resolves its prompts and model catalog relative to its
        // *working directory* and workspace — both of which are the user's
        // project here, and neither contains those files. Without pinning them
        // to the installation the server either exits at startup or comes up
        // with an empty model catalog.
        for (key, value) in Self.installationEnvironment(for: binary) where env[key] == nil {
            env[key] = value
        }
        for (key, value) in extraEnvironment { env[key] = value }
        environment = env

        let task = Process()
        task.executableURL = binary
        task.environment = env
        task.currentDirectoryURL = workspace

        let errors = Pipe()
        task.standardError = errors
        task.standardOutput = Pipe()

        do {
            try task.run()
        } catch {
            throw SupervisorError(
                message: "Could not launch harnessd at \(binary.path)",
                details: error.localizedDescription)
        }

        process = task
        stderrPipe = errors
        baseURL = url

        try await waitForHealth(at: url, process: task, errors: errors)
        return url
    }

    public func stop() async {
        guard let task = process, task.isRunning else {
            process = nil
            baseURL = nil
            return
        }
        // Cooperative shutdown first so harnessd can flush its stores.
        task.terminate()
        if await !waitForExit(task, within: .seconds(5)) {
            kill(task.processIdentifier, SIGKILL)
        }
        process = nil
        stderrPipe = nil
        baseURL = nil
    }

    // MARK: - Health

    private func waitForHealth(at url: URL, process task: Process, errors: Pipe) async throws {
        var request = URLRequest(url: url.appending(path: "/healthz"))
        request.timeoutInterval = 2

        let deadline = ContinuousClock.now.advanced(by: healthTimeout)
        while ContinuousClock.now < deadline {
            // A child that already exited will never become healthy; fail now
            // with its stderr rather than waiting out the whole timeout.
            guard task.isRunning else {
                let details = String(
                    decoding: errors.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
                self.process = nil
                self.baseURL = nil
                throw SupervisorError(
                    message: "harnessd exited during startup (code \(task.terminationStatus))",
                    details: details.trimmingCharacters(in: .whitespacesAndNewlines))
            }

            if let (_, response) = try? await URLSession.shared.data(for: request),
                (response as? HTTPURLResponse)?.statusCode == 200
            {
                return
            }
            try? await Task.sleep(for: .milliseconds(100))
        }

        await stop()
        throw SupervisorError(
            message: "harnessd did not become healthy within \(healthTimeout)",
            details: "")
    }

    private func waitForExit(_ task: Process, within timeout: Duration) async -> Bool {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if !task.isRunning { return true }
            try? await Task.sleep(for: .milliseconds(50))
        }
        return !task.isRunning
    }

    // MARK: - Resources

    /// Walks up from the binary for the harness installation — the directory
    /// holding `prompts/catalog.yaml`. Mirrors harnessd's own search, but
    /// anchored to the installation rather than to the workspace.
    static func findInstallationRoot(
        near binary: URL, fileManager: FileManager = .default
    ) -> URL? {
        var directory = binary.deletingLastPathComponent()
        for _ in 0..<8 {
            let catalog = directory.appending(path: "prompts").appending(path: "catalog.yaml")
            if fileManager.fileExists(atPath: catalog.path) { return directory }
            let parent = directory.deletingLastPathComponent()
            if parent.path == directory.path { break }
            directory = parent
        }
        return nil
    }

    static func findPromptsDirectory(
        near binary: URL, fileManager: FileManager = .default
    ) -> URL? {
        findInstallationRoot(near: binary, fileManager: fileManager)?.appending(path: "prompts")
    }

    /// Environment pinning every installation-relative resource harnessd would
    /// otherwise look for beside the user's project.
    static func installationEnvironment(
        for binary: URL, fileManager: FileManager = .default
    ) -> [String: String] {
        guard let root = findInstallationRoot(near: binary, fileManager: fileManager) else {
            return [:]
        }
        var environment = ["HARNESS_PROMPTS_DIR": root.appending(path: "prompts").path]

        let models = root.appending(path: "catalog").appending(path: "models.json")
        if fileManager.fileExists(atPath: models.path) {
            environment["HARNESS_MODEL_CATALOG_PATH"] = models.path
        }
        let pricing = root.appending(path: "catalog").appending(path: "pricing.json")
        if fileManager.fileExists(atPath: pricing.path) {
            environment["HARNESS_PRICING_CATALOG_PATH"] = pricing.path
        }
        // Workflow authoring compiles Go against the harness module, so it needs
        // the module root. Without it create_workflow and run_workflow fail
        // outright — not degraded, unavailable — for every app-launched daemon,
        // because nothing else in the app's environment supplies it.
        if fileManager.fileExists(atPath: root.appending(path: "go.mod").path) {
            environment["HARNESS_SOURCE_ROOT"] = root.path
        }
        return environment
    }

    // MARK: - Ports

    /// Binds port 0 to have the kernel allocate a free port, then releases it.
    ///
    /// There is an inherent race between releasing and the child binding, but
    /// harnessd takes the port within milliseconds and the alternative — passing
    /// a listening socket to a child that wants to bind its own — is not
    /// something harnessd supports.
    private static func reserveFreePort() throws -> Int {
        let handle = socket(AF_INET, SOCK_STREAM, 0)
        guard handle >= 0 else {
            throw SupervisorError(message: "Could not allocate a port", details: "socket() failed")
        }
        defer { close(handle) }

        var reuse: Int32 = 1
        setsockopt(handle, SOL_SOCKET, SO_REUSEADDR, &reuse, socklen_t(MemoryLayout<Int32>.size))

        var address = sockaddr_in()
        address.sin_family = sa_family_t(AF_INET)
        address.sin_port = 0
        address.sin_addr.s_addr = inet_addr("127.0.0.1")

        let bound = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                bind(handle, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
            }
        }
        guard bound == 0 else {
            throw SupervisorError(message: "Could not allocate a port", details: "bind() failed")
        }

        var length = socklen_t(MemoryLayout<sockaddr_in>.size)
        let named = withUnsafeMutablePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                getsockname(handle, $0, &length)
            }
        }
        guard named == 0 else {
            throw SupervisorError(
                message: "Could not allocate a port", details: "getsockname() failed")
        }
        return Int(UInt16(bigEndian: address.sin_port))
    }
}

/// Builds a URL from an interpolated string that is known-valid at the call
/// site, keeping `start()` free of optional juggling.
private func makeURL(_ string: String) throws -> URL {
    guard let url = URL(string: string) else {
        throw SupervisorError(message: "Invalid server URL", details: string)
    }
    return url
}
