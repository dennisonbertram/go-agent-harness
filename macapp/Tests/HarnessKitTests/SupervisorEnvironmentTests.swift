import Foundation
import Testing

@testable import HarnessKit

/// Workflow authoring compiles Go against the harness module. Without the module
/// root the tools do not degrade, they fail outright — and nothing else in the
/// app's environment supplies it, so a missing value here means every
/// app-launched daemon cannot create or run a workflow at all.
@Suite("Supervisor installation environment")
struct SupervisorEnvironmentTests {
    private func makeInstallation() throws -> URL {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "supervisor-env-\(UUID().uuidString)")
        let fm = FileManager.default
        try fm.createDirectory(
            at: root.appending(path: "prompts"), withIntermediateDirectories: true)
        try "".write(
            to: root.appending(path: "prompts").appending(path: "catalog.yaml"),
            atomically: true, encoding: .utf8)
        try "module go-agent-harness\n".write(
            to: root.appending(path: "go.mod"),
            atomically: true, encoding: .utf8)
        try fm.createDirectory(at: root.appending(path: "bin"), withIntermediateDirectories: true)
        return root
    }

    @Test("the module root is passed to the daemon")
    func passesSourceRoot() throws {
        let root = try makeInstallation()
        defer { try? FileManager.default.removeItem(at: root) }
        let binary = root.appending(path: "bin").appending(path: "harnessd")
        let env = HarnessSupervisor.installationEnvironment(for: binary)
        #expect(env["HARNESS_SOURCE_ROOT"] == root.path)
    }

    @Test("an installation without the module is not claimed to have one")
    func omitsSourceRootWhenAbsent() throws {
        let root = try makeInstallation()
        defer { try? FileManager.default.removeItem(at: root) }
        try FileManager.default.removeItem(at: root.appending(path: "go.mod"))
        let binary = root.appending(path: "bin").appending(path: "harnessd")
        let env = HarnessSupervisor.installationEnvironment(for: binary)
        #expect(env["HARNESS_SOURCE_ROOT"] == nil)
    }
}
