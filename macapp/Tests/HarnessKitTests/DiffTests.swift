import Foundation
import Testing

@testable import HarnessKit

@Suite("Diff")
struct DiffTests {

    @Test("marks unchanged lines as context")
    func unchangedLines() {
        let lines = Diff.lines(from: "a\nb\nc", to: "a\nb\nc")
        #expect(lines.allSatisfy { $0.kind == .context })
        #expect(lines.count == 3)
    }

    @Test("marks a pure insertion")
    func insertion() {
        let lines = Diff.lines(from: "a\nc", to: "a\nb\nc")
        #expect(lines.map(\.kind) == [.context, .added, .context])
        #expect(lines[1].text == "b")
    }

    @Test("marks a pure deletion")
    func deletion() {
        let lines = Diff.lines(from: "a\nb\nc", to: "a\nc")
        #expect(lines.map(\.kind) == [.context, .removed, .context])
        #expect(lines[1].text == "b")
    }

    @Test("marks a replacement as a delete followed by an add")
    func replacement() {
        let lines = Diff.lines(from: "a\nb\nc", to: "a\nB\nc")
        #expect(lines.map(\.kind) == [.context, .removed, .added, .context])
    }

    /// Line numbers drive the gutter; they must track each side independently
    /// or added and removed lines misalign with the file.
    @Test("numbers each side independently")
    func lineNumbers() {
        let lines = Diff.lines(from: "a\nb", to: "a\nx\nb")
        #expect(lines[0].oldNumber == 1)
        #expect(lines[0].newNumber == 1)
        // The inserted line exists only on the new side.
        #expect(lines[1].oldNumber == nil)
        #expect(lines[1].newNumber == 2)
        #expect(lines[2].oldNumber == 2)
        #expect(lines[2].newNumber == 3)
    }

    @Test("counts additions and deletions")
    func stats() {
        let diff = Diff(from: "a\nb\nc", to: "a\nX\nc\nd")
        #expect(diff.additions == 2)
        #expect(diff.deletions == 1)
        #expect(diff.hasChanges)
    }

    @Test("reports no changes for identical input")
    func noChanges() {
        let diff = Diff(from: "same", to: "same")
        #expect(!diff.hasChanges)
        #expect(diff.additions == 0)
        #expect(diff.deletions == 0)
    }

    @Test("handles empty sides")
    func emptySides() {
        #expect(Diff(from: "", to: "a\nb").additions == 2)
        #expect(Diff(from: "a\nb", to: "").deletions == 2)
    }

    /// A whole-file rewrite must not take quadratic time on a large file — the
    /// inspector renders these synchronously.
    @Test(.timeLimit(.minutes(1)))
    func handlesLargeFilesQuickly() {
        let before = (0..<4000).map { "line \($0)" }.joined(separator: "\n")
        let after = (0..<4000).map { $0 == 2000 ? "changed" : "line \($0)" }.joined(separator: "\n")
        let diff = Diff(from: before, to: after)
        #expect(diff.additions == 1)
        #expect(diff.deletions == 1)
    }
}

@Suite("Tool edit extraction")
struct ToolEditTests {

    /// The `edit` tool carries the before/after text in its arguments, which is
    /// what makes an inline diff possible without re-reading the file.
    @Test("extracts an edit from the edit tool's arguments")
    func extractsEdit() {
        let arguments = """
            {"path":"a/b.swift","old_text":"let x = 1","new_text":"let x = 2"}
            """
        let edit = ToolEdit(tool: "edit", arguments: arguments)
        #expect(edit?.path == "a/b.swift")
        #expect(edit?.before == "let x = 1")
        #expect(edit?.after == "let x = 2")
    }

    /// Both tools accept `file_path` as an alias for `path`, and write accepts
    /// four different names for its body; missing an alias silently drops the
    /// diff for that edit.
    @Test("accepts the documented field aliases")
    func acceptsAliases() {
        let viaFilePath = ToolEdit(
            tool: "edit",
            arguments: #"{"file_path":"x.swift","old_text":"a","new_text":"b"}"#)
        #expect(viaFilePath?.path == "x.swift")

        for key in ["content", "new_text", "new_string", "text"] {
            let write = ToolEdit(tool: "write", arguments: #"{"path":"n.txt","\#(key)":"body"}"#)
            #expect(write?.after == "body", "write alias \(key) not handled")
        }
    }

    @Test("extracts a whole-file write as an addition")
    func extractsWrite() {
        let edit = ToolEdit(tool: "write", arguments: #"{"path":"new.txt","content":"hello"}"#)
        #expect(edit?.path == "new.txt")
        #expect(edit?.before.isEmpty == true)
        #expect(edit?.after == "hello")
    }

    @Test("returns nil for tools that do not edit files")
    func ignoresNonEditTools() {
        #expect(ToolEdit(tool: "ls", arguments: #"{"path":"."}"#) == nil)
        #expect(ToolEdit(tool: "bash", arguments: #"{"command":"ls"}"#) == nil)
    }

    @Test("returns nil for malformed arguments")
    func ignoresMalformedArguments() {
        #expect(ToolEdit(tool: "edit", arguments: "not json") == nil)
        #expect(ToolEdit(tool: "edit", arguments: "{}") == nil)
    }
}
