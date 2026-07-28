import HarnessKit
import SwiftUI

/// Background work and the current run's plan: the TUI's `/tasks` panel and
/// `/dashboard` combined, since on macOS they are one "what's happening" view.
struct ActivityView: View {
    @Bindable var project: ProjectSession

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                if !project.todos.isEmpty {
                    SectionBox(title: "Plan") {
                        ForEach(project.todos, id: \.stableID) { todo in
                            HStack(spacing: 8) {
                                Image(
                                    systemName: todo.isDone
                                        ? "checkmark.circle.fill" : "circle"
                                )
                                .foregroundStyle(todo.isDone ? .green : Theme.foregroundTertiary)
                                Text(todo.text)
                                    .strikethrough(todo.isDone)
                                    .foregroundStyle(
                                        todo.isDone ? Theme.foregroundTertiary : Theme.foreground)
                                Spacer()
                            }
                            .font(.callout)
                        }
                    }
                }

                SectionBox(title: "Background work") {
                    if project.tasks.isEmpty {
                        Text("Nothing running.")
                            .font(.callout).foregroundStyle(Theme.foregroundTertiary)
                    } else {
                        ForEach(project.tasks) { task in
                            TaskRow(task: task)
                        }
                    }
                }

                SectionBox(title: "Runs") {
                    if let runs = project.runs {
                        if runs.isEmpty {
                            Text("No runs recorded yet.")
                                .font(.callout).foregroundStyle(Theme.foregroundTertiary)
                        } else {
                            ForEach(runs) { run in
                                RunRow(run: run)
                            }
                        }
                    } else {
                        // 501 from /v1/runs is a configuration state, not a fault.
                        Text(
                            "This server has no run store configured, so past runs are not listed."
                        )
                        .font(.callout).foregroundStyle(Theme.foregroundTertiary)
                    }
                }
            }
            .padding(16)
        }
        .task { await project.refreshActivity() }
        // Polling stops when the view is not shown, rather than running a fixed
        // 2s timer forever the way the TUI dashboard does.
        .task {
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(3))
                guard !Task.isCancelled else { return }
                await project.refreshActivity()
            }
        }
    }
}

private struct TaskRow: View {
    let task: TaskInfo

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: icon).foregroundStyle(.tint)
            VStack(alignment: .leading, spacing: 2) {
                Text(task.label).font(.callout).lineLimit(1)
                MetadataRow {
                    Text(task.type)
                    Text(task.status)
                    if let age = task.ageSeconds { Text("\(age)s") }
                }
            }
            Spacer()
        }
    }

    private var icon: String {
        switch task.type {
        case "subagent": return "person.2"
        case "cron": return "calendar"
        case "callback": return "arrow.uturn.left"
        default: return "gearshape.2"
        }
    }
}

private struct RunRow: View {
    let run: RunSummaryInfo

    var body: some View {
        HStack(spacing: 8) {
            Circle().fill(color).frame(width: 7, height: 7)
            VStack(alignment: .leading, spacing: 2) {
                Text(run.prompt ?? run.id).font(.callout).lineLimit(1)
                MetadataRow {
                    if let status = run.status { Text(status) }
                    if let model = run.model { Text(model) }
                    if let date = run.createdAt {
                        Text(date.formatted(date: .omitted, time: .shortened))
                    }
                }
            }
            Spacer()
        }
    }

    private var color: Color {
        switch run.status {
        case "completed": return .green
        case "failed": return .red
        case "running", "queued": return .blue
        default: return Theme.foregroundQuaternary
        }
    }
}

private struct SectionBox<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.caption.weight(.semibold)).foregroundStyle(Theme.foregroundQuaternary)
            VStack(alignment: .leading, spacing: 7) { content }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .cardStyle()
        }
    }
}
