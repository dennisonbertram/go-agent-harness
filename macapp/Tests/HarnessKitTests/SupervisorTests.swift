import Foundation
import Testing

@testable import HarnessKit

/// Writes an executable stand-in for harnessd: a trivial HTTP server that
/// answers `/healthz`. Using a stub rather than the real binary keeps these
/// tests hermetic — they exercise supervisor logic (port selection, health
/// polling, termination), not harnessd itself.
private func makeStubServer(
    healthyAfter delay: Double = 0, exitImmediately: Bool = false
) throws -> URL {
    let dir = URL(fileURLWithPath: NSTemporaryDirectory())
        .appending(path: "supervisor-stub-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    let script = dir.appending(path: "stub-harnessd")

    let body =
        exitImmediately
        ? """
        #!/bin/sh
        echo "stub failed to bind" >&2
        exit 1
        """
        : """
        #!/bin/sh
        exec /usr/bin/python3 -c '
        import os, sys, time, http.server
        time.sleep(\(delay))
        addr = os.environ["HARNESS_ADDR"]
        host, port = addr.split(":")
        class H(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path == "/healthz":
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(b"{\\"status\\":\\"ok\\"}")
                else:
                    self.send_response(404); self.end_headers()
            def log_message(self, *a): pass
        http.server.HTTPServer((host, int(port)), H).serve_forever()
        '
        """

    try body.write(to: script, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: script.path)
    return script
}

@Suite("HarnessSupervisor", .serialized)
struct SupervisorTests {

    @Test("starts a server, reports its base URL, and stops it")
    func startsAndStops() async throws {
        let supervisor = HarnessSupervisor(
            binary: try makeStubServer(),
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()))

        let baseURL = try await supervisor.start()
        #expect(baseURL.scheme == "http")
        #expect(try #require(baseURL.port) > 0)
        #expect(await supervisor.isRunning)

        // Prove it is genuinely serving, not merely spawned.
        let (data, _) = try await URLSession.shared.data(
            from: baseURL.appending(path: "/healthz"))
        #expect(String(decoding: data, as: UTF8.self).contains("ok"))

        await supervisor.stop()
        #expect(await supervisor.isRunning == false)
    }

    /// A supervisor that leaks child processes fills the machine with orphaned
    /// servers over a day of opening and closing projects.
    @Test("terminates the child process on stop")
    func terminatesChild() async throws {
        let supervisor = HarnessSupervisor(
            binary: try makeStubServer(),
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()))
        let baseURL = try await supervisor.start()
        await supervisor.stop()

        // The port must stop answering once stopped.
        var request = URLRequest(url: baseURL.appending(path: "/healthz"))
        request.timeoutInterval = 2
        await #expect(throws: (any Error).self) {
            _ = try await URLSession.shared.data(for: request)
        }
    }

    @Test("waits for the server to become healthy before returning")
    func waitsForHealth() async throws {
        let supervisor = HarnessSupervisor(
            binary: try makeStubServer(healthyAfter: 1.0),
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()))

        let baseURL = try await supervisor.start()
        // start() must not return until /healthz answers.
        let (data, _) = try await URLSession.shared.data(
            from: baseURL.appending(path: "/healthz"))
        #expect(String(decoding: data, as: UTF8.self).contains("ok"))
        await supervisor.stop()
    }

    /// A silent failure here looks to the user like an app that just does
    /// nothing, so the child's stderr must reach the error.
    @Test("surfaces the child's stderr when it dies during startup")
    func surfacesStartupFailure() async throws {
        let supervisor = HarnessSupervisor(
            binary: try makeStubServer(exitImmediately: true),
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()),
            healthTimeout: .seconds(5))

        await #expect(throws: SupervisorError.self) {
            _ = try await supervisor.start()
        }

        do {
            _ = try await supervisor.start()
        } catch let error as SupervisorError {
            #expect(error.details.contains("stub failed to bind"))
        }
        #expect(await supervisor.isRunning == false)
    }

    @Test("assigns a distinct port to each project")
    func assignsDistinctPorts() async throws {
        let a = HarnessSupervisor(
            binary: try makeStubServer(),
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()))
        let b = HarnessSupervisor(
            binary: try makeStubServer(),
            workspace: URL(fileURLWithPath: NSTemporaryDirectory()))

        let first = try await a.start()
        let second = try await b.start()
        #expect(first.port != second.port)

        await a.stop()
        await b.stop()
    }

    /// Without this the daemon answers 501 for every conversation route, so
    /// sessions, fork, undo and rewind silently do nothing in the app.
    @Test("configures a conversation store for the project")
    func configuresConversationStore() async throws {
        let workspace = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "proj-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: workspace, withIntermediateDirectories: true)

        let supervisor = HarnessSupervisor(binary: try makeStubServer(), workspace: workspace)
        _ = try await supervisor.start()
        let path = await supervisor.environment["HARNESS_CONVERSATION_DB"]
        #expect(path?.hasPrefix(workspace.path) == true)
        #expect(path?.hasSuffix("conversations.db") == true)
        await supervisor.stop()
    }

    @Test("passes the workspace to the child so it serves the right project")
    func passesWorkspace() async throws {
        let workspace = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "proj-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: workspace, withIntermediateDirectories: true)

        let supervisor = HarnessSupervisor(binary: try makeStubServer(), workspace: workspace)
        _ = try await supervisor.start()
        #expect(await supervisor.environment["HARNESS_WORKSPACE"] == workspace.path)
        await supervisor.stop()
    }
}

