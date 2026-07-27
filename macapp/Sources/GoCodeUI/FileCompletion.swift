import Foundation

/// Fuzzy file completion for `@`-mentions in the composer.
///
/// Scans off the main thread and is cancellable, because a large repo has
/// hundreds of thousands of files and the composer must stay responsive.
public struct FileCompletion: Sendable {
    public struct Match: Sendable, Hashable, Identifiable {
        public var id: String { relativePath }
        public let relativePath: String
        public let score: Int
    }

    private let roots: [URL]
    /// Directories that are never worth completing and dominate the scan cost.
    private static let skipped: Set<String> = [
        ".git", "node_modules", ".build", "build", "dist", ".venv", "venv",
        "DerivedData", ".next", "target", "vendor", "__pycache__", ".swiftpm",
    ]

    public init(roots: [URL]) {
        self.roots = roots
    }

    /// Returns the best matches for `query`, most relevant first.
    public func matches(for query: String, limit: Int = 30) async -> [Match] {
        let needle = query.lowercased()
        let roots = self.roots
        return await Task.detached(priority: .userInitiated) {
            Self.scan(roots: roots, needle: needle, limit: limit)
        }.value
    }

    /// Synchronous so `FileManager`'s enumerator can be iterated — its iterator
    /// is unavailable from an async context.
    private static func scan(roots: [URL], needle: String, limit: Int) -> [Match] {
        var results: [Match] = []
        for suppliedRoot in roots {
            // macOS resolves /tmp and /var through symlinks, so the enumerator
            // yields /private/... paths while the supplied root does not.
            // `resolvingSymlinksInPath()` does not fix this for these prefixes,
            // so canonicalise with realpath — otherwise the prefix strip fails
            // and every completion is shown as an absolute path.
            let root = canonical(suppliedRoot)
            guard
                let enumerator = FileManager.default.enumerator(
                    at: root,
                    includingPropertiesForKeys: [.isDirectoryKey],
                    options: [.skipsHiddenFiles])
            else { continue }

            while let url = enumerator.nextObject() as? URL {
                if Task.isCancelled { return [] }

                let name = url.lastPathComponent
                if skipped.contains(name) {
                    enumerator.skipDescendants()
                    continue
                }
                guard
                    (try? url.resourceValues(forKeys: [.isDirectoryKey]))?.isDirectory == false
                else { continue }

                let relative =
                    url.path.hasPrefix(root.path)
                    ? String(url.path.dropFirst(root.path.count + 1))
                    : url.path
                guard let score = score(relative.lowercased(), needle) else { continue }
                results.append(Match(relativePath: relative, score: score))
            }
        }
        // Best score first, then shortest path — shallow files are usually the
        // intended target.
        return
            results
            .sorted {
                $0.score == $1.score
                    ? $0.relativePath.count < $1.relativePath.count : $0.score > $1.score
            }
            .prefix(limit)
            .map { $0 }
    }

    /// Canonical filesystem path, with every symlink resolved.
    private static func canonical(_ url: URL) -> URL {
        guard let resolved = realpath(url.path, nil) else { return url }
        defer { free(resolved) }
        return URL(fileURLWithPath: String(cString: resolved))
    }

    /// Subsequence match, scoring contiguous runs and basename hits higher.
    static func score(_ candidate: String, _ needle: String) -> Int? {
        guard !needle.isEmpty else { return 1 }

        var score = 0
        var run = 0
        var index = candidate.startIndex

        for character in needle {
            guard let found = candidate[index...].firstIndex(of: character) else { return nil }
            // Contiguous matches are worth more than scattered ones.
            run = found == index ? run + 1 : 0
            score += 1 + run
            index = candidate.index(after: found)
        }

        let basename = candidate.split(separator: "/").last.map(String.init) ?? candidate
        if basename.contains(needle) { score += 10 }
        if basename.hasPrefix(needle) { score += 10 }
        return score
    }
}

/// Locates the `@mention` the caret is currently inside.
public enum MentionQuery {
    /// Returns the partial path after the last `@`, or nil when there is no
    /// mention in progress.
    public static func current(in text: String) -> String? {
        guard let at = text.lastIndex(of: "@") else { return nil }
        let fragment = text[text.index(after: at)...]
        // A space ends the mention: the user has moved on.
        guard !fragment.contains(" "), !fragment.contains("\n") else { return nil }
        return String(fragment)
    }

    /// Replaces the in-progress mention with `path`, leaving a trailing space
    /// so the user can keep typing.
    public static func replacing(_ text: String, with path: String) -> String {
        guard let at = text.lastIndex(of: "@") else { return text }
        return text[..<at] + "@" + path + " "
    }
}
