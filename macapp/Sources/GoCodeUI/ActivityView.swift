import HarnessKit
import SwiftUI

/// Background work and the current run's plan: the TUI's `/tasks` panel and
/// `/dashboard` combined, since on macOS they are one "what's happening" view.
struct ActivityView: View {
    @Bindable var project: ProjectSession

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.section) {
                if !project.todos.isEmpty {
                    SectionBox(title: "Plan") {
                        ForEach(project.todos, id: \.stableID) { todo in
                            HStack(spacing: Spacing.standard) {
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
                            .font(Typography.body)
                        }
                    }
                }

                SectionBox(title: "Background work") {
                    if project.tasksLoadState.showsBlockingError(itemCount: project.tasks.count) {
                        CollectionErrorState(message: project.tasksLoadState.errorMessage ?? "") {
                            Task { await project.refreshActivity() }
                        }
                    } else if project.tasksLoadState.showsEmptyState(itemCount: project.tasks.count)
                    {
                        Text("Nothing running.")
                            .font(Typography.body).foregroundStyle(Theme.foregroundTertiary)
                    } else if project.tasksLoadState.showsPlaceholder(
                        itemCount: project.tasks.count)
                    {
                        LoadingPlaceholder()
                    } else {
                        if project.tasksLoadState.showsRefreshError(
                            itemCount: project.tasks.count)
                        {
                            CollectionRefreshErrorState(
                                message: project.tasksLoadState.errorMessage ?? ""
                            ) {
                                Task { await project.refreshActivity() }
                            }
                        }
                        ForEach(project.tasks) { task in
                            TaskRow(task: task)
                        }
                    }
                }

                SectionBox(title: "Runs") {
                    if project.runsLoadState.showsBlockingError(
                        itemCount: project.runs?.count ?? 0)
                    {
                        CollectionErrorState(message: project.runsLoadState.errorMessage ?? "") {
                            Task { await project.refreshActivity() }
                        }
                    } else if project.runsLoadState.showsPlaceholder(
                        itemCount: project.runs?.count ?? 0)
                    {
                        LoadingPlaceholder()
                    } else if let runs = project.runs {
                        if project.runsLoadState.showsRefreshError(itemCount: runs.count) {
                            CollectionRefreshErrorState(
                                message: project.runsLoadState.errorMessage ?? ""
                            ) {
                                Task { await project.refreshActivity() }
                            }
                        }
                        if runs.isEmpty {
                            Text("No runs recorded yet.")
                                .font(Typography.body).foregroundStyle(Theme.foregroundTertiary)
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
                        .font(Typography.body).foregroundStyle(Theme.foregroundTertiary)
                    }
                }
            }
            .padding(Spacing.large)
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
        HStack(spacing: Spacing.standard) {
            Image(systemName: icon).foregroundStyle(.tint)
            VStack(alignment: .leading, spacing: Spacing.tight) {
                Text(task.label).font(Typography.body).lineLimit(1)
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
        HStack(spacing: Spacing.standard) {
            Circle().fill(color).frame(width: IconSize.status, height: IconSize.status)
            VStack(alignment: .leading, spacing: Spacing.tight) {
                Text(run.prompt ?? run.id).font(Typography.body).lineLimit(1)
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
        VStack(alignment: .leading, spacing: Spacing.standard) {
            Text(title).font(Typography.caption.weight(.semibold)).foregroundStyle(
                Theme.foregroundQuaternary)
            VStack(alignment: .leading, spacing: Spacing.small) { content }
                .padding(Spacing.inset)
                .frame(maxWidth: .infinity, alignment: .leading)
                .cardStyle()
        }
    }
}