@Suite("harnessd resource resolution")
struct SupervisorResourceTests {

    /// harnessd resolves its prompt catalog relative to its working directory,
    /// which for a supervised server is the user's project. Without an explicit
    /// `HARNESS_PROMPTS_DIR` the server exits at startup for every workspace
    /// outside the harness repo.
    @Test("finds the prompts directory by walking up from the binary")
    func findsPromptsNearBinary() throws {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "install-\(UUID().uuidString)")
        let prompts = root.appending(path: "prompts")
        let binDirectory = root.appending(path: "bin")
        try FileManager.default.createDirectory(at: prompts, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDirectory, withIntermediateDirectories: true)
        try "catalog: {}".write(
            to: prompts.appending(path: "catalog.yaml"), atomically: true, encoding: .utf8)

        let found = HarnessSupervisor.findPromptsDirectory(
            near: binDirectory.appending(path: "harnessd"))
        #expect(found?.path == prompts.path)
    }

    @Test("returns nil when no prompt catalog is installed")
    func returnsNilWithoutCatalog() throws {
        let empty = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "empty-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: empty, withIntermediateDirectories: true)
        #expect(
            HarnessSupervisor.findPromptsDirectory(near: empty.appending(path: "harnessd")) == nil)
    }
}

extension SupervisorResourceTests {
    /// An empty model catalog is worse than a startup failure: the app comes up
    /// with no models and no explanation.
    @Test("pins the model and pricing catalogs to the installation")
    func pinsCatalogPaths() throws {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "install-\(UUID().uuidString)")
        let prompts = root.appending(path: "prompts")
        let catalog = root.appending(path: "catalog")
        try FileManager.default.createDirectory(at: prompts, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: catalog, withIntermediateDirectories: true)
        try "catalog: {}".write(
            to: prompts.appending(path: "catalog.yaml"), atomically: true, encoding: .utf8)
        try "{}".write(
            to: catalog.appending(path: "models.json"), atomically: true, encoding: .utf8)
        try "{}".write(
            to: catalog.appending(path: "pricing.json"), atomically: true, encoding: .utf8)

        let environment = HarnessSupervisor.installationEnvironment(
            for: root.appending(path: "harnessd"))
        #expect(environment["HARNESS_PROMPTS_DIR"] == prompts.path)
        #expect(
            environment["HARNESS_MODEL_CATALOG_PATH"] == catalog.appending(path: "models.json").path
        )
        #expect(
            environment["HARNESS_PRICING_CATALOG_PATH"]
                == catalog.appending(path: "pricing.json").path)
    }
}
