import Foundation

/// One line of `scripts/ui-walk-tools.txt`: the registered tool name and the
/// exact prompt to submit for it. Kept as a plain value type so parsing is
/// testable with no daemon involved.
struct ToolSpec: Equatable, Sendable {
    let name: String
    let prompt: String
}

enum ToolSpecError: Error, Equatable {
    case malformedLine(String)
}

// STUB — deliberately wrong, to prove the tests above fail for the right
// reason before the real parser is written.
func parseToolSpecs(_ contents: String) throws -> [ToolSpec] {
    []
}
