import Foundation
import GoCodeUI
import HarnessKit

enum ToolWalkError: Error {
    case repoRootNotFound
}

/// Headlessly walks every registered tool through the app's own client and
/// transcript code: `ProjectSession`/`RunSession`/`HarnessClient` from
/// `GoCodeUI`/`HarnessKit`, the same objects the composer's Send button
/// drives. No `URLRequest` of its own — see scripts/ui-walk-tools.txt for the
/// per-tool prompts and CLAUDE.md context for why this exists (the target
/// machine's screen is locked, so an accessibility-driven walk cannot run).
@main
@MainActor
struct ToolWalkMain {
    static func main() async throws {
        let environment = ProcessInfo.processInfo.environment
        let repoRoot = try resolveRepoRoot()
        let toolsFile = repoRoot.appending(path: "scripts").appending(path: "ui-walk-tools.txt")
        let specs = try parseToolSpecs(String(contentsOf: toolsFile, encoding: .utf8))
        print("[toolwalk] \(specs.count) tools to walk, from \(toolsFile.path)")

        let workspace =
            environment["TOOLWALK_WORKSPACE"].map(URL.init(fileURLWithPath:))
            ?? URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "toolwalk-\(UUID().uuidString)")
        try seedWorkspace(at: workspace)
        print("[toolwalk] workspace: \(workspace.path)")

        guard HarnessBinary.locate() != nil else {
            print("[toolwalk] FATAL: could not locate a harnessd binary. Set HARNESS_BINARY.")
            exit(1)
        }

        let project = ProjectSession(workspace: workspace)
        await project.start()
        guard project.phase == .ready else {
            print("[toolwalk] FATAL: harnessd did not start: \(project.phase)")
            exit(1)
        }
        project.selectedModel = environment["TOOLWALK_MODEL"] ?? "gpt-oss-120b"

        let timeoutSeconds = environment["TOOLWALK_TIMEOUT_SECONDS"].flatMap { Int($0) } ?? 240
        let config = RunnerConfig(
            timeoutPerTool: .seconds(timeoutSeconds), pollInterval: .milliseconds(200))

        let results = await Runner.walk(project: project, specs: specs, config: config)
        await project.shutdown()

        let resultsPath = environment["TOOLWALK_RESULTS_PATH"] ?? "/tmp/toolwalk-results.json"
        try writeResults(results, to: URL(fileURLWithPath: resultsPath))
        print("[toolwalk] results written to \(resultsPath)")

        printSummary(results)
        exit(results.allSatisfy { $0.verdict == "pass" } ? 0 : 1)
    }
}

/// Finds the repo root by walking up for `scripts/ui-walk-tools.txt`, mirroring
/// how `HarnessSupervisor` walks up for `prompts/catalog.yaml` — so the binary
/// works when invoked from anywhere inside the checkout, not only its root.
private func resolveRepoRoot(fileManager: FileManager = .default) throws -> URL {
    var directory = URL(fileURLWithPath: fileManager.currentDirectoryPath)
    for _ in 0..<8 {
        let candidate = directory.appending(path: "scripts").appending(path: "ui-walk-tools.txt")
        if fileManager.fileExists(atPath: candidate.path) { return directory }
        let parent = directory.deletingLastPathComponent()
        if parent.path == directory.path { break }
        directory = parent
    }
    throw ToolWalkError.repoRootNotFound
}

private func writeResults(_ results: [ToolResult], to url: URL) throws {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
    try encoder.encode(results).write(to: url)
}

private func printSummary(_ results: [ToolResult]) {
    let nameWidth = results.map { $0.name.count }.max() ?? 0
    print("\n[toolwalk] RESULTS")
    print(String(repeating: "-", count: 72))
    for result in results {
        let mark = result.verdict == "pass" ? "PASS" : "FAIL"
        let paddedName = result.name.padding(toLength: nameWidth, withPad: " ", startingAt: 0)
        print("\(mark)  \(paddedName)  \(result.reply.prefix(80))")
    }
    print(String(repeating: "-", count: 72))
    let passCount = results.filter { $0.verdict == "pass" }.count
    print("[toolwalk] \(passCount)/\(results.count) tools passed")
}
