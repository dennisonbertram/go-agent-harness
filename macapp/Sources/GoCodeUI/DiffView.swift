import HarnessKit
import SwiftUI

/// Renders a file edit as a unified diff.
///
/// Rows are virtualised: a whole-file rewrite can be thousands of lines and the
/// inspector renders synchronously.
struct DiffView: View {
    let edit: ToolEdit
    @State private var showsContext = true

    private var diff: Diff { edit.diff }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Image(systemName: "doc.text").foregroundStyle(.secondary)
                Text(edit.path).font(.callout.monospaced())
                    .lineLimit(1).truncationMode(.head)
                Spacer()
                Text("+\(diff.additions)").foregroundStyle(.green)
                Text("−\(diff.deletions)").foregroundStyle(.red)
            }
            .font(.caption)

            if diff.hasChanges {
                Toggle("Show unchanged lines", isOn: $showsContext)
                    .toggleStyle(.checkbox).font(.caption)
            }

            ScrollView([.horizontal, .vertical]) {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(visibleLines) { line in
                        DiffRow(line: line)
                    }
                }
            }
            .background(.quaternary.opacity(0.25), in: .rect(cornerRadius: 8))
        }
    }

    private var visibleLines: [Diff.Line] {
        showsContext ? diff.lines : diff.lines.filter { $0.kind != .context }
    }
}

private struct DiffRow: View {
    let line: Diff.Line

    var body: some View {
        HStack(spacing: 0) {
            gutter(line.oldNumber)
            gutter(line.newNumber)
            Text(marker)
                .font(.caption.monospaced())
                .foregroundStyle(markerColor)
                .frame(width: 14)
            Text(line.text.isEmpty ? " " : line.text)
                .font(.caption.monospaced())
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.vertical, 1)
        .background(background)
    }

    private func gutter(_ number: Int?) -> some View {
        Text(number.map(String.init) ?? "")
            .font(.caption2.monospaced())
            .foregroundStyle(.tertiary)
            .frame(width: 38, alignment: .trailing)
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
        case .context: return .secondary
        }
    }

    private var background: Color {
        switch line.kind {
        case .added: return .green.opacity(0.12)
        case .removed: return .red.opacity(0.12)
        case .context: return .clear
        }
    }
}
