import Foundation

/// Shared source-scan helpers for reachability tests (`TranscriptFeatureReachabilityTests`,
/// `AccessibilityReachabilityTests`): read one named file from `Sources/GoCodeUI`, or every
/// `.swift` file there joined together. These tests assert production source contains (or
/// no longer contains) a literal shape, since alert presentation, key handling, and
/// accessibility traits are not otherwise assertable through a headless test on this stack.
///
/// `testFilePath` defaults to `#filePath` evaluated at the *call site* (the standard
/// `#filePath`-as-default-parameter idiom), so each caller resolves paths relative to its
/// own file without repeating the `deletingLastPathComponent()` walk up to
/// `Tests/GoCodeUITests` itself.
enum ReachabilitySource {
    static func file(_ name: String, testFilePath: String = #filePath) throws -> String {
        try String(
            contentsOf: sourcesDirectory(from: testFilePath).appending(path: name),
            encoding: .utf8)
    }

    static func wholeModule(testFilePath: String = #filePath) throws -> String {
        let directory = sourcesDirectory(from: testFilePath)
        return try FileManager.default
            .contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "swift" }
            .map { try String(contentsOf: $0, encoding: .utf8) }
            .joined(separator: "\n")
    }

    private static func sourcesDirectory(from testFilePath: String) -> URL {
        URL(filePath: testFilePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
    }
}
