import Foundation
import Testing

@testable import GoCodeUI

/// A `FileManager` that reports a fixed current directory and treats one
/// extra path as executable. Both the real cwd and `PATH` are process-global,
/// so faking them through the `fileManager` seam this function already takes
/// keeps the test hermetic instead of mutating global state other tests
/// might read concurrently.
private final class FakeLocateFileManager: FileManager, @unchecked Sendable {
    let fixedCurrentDirectory: String
    let extraExecutablePath: String

    init(fixedCurrentDirectory: String, extraExecutablePath: String) {
        self.fixedCurrentDirectory = fixedCurrentDirectory
        self.extraExecutablePath = extraExecutablePath
    }

    override var currentDirectoryPath: String { fixedCurrentDirectory }

    override func isExecutableFile(atPath path: String) -> Bool {
        path == extraExecutablePath || super.isExecutableFile(atPath: path)
    }
}

/// `HarnessBinary.locate`'s doc comment always promised "explicit override,
/// then PATH, then a repo-local build", but no repo-local fallback existed.
/// This is not theoretical: a PATH-installed `harnessd` has no `prompts/`
/// directory above it, so the daemon boots and immediately exits — the app
/// launched a server that was already dead (#951 finding 11).
@Suite("HarnessBinary.locate")
struct HarnessBinaryTests {

    @Test(
        "prefers a bootable repo-local build over a PATH entry that cannot boot",
        .enabled(if: ProcessInfo.processInfo.environment["HARNESS_BINARY"] == nil)
    )
    func prefersBootableRepoLocalBuild() throws {
        let fm = FileManager.default
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "gocode-locate-\(UUID().uuidString)")
        defer { try? fm.removeItem(at: root) }

        // A repo-local build under `.harnessd-bin/` — what `live-test.sh`
        // produces — with `prompts/catalog.yaml` resolvable above it.
        let binDir = root.appending(path: ".harnessd-bin")
        try fm.createDirectory(at: binDir, withIntermediateDirectories: true)
        let repoLocalBinary = binDir.appending(path: "harnessd")
        try Data().write(to: repoLocalBinary)
        try fm.setAttributes([.posixPermissions: 0o755], ofItemAtPath: repoLocalBinary.path)
        let promptsDir = root.appending(path: "prompts")
        try fm.createDirectory(at: promptsDir, withIntermediateDirectories: true)
        try Data().write(to: promptsDir.appending(path: "catalog.yaml"))

        // A PATH entry that the fake FileManager reports as executable, but
        // with no prompts/ directory anywhere above it — it cannot boot.
        let firstPathDir = try #require(
            ProcessInfo.processInfo.environment["PATH"]?.split(separator: ":").first
                .map(String.init))
        let unbootablePathCandidate = firstPathDir + "/harnessd"

        let fakeFileManager = FakeLocateFileManager(
            fixedCurrentDirectory: root.path, extraExecutablePath: unbootablePathCandidate)

        let located = HarnessBinary.locate(fileManager: fakeFileManager)
        #expect(located?.path == repoLocalBinary.path)
    }

    /// Regression angle: when nothing anywhere is bootable, `locate()` must
    /// still fall back to the first candidate it found (old PATH-only
    /// behaviour) rather than giving up and returning nil — a real harnessd
    /// that pins its own prompts directory via `HARNESS_PROMPTS_DIR` boots
    /// fine even without a nearby `prompts/catalog.yaml`.
    @Test(
        "falls back to the PATH candidate when no candidate is bootable",
        .enabled(if: ProcessInfo.processInfo.environment["HARNESS_BINARY"] == nil)
    )
    func fallsBackToPathCandidateWhenNoneBoot() throws {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "gocode-locate-empty-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }

        let firstPathDir = try #require(
            ProcessInfo.processInfo.environment["PATH"]?.split(separator: ":").first
                .map(String.init))
        let pathCandidate = firstPathDir + "/harnessd"

        let fakeFileManager = FakeLocateFileManager(
            fixedCurrentDirectory: root.path, extraExecutablePath: pathCandidate)

        let located = HarnessBinary.locate(fileManager: fakeFileManager)
        #expect(located?.path == pathCandidate)
    }
}
