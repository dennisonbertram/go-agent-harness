import Foundation
import Testing

@testable import GoCodeUI
@testable import HarnessKit

@Suite("Diff memoisation")
struct DiffViewTests {

    /// Builds a `ToolEdit` via the same JSON path the app uses, so the fixture
    /// exercises the real `ToolEdit.diff` computation, not a stand-in.
    private func makeEdit(before: String, after: String) -> ToolEdit {
        let object: [String: Any] = ["path": "a.txt", "old_text": before, "new_text": after]
        let data = try! JSONSerialization.data(withJSONObject: object)
        let arguments = String(data: data, encoding: .utf8)!
        return ToolEdit(tool: "edit", arguments: arguments)!
    }

    /// Regression guard for issue #951 finding 4: `DiffView` used to read
    /// `edit.diff` (a computed property that reruns the whole LCS walk) once
    /// per access — `.additions`, `.deletions`, `.hasChanges` and `.lines` each
    /// paid for it separately. The view must compute it exactly once and reuse
    /// the stored value for every property it reads while rendering.
    @Test("computes the diff once per view instance, not once per property read")
    @MainActor
    func computesDiffOnce() {
        let before = (0..<200).map { "line \($0)" }.joined(separator: "\n")
        let after = (0..<200).map { $0 == 100 ? "changed" : "line \($0)" }.joined(separator: "\n")
        let edit = makeEdit(before: before, after: after)

        DiffComputationCounter.count = 0
        let view = DiffView(edit: edit)

        // Simulate exactly the reads `DiffView.body` performs per render.
        _ = view.diff.additions
        _ = view.diff.deletions
        _ = view.diff.hasChanges
        _ = view.diff.lines

        #expect(DiffComputationCounter.count == 1)
    }
}
