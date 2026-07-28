import HarnessKit
import SwiftUI

/// Codex groups environment state by kind. This card deliberately has no
/// height constraint: it occupies only the space its present cards require,
/// instead of reserving a permanent full-height column for an optional detail.
struct EnvironmentInspector: View {
    @Bindable var project: ProjectSession
    let usage: UsageTotals
    let activities: [ToolActivity]
    @Binding var selected: ToolActivity?

    private var subagents: [TaskInfo] { project.tasks.filter { $0.type == "subagent" } }
    private var backgroundTasks: [TaskInfo] { project.tasks.filter { $0.type != "subagent" } }

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.standard) {
            Text("Environment")
                .font(Typography.heading)

            if usage.totalTokens > 0 {
                EnvironmentCard(title: "Usage", icon: "gauge.with.dots.needle.67percent") {
                    UsageLabel(usage: usage)
                }
            }

            if !project.rewindPoints.isEmpty {
                EnvironmentCard(title: "Changes", icon: "arrow.uturn.backward.circle") {
                    ForEach(project.rewindPoints) { point in
                        CheckpointSummary(point: point)
                    }
                }
            }

            if !subagents.isEmpty {
                EnvironmentCard(title: "Subagents", icon: "person.2") {
                    ForEach(subagents) { task in TaskSummary(task: task) }
                }
            }

            if !backgroundTasks.isEmpty || !activities.isEmpty {
                EnvironmentCard(title: "Background processes", icon: "gearshape.2") {
                    ForEach(backgroundTasks) { task in TaskSummary(task: task) }
                    ForEach(activities) { activity in
                        ToolActivitySummary(
                            activity: activity, isSelected: selected?.id == activity.id
                        ) {
                            selected = activity
                        }
                    }
                    if let selected {
                        Divider()
                        ToolActivityDetail(activity: selected)
                    }
                }
            }

            if usage.totalTokens == 0 && project.rewindPoints.isEmpty && subagents.isEmpty
                && backgroundTasks.isEmpty && activities.isEmpty
            {
                EnvironmentCard(title: "Environment", icon: "sidebar.right") {
                    Text("No changes or background work yet.")
                        .font(Typography.caption)
                        .foregroundStyle(Theme.foregroundTertiary)
                }
            }
        }
        .padding(Spacing.inset)
        .frame(width: Layout.inspectorCardWidth, alignment: .leading)
        .fixedSize(horizontal: false, vertical: true)
        .cardStyle()
        .task {
            await project.refreshRewindPoints()
            await project.refreshActivity()
        }
    }
}

/// Reused card chrome keeps each environment kind distinct without making the
/// inspector a visually undifferentiated column again.
private struct EnvironmentCard<Content: View>: View {
    let title: String
    let icon: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.small) {
            Label(title, systemImage: icon)
                .font(Typography.caption.weight(.semibold))
                .foregroundStyle(Theme.foregroundSecondary)
            VStack(alignment: .leading, spacing: Spacing.small) { content }
        }
        .padding(Spacing.standard)
        .frame(maxWidth: .infinity, alignment: .leading)
        .compactElevatedSurface()
    }
}

private struct CheckpointSummary: View {
    let point: RewindPoint

    var body: some View {
        HStack(spacing: Spacing.small) {
            Text(point.tool.map { "Before \($0)" } ?? "Checkpoint")
                .font(Typography.caption)
                .lineLimit(1)
            Spacer(minLength: Spacing.none)
            if let date = point.createdAt {
                Text(date.formatted(date: .omitted, time: .shortened))
                    .font(Typography.detail)
                    .foregroundStyle(Theme.foregroundTertiary)
            }
        }
    }
}

private struct TaskSummary: View {
    let task: TaskInfo

    var body: some View {
        HStack(spacing: Spacing.small) {
            Text(task.label).font(Typography.caption).lineLimit(1)
            Spacer(minLength: Spacing.none)
            Text(task.status).font(Typography.detail).foregroundStyle(Theme.foregroundTertiary)
        }
    }
}

private struct ToolActivitySummary: View {
    let activity: ToolActivity
    let isSelected: Bool
    let onSelect: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: Spacing.small) {
                Text(activity.tool).font(Typography.caption).lineLimit(1)
                Spacer(minLength: Spacing.none)
                Text(String(describing: activity.status))
                    .font(Typography.detail)
                    .foregroundStyle(Theme.foregroundTertiary)
            }
            .padding(Spacing.compact)
            .background(
                isSelected ? Theme.surfaceHighest : .clear,
                in: .rect(cornerRadius: CornerRadius.tag)
            )
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
    }
}

/// The selected activity stays in its kind's card so detail does not cause a
/// second, permanent inspector region to reappear beside the conversation.
private struct ToolActivityDetail: View {
    let activity: ToolActivity

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.small) {
            if let edit = ToolEdit(tool: activity.tool, arguments: activity.arguments) {
                DiffView(edit: edit)
            } else if !activity.arguments.isEmpty {
                LabelledCode(title: "Arguments", body: activity.arguments)
            }
            if !activity.output.isEmpty {
                LabelledCode(title: "Output", body: activity.output)
            }
        }
    }
}
