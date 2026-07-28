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

/// Parses `tool|prompt` lines. Only the first "|" delimits: a prompt itself
/// may contain "|" (several in the real file do, e.g. quoted shell pipes),
/// so splitting on every "|" would truncate those prompts.
func parseToolSpecs(_ contents: String) throws -> [ToolSpec] {
    var specs: [ToolSpec] = []
    for rawLine in contents.split(separator: "\n", omittingEmptySubsequences: false) {
        let line = rawLine.trimmingCharacters(in: .whitespaces)
        guard !line.isEmpty else { continue }

        guard let separator = line.firstIndex(of: "|") else {
            throw ToolSpecError.malformedLine(line)
        }
        let name = String(line[line.startIndex..<separator]).trimmingCharacters(in: .whitespaces)
        let prompt = String(line[line.index(after: separator)...]).trimmingCharacters(
            in: .whitespaces)
        guard !name.isEmpty, !prompt.isEmpty else {
            throw ToolSpecError.malformedLine(line)
        }
        specs.append(ToolSpec(name: name, prompt: prompt))
    }
    return specs
}
