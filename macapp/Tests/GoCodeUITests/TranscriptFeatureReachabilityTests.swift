import Foundation
import Testing

@Suite("Transcript feature reachability")
struct TranscriptFeatureReachabilityTests {

    @Test("usage and whole-conversation copy retain production call sites")
    func featuresHaveProductionCallSites() throws {
        let sourceDirectory = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
        let source = try FileManager.default
            .contentsOfDirectory(at: sourceDirectory, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "swift" }
            .map { try String(contentsOf: $0, encoding: .utf8) }
            .joined(separator: "\n")

        #expect(source.contains("UsageLabel(usage: usage)"))
        #expect(source.contains("TranscriptText.plain(items)"))
    }

    @Test("rail selection and user prompts retain their semantic layout tokens")
    func transcriptAndRailUseSemanticTokens() throws {
        let sourceDirectory = URL(filePath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appending(path: "Sources/GoCodeUI")
        let appShell = try String(
            contentsOf: sourceDirectory.appending(path: "AppShell.swift"), encoding: .utf8)
        let chatView = try String(
            contentsOf: sourceDirectory.appending(path: "ChatView.swift"), encoding: .utf8)

        #expect(appShell.contains("Theme.selectedRowSurface"))
        #expect(appShell.contains("Theme.selectedRowForeground"))
        #expect(appShell.contains("Theme.surface.ignoresSafeArea(.container, edges: .top)"))
        #expect(chatView.contains("Layout.userMessageMaximumWidth"))
        #expect(chatView.contains(".foregroundStyle(Theme.foreground)"))
    }
}
