import HarnessKit
import SwiftUI

/// Counts calls that compute a fresh `Diff` for a `DiffView` instance.
///
/// Test-only seam (issue #951 finding 4): `Diff` rebuilds the whole LCS table
/// on every access, so this proves the view computes it exactly once per
/// instance instead of once per property read. `ToolEdit` lives in HarnessKit,
/// which this task may not touch, so the cache — and the counter — live here.
@MainActor
enum DiffComputationCounter {
    static var count = 0
}

/// Renders a file edit as a unified diff.
///
/// Rows are virtualised: a whole-file rewrite can be thousands of lines and the
/// inspector renders synchronously.
struct DiffView: View {
    let edit: ToolEdit
    @State private var showsContext = true

    /// Computed once, in `init`, rather than left as `edit.diff`: the body
    /// reads `.additions`, `.deletions`, `.hasChanges` and `.lines`
    /// separately, and each of those used to rerun the LCS walk — again on
    /// every re-render, e.g. toggling "show unchanged lines".
    @State var diff: Diff

    init(edit: ToolEdit) {
        self.edit = edit
        DiffComputationCounter.count += 1
        _diff = State(initialValue: edit.diff)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.standard) {
            HStack(spacing: Spacing.standard) {
                Image(systemName: "doc.text").foregroundStyle(Theme.foregroundTertiary)
                Text(edit.path).font(Typography.code)
                    .lineLimit(1).truncationMode(.head)
                Spacer()
                Text("+\(diff.additions)").foregroundStyle(.green)
                Text("−\(diff.deletions)").foregroundStyle(.red)
            }
            .font(Typography.caption)

            if diff.hasChanges {
                Toggle("Show unchanged lines", isOn: $showsContext)
                    .toggleStyle(.checkbox).font(Typography.caption)
            }

            ScrollView([.horizontal, .vertical]) {
                LazyVStack(alignment: .leading, spacing: Spacing.none) {
                    ForEach(visibleLines) { line in
                        DiffRow(line: line)
                    }
                }
            }
            .cardStyle(cornerRadius: CornerRadius.control)
        }
    }

    private var visibleLines: [Diff.Line] {
        showsContext ? diff.lines : diff.lines.filter { $0.kind != .context }
    }
}

private struct DiffRow: View {
    let line: Diff.Line

    var body: some View {
        HStack(spacing: Spacing.none) {
            gutter(line.oldNumber)
            gutter(line.newNumber)
            Text(marker)
                .font(Typography.codeCaption)
                .foregroundStyle(markerColor)
                .frame(width: IconSize.detail)
            Text(line.text.isEmpty ? " " : line.text)
                .font(Typography.codeCaption)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.vertical, Spacing.hairline)
        .background(background)
    }

    private func gutter(_ number: Int?) -> some View {
        Text(number.map(String.init) ?? "")
            .font(Typography.codeDetail)
            .foregroundStyle(Theme.foregroundQuaternary)
            .frame(width: Layout.diffGutterWidth, alignment: .trailing)
            .padding(.trailing, 5)
    }

    private var marker: String {
        switch line.kind {
        case .added: return "+"
        case .removed: return "−"
        case .context: return " "
        }
    }

    private var markerColor: Color {
        switch line.kind {
        case .added: return .green
        case .removed: return .red
        case .context: return Theme.foregroundQuaternary
        }
    }

    private var background: Color {
        switch line.kind {
        case .added: return .green.opacity(StateOpacity.emphasis)
        case .removed: return .red.opacity(StateOpacity.emphasis)
        case .context: return .clear
        }
    }
}
