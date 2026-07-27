import Foundation
import Testing

@testable import GoCodeUI

@Suite("FileCompletion")
struct FileCompletionTests {

    private func makeTree() throws -> URL {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "complete-\(UUID().uuidString)")
        let files = [
            "README.md",
            "Sources/App/Main.swift",
            "Sources/App/Model.swift",
            "Tests/AppTests/MainTests.swift",
            "node_modules/left-pad/index.js",
            ".git/config",
        ]
        for file in files {
            let url = root.appending(path: file)
            try FileManager.default.createDirectory(
                at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try "x".write(to: url, atomically: true, encoding: .utf8)
        }
        return root
    }

    @Test("matches on a subsequence of the path")
    func matchesSubsequence() async throws {
        let completion = FileCompletion(roots: [try makeTree()])
        let matches = await completion.matches(for: "main")
        #expect(matches.contains { $0.relativePath == "Sources/App/Main.swift" })
    }

    /// Scanning `node_modules` and `.git` dominates the cost in a real repo and
    /// never produces a result anyone wants.
    @Test("skips vendored and VCS directories")
    func skipsNoise() async throws {
        let completion = FileCompletion(roots: [try makeTree()])
        let matches = await completion.matches(for: "index")
        #expect(!matches.contains { $0.relativePath.contains("node_modules") })

        let git = await completion.matches(for: "config")
        #expect(!git.contains { $0.relativePath.contains(".git") })
    }

    @Test("ranks basename matches above incidental path matches")
    func ranksBasenameFirst() async throws {
        let completion = FileCompletion(roots: [try makeTree()])
        let matches = await completion.matches(for: "model")
        #expect(matches.first?.relativePath == "Sources/App/Model.swift")
    }

    @Test("returns nothing when the query cannot match")
    func noMatches() async throws {
        let completion = FileCompletion(roots: [try makeTree()])
        #expect(await completion.matches(for: "zzzznotathing").isEmpty)
    }

    @Test("scores contiguous runs above scattered characters")
    func scoresContiguity() throws {
        let contiguous = try #require(FileCompletion.score("main.swift", "main"))
        let scattered = try #require(FileCompletion.score("m-a-i-n.swift", "main"))
        #expect(contiguous > scattered)
    }

    @Test("returns nil when a character is missing")
    func rejectsNonSubsequence() {
        #expect(FileCompletion.score("abc", "abd") == nil)
    }
}

@Suite("Composer mention parsing")
struct MentionParsingTests {

    @Test("finds an in-progress mention at the caret")
    func findsMention() {
        #expect(MentionQuery.current(in: "look at @Main") == "Main")
        #expect(MentionQuery.current(in: "@") == "")
        #expect(MentionQuery.current(in: "@Sources/App") == "Sources/App")
    }

    /// A completed mention followed by a space is finished; reopening the popup
    /// would fight the user as they keep typing.
    @Test("ignores a mention already terminated by a space")
    func ignoresFinishedMention() {
        #expect(MentionQuery.current(in: "look at @Main.swift and") == nil)
        #expect(MentionQuery.current(in: "no mention here") == nil)
        #expect(MentionQuery.current(in: "") == nil)
    }

    @Test("replaces the active mention with the chosen path")
    func replacesMention() {
        #expect(
            MentionQuery.replacing("look at @Mai", with: "Sources/App/Main.swift")
                == "look at @Sources/App/Main.swift ")
        #expect(MentionQuery.replacing("@", with: "a.txt") == "@a.txt ")
    }
}
