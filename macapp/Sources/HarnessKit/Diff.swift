import Foundation

/// A line-level diff between two versions of a file.
public struct Diff: Sendable, Hashable {
    public enum Kind: Sendable, Hashable {
        case context, added, removed
    }

    public struct Line: Sendable, Hashable, Identifiable {
        public let id: Int
        public let kind: Kind
        public let text: String
        /// Line number on the old side, nil for added lines.
        public let oldNumber: Int?
        /// Line number on the new side, nil for removed lines.
        public let newNumber: Int?
    }

    public let lines: [Line]

    public init(from before: String, to after: String) {
        self.lines = Diff.lines(from: before, to: after)
    }

    public var additions: Int { lines.count { $0.kind == .added } }
    public var deletions: Int { lines.count { $0.kind == .removed } }
    public var hasChanges: Bool { lines.contains { $0.kind != .context } }

    /// Diffs by line using a longest-common-subsequence walk.
    ///
    /// Common prefixes and suffixes are stripped first, which is what keeps a
    /// one-line change in a large file cheap: the LCS table is built only over
    /// the differing middle, not the whole file.
    public static func lines(from before: String, to after: String) -> [Line] {
        let old = before.isEmpty ? [] : before.components(separatedBy: "\n")
        let new = after.isEmpty ? [] : after.components(separatedBy: "\n")

        var prefix = 0
        while prefix < old.count, prefix < new.count, old[prefix] == new[prefix] {
            prefix += 1
        }
        var suffix = 0
        while suffix < old.count - prefix, suffix < new.count - prefix,
            old[old.count - 1 - suffix] == new[new.count - 1 - suffix]
        {
            suffix += 1
        }

        let oldMiddle = Array(old[prefix..<(old.count - suffix)])
        let newMiddle = Array(new[prefix..<(new.count - suffix)])

        var result: [Line] = []
        var id = 0
        var oldNumber = 1
        var newNumber = 1

        func append(_ kind: Kind, _ text: String) {
            switch kind {
            case .context:
                result.append(
                    Line(id: id, kind: kind, text: text, oldNumber: oldNumber, newNumber: newNumber)
                )
                oldNumber += 1
                newNumber += 1
            case .added:
                result.append(
                    Line(id: id, kind: kind, text: text, oldNumber: nil, newNumber: newNumber))
                newNumber += 1
            case .removed:
                result.append(
                    Line(id: id, kind: kind, text: text, oldNumber: oldNumber, newNumber: nil))
                oldNumber += 1
            }
            id += 1
        }

        for index in 0..<prefix { append(.context, old[index]) }
        for change in changes(oldMiddle, newMiddle) { append(change.0, change.1) }
        for index in (old.count - suffix)..<old.count { append(.context, old[index]) }

        return result
    }

    /// Classic LCS backtrack over the differing region.
    private static func changes(_ old: [String], _ new: [String]) -> [(Kind, String)] {
        guard !old.isEmpty else { return new.map { (.added, $0) } }
        guard !new.isEmpty else { return old.map { (.removed, $0) } }

        var table = [[Int]](
            repeating: [Int](repeating: 0, count: new.count + 1), count: old.count + 1)
        for i in stride(from: old.count - 1, through: 0, by: -1) {
            for j in stride(from: new.count - 1, through: 0, by: -1) {
                table[i][j] =
                    old[i] == new[j]
                    ? table[i + 1][j + 1] + 1
                    : max(table[i + 1][j], table[i][j + 1])
            }
        }

        var result: [(Kind, String)] = []
        var i = 0
        var j = 0
        while i < old.count, j < new.count {
            if old[i] == new[j] {
                result.append((.context, old[i]))
                i += 1
                j += 1
            } else if table[i + 1][j] >= table[i][j + 1] {
                // Deletions are emitted before insertions so a replacement reads
                // as removed-then-added.
                result.append((.removed, old[i]))
                i += 1
            } else {
                result.append((.added, new[j]))
                j += 1
            }
        }
        while i < old.count {
            result.append((.removed, old[i]))
            i += 1
        }
        while j < new.count {
            result.append((.added, new[j]))
            j += 1
        }
        return result
    }
}

/// A file change recovered from a tool call's arguments, so an edit can be
/// shown as a diff without re-reading the file from disk.
public struct ToolEdit: Sendable, Hashable {
    public let path: String
    public let before: String
    public let after: String

    public var diff: Diff { Diff(from: before, to: after) }

    public init?(tool: String, arguments: String) {
        guard let data = arguments.data(using: .utf8),
            let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return nil }

        // Both tools accept `file_path` as an alias for `path`.
        guard
            let path = (object["path"] as? String) ?? (object["file_path"] as? String),
            !path.isEmpty
        else { return nil }

        switch tool {
        case "edit", "apply_patch":
            guard let before = object["old_text"] as? String,
                let after = object["new_text"] as? String
            else { return nil }
            self.path = path
            self.before = before
            self.after = after
        case "write":
            // write accepts four different names for the body.
            let keys = ["content", "new_text", "new_string", "text"]
            guard let after = keys.compactMap({ object[$0] as? String }).first else { return nil }
            self.path = path
            self.before = ""
            self.after = after
        default:
            return nil
        }
    }
}
