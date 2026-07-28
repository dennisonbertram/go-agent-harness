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

@Suite("FileCompletion bounded scan")
struct FileCompletionBoundedScanTests {

    /// `count` files that all match "target" with an identical score (same
    /// contiguous prefix, same basename bonuses) but strictly increasing path
    /// length, so the expected top-`limit` is deterministic regardless of the
    /// order the filesystem enumerator happens to visit them in.
    private func makeTieBreakTree(count: Int) throws -> URL {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "bounded-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for index in 0..<count {
            let name = "target" + String(repeating: "x", count: index) + ".txt"
            try "x".write(to: root.appending(path: name), atomically: true, encoding: .utf8)
        }
        return root
    }

    /// `count` files that all match "target", named so length stays bounded
    /// (unlike `makeTieBreakTree`, which grows the name with the index).
    private func makeLargeTree(count: Int) throws -> URL {
        let root = URL(fileURLWithPath: NSTemporaryDirectory())
            .appending(path: "bounded-large-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for index in 0..<count {
            try "x".write(
                to: root.appending(path: "target-\(index).txt"), atomically: true,
                encoding: .utf8)
        }
        return root
    }

    /// Regression guard for issue #951 finding 5: the scan used to accumulate
    /// every match in the repo before sorting and truncating. With more
    /// matching candidates than `limit`, exactly `limit` results must come
    /// back, and they must be the genuinely best-scoring ones — not just
    /// whichever `limit` happened to be encountered first.
    @Test("keeps only the best `limit` matches when candidates exceed it")
    func boundedToLimit() async throws {
        let completion = FileCompletion(roots: [try makeTieBreakTree(count: 20)])
        let matches = await completion.matches(for: "target", limit: 5)
        #expect(matches.count == 5)
        #expect(
            Set(matches.map(\.relativePath))
                == Set([
                    "target.txt", "targetx.txt", "targetxx.txt", "targetxxx.txt",
                    "targetxxxx.txt",
                ]))
    }

    /// Regression guard for issue #951 finding 5: the doc comment claims the
    /// scan is cancellable, but `Task.detached` inside `matches` never
    /// inherited the caller's cancellation, so the `Task.isCancelled` checks
    /// in `scan` never tripped. Cancelling the caller must now stop the scan.
    @Test("stops scanning when the caller's task is cancelled")
    func cancellationPropagates() async throws {
        let root = try makeLargeTree(count: 4000)
        let completion = FileCompletion(roots: [root])
        let task = Task { await completion.matches(for: "target", limit: 4000) }
        task.cancel()
        let result = await task.value
        #expect(result.isEmpty)
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
