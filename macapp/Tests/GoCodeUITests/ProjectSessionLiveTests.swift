import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Path to a real `harnessd`. Built by `scripts/build-harnessd.sh`.
private let harnessBinary: URL? = {
    guard let path = ProcessInfo.processInfo.environment["HARNESS_BINARY"] else { return nil }
    let url = URL(fileURLWithPath: path)
    return FileManager.default.isExecutableFile(atPath: url.path) ? url : nil
}()

/// Exercises the full stack the app actually uses: spawn a real harnessd for a
/// real directory, run a prompt through it, and shut it down.
///
/// This is the only test that covers process supervision against the real
/// binary — the supervisor unit tests use a stub server.
@Suite("ProjectSession against a spawned harnessd", .serialized)
@MainActor
struct ProjectSessionLiveTests {

    private func makeWorkspace() throws -> URL {
        let url = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "gocode-project-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        try "hello".write(to: url.appending(path: "README.md"), atomically: true, encoding: .utf8)
        return url
    }

    private func wait(
        timeout: Duration = .seconds(60), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(100))
        }
        Issue.record("timed out")
    }

    @Test(
        "starts its own server, runs a prompt, and shuts down", .enabled(if: harnessBinary != nil))
    func fullProjectLifecycle() async throws {
        let workspace = try makeWorkspace()
        let project = ProjectSession(workspace: workspace)

        #expect(project.phase == .idle)
        await project.start()

        if case .failed(let message) = project.phase {
            Issue.record("server failed to start: \(message)")
            return
        }
        #expect(project.phase == .ready)

        // The catalog is fetched on connect; it proves the client reached the
        // server this project spawned, not some other one.
        try await wait(timeout: .seconds(20)) { !project.models.isEmpty }
        #expect(project.models.count > 100)

        let run = try #require(project.run)
        run.draft = "list the workspace"
        project.submit()

        try await wait { !run.isBusy }
        #expect(
            run.transcript.runState == .completed,
            "run ended as \(run.transcript.runState): \(run.connectionError ?? "no error")")
        #expect(run.transcript.items.count >= 2)
        #expect(run.conversationID != nil)

        await project.shutdown()
        #expect(project.phase == .idle)
    }

    /// A supervisor that leaks servers fills the machine over a day of work.
    @Test("stops its server on shutdown", .enabled(if: harnessBinary != nil))
    func shutdownStopsServer() async throws {
        let project = ProjectSession(workspace: try makeWorkspace())
        await project.start()
        guard project.phase == .ready else {
            Issue.record("server did not start")
            return
        }

        let before = try harnessdProcessCount()
        await project.shutdown()
        // Give the child a moment to actually exit after SIGTERM.
        try await Task.sleep(for: .seconds(2))
        let after = try harnessdProcessCount()

        #expect(after < before, "harnessd still running after shutdown (\(before) → \(after))")
    }

    private func harnessdProcessCount() throws -> Int {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/pgrep")
        process.arguments = ["-f", harnessBinary?.path ?? "harnessd"]
        let pipe = Pipe()
        process.standardOutput = pipe
        try process.run()
        process.waitUntilExit()
        let output = String(
            decoding: pipe.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        return output.split(separator: "\n").count
    }

    @Test("reports a clear error when the binary is missing")
    func missingBinaryIsExplained() async throws {
        // No HARNESS_BINARY override and a path with no harnessd on it.
        let project = ProjectSession(workspace: try makeWorkspace())
        // Only meaningful when the environment genuinely has no harnessd; skip
        // otherwise rather than assert a false expectation.
        guard HarnessBinary.locate() == nil else { return }

        await project.start()
        guard case .failed(let message) = project.phase else {
            Issue.record("expected a failed phase")
            return
        }
        #expect(message.contains("harnessd"))
    }
}
