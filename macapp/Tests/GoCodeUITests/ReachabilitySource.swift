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

    /// Walks `Sources/GoCodeUI` recursively so subdirectories (e.g.
    /// `DesignSystem/`) are scanned too -- a flat `contentsOfDirectory` here
    /// used to see only top-level files, silently exempting anything moved
    /// into a subdirectory from every module-wide reachability assertion.
    static func wholeModule(testFilePath: String = #filePath) throws -> String {
        let directory = sourcesDirectory(from: testFilePath)
        guard
            let enumerator = FileManager.default.enumerator(
                at: directory, includingPropertiesForKeys: nil)
        else {
            throw CocoaError(.fileReadNoSuchFile)
        }
        return
            try enumerator
            .compactMap { $0 as? URL }
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
