import HarnessKit
import SwiftUI

/// Background work and the current run's plan: the TUI's `/tasks` panel and
/// `/dashboard` combined, since on macOS they are one "what's happening" view.
struct ActivityView: View {
    @Bindable var project: ProjectSession
    @Binding var section: Section

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
                    if project.tasksLoadState.showsEmptyState(itemCount: project.tasks.count) {
                        Text("Nothing running.")
                            .font(Typography.body).foregroundStyle(Theme.foregroundTertiary)
                    } else if project.tasks.isEmpty {
                        LoadingPlaceholder()
                    } else {
                        ForEach(project.tasks) { task in
                            TaskRow(project: project, section: $section, task: task)
                        }
                    }
                }

                SectionBox(title: "Runs") {
                    if project.runsLoadState != .loaded, project.runs == nil {
                        LoadingPlaceholder()
                    } else if let runs = project.runs {
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
    @Bindable var project: ProjectSession
    @Binding var section: Section
    let task: TaskInfo
    @State private var confirmDelete = false
    @State private var actionInFlight: TaskAction?

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.small) {
            HStack(spacing: Spacing.standard) {
                Image(systemName: icon).foregroundStyle(.tint)
                VStack(alignment: .leading, spacing: Spacing.tight) {
                    Text(task.label).font(Typography.body).lineLimit(1)
                    MetadataRow {
                        Text(task.type.rawValue)
                        Text(task.status.rawValue)
                        if let age = task.ageSeconds { Text("\(age)s") }
                    }
                }
                Spacer()
                taskActions
            }
            ScheduledTaskLifecycle(task: task)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(TaskLifecycleText.accessibilityLabel(for: task))
        .alert("Delete scheduled task?", isPresented: $confirmDelete) {
            Button("Delete", role: .destructive) { perform(.delete) }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("This permanently removes \(task.label).")
        }
    }

    @ViewBuilder
    private var taskActions: some View {
        if task.conversationID?.isEmpty == false {
            Button("Open linked run", systemImage: "bubble.left.and.text.bubble.right") {
                Task { @MainActor in
                    guard await project.openScheduledTask(task) else { return }
                    section = .chat
                }
            }
            .buttonStyle(.borderless)
            .disabled(actionInFlight != nil)
            .accessibilityLabel(linkedRunAccessibilityLabel)
        }
        if task.actions?.contains(.pause) == true {
            actionButton(.pause, title: "Pause", systemImage: "pause.fill")
        }
        if task.actions?.contains(.resume) == true {
            actionButton(.resume, title: "Resume", systemImage: "play.fill")
        }
        if task.actions?.contains(.cancel) == true && task.type == .callback {
            actionButton(.cancel, title: "Cancel callback", systemImage: "xmark")
        }
        if task.actions?.contains(.delete) == true && task.type == .cron {
            Button("Delete", systemImage: "trash", role: .destructive) { confirmDelete = true }
                .buttonStyle(.borderless)
                .disabled(actionInFlight != nil)
                .accessibilityLabel("Delete cron \(task.label)")
        }
    }

    private var linkedRunAccessibilityLabel: String {
        if let runID = task.runID, !runID.isEmpty {
            return "Open linked run \(runID) for \(task.label)"
        }
        return "Open linked conversation for \(task.label)"
    }

    private func actionButton(_ action: TaskAction, title: String, systemImage: String) -> some View
    {
        Button(title, systemImage: systemImage) { perform(action) }
            .buttonStyle(.borderless)
            .disabled(actionInFlight != nil)
            .accessibilityLabel("\(title) \(task.label)")
    }

    private func perform(_ action: TaskAction) {
        actionInFlight = action
        Task {
            await project.performTaskAction(action, for: task)
            actionInFlight = nil
        }
    }

    private var icon: String {
        switch task.type {
        case .subagent: return "person.2"
        case .cron: return "calendar"
        case .callback: return "arrow.uturn.left"
        default: return "gearshape.2"
        }
    }
}

private struct ScheduledTaskLifecycle: View {
    let task: TaskInfo

    var body: some View {
        let lines = TaskLifecycleText.lines(for: task)
        if !lines.isEmpty {
            VStack(alignment: .leading, spacing: Spacing.tight) {
                ForEach(lines, id: \.self) { line in
                    Text(line).font(Typography.detail).foregroundStyle(Theme.foregroundTertiary)
                }
            }
            .accessibilityLabel(TaskLifecycleText.accessibilityLabel(for: task))
        }
    }
}

enum TaskLifecycleText {
    static func lines(for task: TaskInfo) -> [String] {
        var lines: [String] = []
        if let nextRunAt = task.nextRunAt {
            lines.append("Next: \(nextRunAt.formatted(date: .omitted, time: .shortened))")
        }
        if let firesAt = task.firesAt {
            lines.append("Due: \(firesAt.formatted(date: .omitted, time: .shortened))")
        }
        if let lastRunAt = task.lastRunAt {
            lines.append("Last: \(lastRunAt.formatted(date: .omitted, time: .shortened))")
        }
        if let status = task.lastExecutionStatus { lines.append("Last result: \(status.rawValue)") }
        if let runID = task.runID { lines.append("Run: \(runID)") }
        if let attempt = task.attempt { lines.append("Attempt: \(attempt)") }
        if let nextAttemptAt = task.nextAttemptAt {
            lines.append("Retry: \(nextAttemptAt.formatted(date: .omitted, time: .shortened))")
        }
        if let lastError = task.lastError, !lastError.isEmpty {
            lines.append("Error: \(lastError)")
        }
        return lines
    }

    static func accessibilityLabel(for task: TaskInfo) -> String {
        ([task.type.rawValue, task.label, task.status.rawValue] + lines(for: task)).joined(
            separator: ", ")
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
