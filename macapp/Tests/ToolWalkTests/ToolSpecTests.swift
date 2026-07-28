import Foundation
import Testing

@testable import ToolWalk

/// `scripts/ui-walk-tools.txt` is `tool|prompt` per line; these tests pin the
/// parsing rules the live walk depends on before any daemon is involved.
@Suite("ToolSpec parsing")
struct ToolSpecTests {

    @Test("splits each line on the first pipe into name and prompt")
    func splitsNameAndPrompt() throws {
        let specs = try parseToolSpecs(
            """
            bash|Use the bash tool to run: echo OK and report the exact output.
            grep|Use the grep tool to grep for 'module' in go.mod|extra pipe stays in the prompt
            """)

        #expect(specs.count == 2)
        #expect(specs[0].name == "bash")
        #expect(specs[0].prompt == "Use the bash tool to run: echo OK and report the exact output.")
        // Only the first "|" is a delimiter — a prompt containing "|" must not
        // be truncated.
        #expect(
            specs[1].prompt
                == "Use the grep tool to grep for 'module' in go.mod|extra pipe stays in the prompt"
        )
    }

    @Test("trims surrounding whitespace from both fields")
    func trimsWhitespace() throws {
        let specs = try parseToolSpecs("  ls  |  list the directory  ")
        #expect(specs.count == 1)
        #expect(specs[0].name == "ls")
        #expect(specs[0].prompt == "list the directory")
    }

    @Test("skips blank lines without producing a spec")
    func skipsBlankLines() throws {
        let specs = try parseToolSpecs("bash|do a thing\n\n\nread|read a thing\n")
        #expect(specs.count == 2)
    }

    @Test("rejects a line with no separator")
    func rejectsMissingSeparator() {
        #expect(throws: ToolSpecError.self) {
            try parseToolSpecs("bash without a pipe")
        }
    }

    @Test("rejects a line with an empty tool name")
    func rejectsEmptyName() {
        #expect(throws: ToolSpecError.self) {
            try parseToolSpecs("|a prompt with no tool name")
        }
    }

    @Test("rejects a line with an empty prompt")
    func rejectsEmptyPrompt() {
        #expect(throws: ToolSpecError.self) {
            try parseToolSpecs("bash|   ")
        }
    }

    /// Regression: the real file this walk depends on must keep parsing to
    /// exactly 61 uniquely-named tools. If a future edit breaks the format —
    /// a missing pipe, a duplicated tool line — this fails loudly instead of
    /// the walk silently dropping or double-running a tool.
    @Test("the real ui-walk-tools.txt parses to 61 unique tool specs")
    func realToolsFileParsesCompletely() throws {
        let url = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // ToolWalkTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // macapp
            .deletingLastPathComponent()  // repo root
            .appending(path: "scripts").appending(path: "ui-walk-tools.txt")
        let contents = try String(contentsOf: url, encoding: .utf8)

        let specs = try parseToolSpecs(contents)
        #expect(specs.count == 61)
        #expect(Set(specs.map(\.name)).count == 61, "every tool name must be unique")
    }
}
