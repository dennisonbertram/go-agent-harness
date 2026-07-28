import Foundation

enum WorkspaceError: Error {
    case commandFailed(command: String, status: Int32, stderr: String)
}

/// Seeds a throwaway workspace with real git history and a `go.mod` so the
/// git/file tools being walked have something to act on — an empty directory
/// makes prompts like "grep for 'module' in go.mod" or "the most recent
/// commit touching go.mod" nonsensical.
///
/// Two commits, not one: `git_diff_range`'s prompt asks for `HEAD~1..HEAD`,
/// which is not a valid range with only one commit. The first commit's
/// message itself is the fixture for `git_log_search`, whose prompt searches
/// commits for "seed".
func seedWorkspace(at directory: URL) throws {
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)

    let goMod = "module toolwalk\n\ngo 1.21\n"
    try goMod.write(to: directory.appending(path: "go.mod"), atomically: true, encoding: .utf8)

    try runGit(["init", "--quiet"], in: directory)
    try runGit(["config", "user.email", "toolwalk@example.com"], in: directory)
    try runGit(["config", "user.name", "ToolWalk"], in: directory)
    try runGit(["add", "go.mod"], in: directory)
    try runGit(["commit", "--quiet", "-m", "seed: initial commit"], in: directory)

    let readme = "# ToolWalk scratch workspace\n"
    try readme.write(
        to: directory.appending(path: "README.md"), atomically: true, encoding: .utf8)
    try runGit(["add", "README.md"], in: directory)
    try runGit(["commit", "--quiet", "-m", "docs: add readme"], in: directory)
}

@discardableResult
private func runGit(_ arguments: [String], in directory: URL) throws -> String {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = ["git"] + arguments
    process.currentDirectoryURL = directory

    let stdout = Pipe()
    let stderr = Pipe()
    process.standardOutput = stdout
    process.standardError = stderr

    try process.run()
    process.waitUntilExit()

    guard process.terminationStatus == 0 else {
        let errorText = String(
            decoding: stderr.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        throw WorkspaceError.commandFailed(
            command: "git " + arguments.joined(separator: " "),
            status: process.terminationStatus, stderr: errorText)
    }
    return String(decoding: stdout.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
}
