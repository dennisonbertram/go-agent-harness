import AppKit
import HarnessKit
import SwiftUI

public enum Section: String, CaseIterable, Identifiable {
    case chat, activity, sessions, checkpoints, settings

    public var id: String { rawValue }

    var title: String {
        switch self {
        case .chat: return "Chat"
        case .activity: return "Activity"
        case .sessions: return "Sessions"
        case .checkpoints: return "Checkpoints"
        case .settings: return "Settings"
        }
    }

    var icon: String {
        switch self {
        case .chat: return "bubble.left.and.text.bubble.right"
        case .activity: return "chart.bar.doc.horizontal"
        case .sessions: return "clock.arrow.circlepath"
        case .checkpoints: return "arrow.uturn.backward.circle"
        case .settings: return "gearshape"
        }
    }
}

/// Root view. Shows a project picker until a workspace is chosen, because
/// harnessd cannot start without one — its workspace is fixed at process launch.
public struct AppShell: View {
    @State private var project: ProjectSession?
    @State private var section: Section = .chat
    private let initialPrompt: String?

    public init(
        initialWorkspace: URL? = nil, externalBaseURL: URL? = nil, initialPrompt: String? = nil
    ) {
        self.initialPrompt = initialPrompt
        if let initialWorkspace {
            _project = State(
                initialValue: ProjectSession(
                    workspace: initialWorkspace, externalBaseURL: externalBaseURL))
        }
    }

    public var body: some View {
        Group {
            if let project {
                ProjectView(project: project, section: $section, initialPrompt: initialPrompt) {
                    Task {
                        await project.shutdown()
                        self.project = nil
                    }
                }
            } else {
                ProjectPicker { url in
                    project = ProjectSession(workspace: url)
                }
            }
        }
        // Widened for the labeled rail (was 50pt icon-only, now 220pt) plus
        // room for the inspector pane to open without squeezing the
        // transcript below its own minWidth.
        .frame(minWidth: Layout.appMinimumWidth, minHeight: Layout.appMinimumHeight)
        // The root surface. Every other token in `Theme` is defined relative
        // to this value, so it has to be painted explicitly rather than left
        // as the system window background — that's the one substitution
        // that pulled every surface in the app toward the same narrow, warm
        // range (`.ux/design-baseline.md` §8).
        .background(Theme.background)
    }
}

private struct ProjectPicker: View {
    let onOpen: (URL) -> Void

    var body: some View {
        VStack(spacing: Spacing.section) {
            Image(systemName: "chevron.left.forwardslash.chevron.right")
                .font(.system(size: IconSize.launch, weight: .light))
                .foregroundStyle(.tint)
            VStack(spacing: Spacing.small) {
                Text("Open a project").font(Typography.title.weight(.semibold))
                Text("Each project runs its own harness server.")
                    .font(Typography.body).foregroundStyle(Theme.foregroundTertiary)
            }
            Button("Choose Folder…", action: choose)
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func choose() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.prompt = "Open"
        if panel.runModal() == .OK, let url = panel.url {
            onOpen(url)
        }
    }
}

private struct ProjectView: View {
    @Bindable var project: ProjectSession
    @Binding var section: Section
    var initialPrompt: String?
    let onClose: () -> Void

    var body: some View {
        HStack(spacing: Spacing.none) {
            IconRail(section: $section, project: project, onClose: onClose)
            Divider()
            content
        }
        .task {
            await project.start()
            // Lets the app be driven from the command line, and is how the
            // streaming UI gets exercised without a human at the keyboard.
            if let initialPrompt, project.isReady, let run = project.run {
                run.draft = initialPrompt
                project.submit()
            }
        }
        .toolbar { ToolbarItem(placement: .navigation) { header } }
        // The native toolbar material is warmer and much lighter than the
        // page. Painting it with the root token lets the page run cleanly to
        // the window edge instead of leaving a separate chrome band.
        .toolbarBackground(Theme.background, for: .windowToolbar)
        .toolbarBackground(.visible, for: .windowToolbar)
    }

    private var header: some View {
        HStack(spacing: Spacing.small) {
            Text(project.name).font(Typography.heading)
            phaseBadge
        }
    }

    @ViewBuilder
    private var phaseBadge: some View {
        switch project.phase {
        case .starting:
            ProgressView().controlSize(.small).scaleEffect(0.6)
        case .failed:
            Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(.orange)
        case .ready, .idle:
            EmptyView()
        }
    }

    @ViewBuilder
    private var content: some View {
        switch project.phase {
        case .idle, .starting:
            StartingView(workspace: project.workspace)
        case .failed(let message):
            StartupFailureView(message: message) { Task { await project.start() } }
        case .ready:
            switch section {
            case .chat:
                if let run = project.run {
                    ChatView(project: project, run: run)
                }
            case .activity:
                ActivityView(project: project)
            case .sessions:
                SessionsView(project: project, section: $section)
            case .checkpoints:
                CheckpointsView(project: project)
            case .settings:
                SettingsView(project: project)
            }
        }
    }
}

/// Labeled, sectioned nav — was a 50pt icon-only strip with no labels and no
/// grouping (baseline gap #5). Sessions + Checkpoints are grouped under a
/// "History" heading per the design plan's A2, since both browse past state
/// rather than switching between live-run modes the way Chat/Activity do.
private struct IconRail: View {
    @Binding var section: Section
    let project: ProjectSession
    let onClose: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.tight) {
            RailRow(section: $section, item: .chat)
            RailRow(section: $section, item: .activity)

            RailSectionHeader("History")
            RailRow(section: $section, item: .sessions)
            RailRow(section: $section, item: .checkpoints)

            Spacer()

            RailRow(section: $section, item: .settings)

            Button(action: onClose) {
                HStack(spacing: Spacing.comfortable) {
                    Image(systemName: "xmark.circle")
                        .font(.system(size: IconSize.standard)).frame(width: IconSize.row)
                    Text("Close").font(Typography.body)
                    Spacer(minLength: Spacing.none)
                }
                .padding(.horizontal, Spacing.comfortable).padding(.vertical, Spacing.standard)
                .frame(maxWidth: .infinity, alignment: .leading)
                .foregroundStyle(Theme.foregroundSecondary)
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .help("Close project and stop its server")
            .accessibilityLabel("Close project and stop its server")
        }
        .padding(.vertical, Spacing.comfortable).padding(.horizontal, Spacing.standard)
        .frame(width: Layout.railWidth)
        // The rail is continuous window chrome rather than a translucent
        // overlay: its surface reaches the toolbar-safe area while the
        // navigation controls retain their traffic-light-safe placement.
        .background(Theme.surface.ignoresSafeArea(.container, edges: .top))
    }
}

/// One nav row: icon + text label, so the accessibility name is the label
/// text rather than the SF Symbol identifier (baseline gap #5) and a sighted
/// user gets a name too, not just a glyph.
private struct RailRow: View {
    @Binding var section: Section
    let item: Section

    var body: some View {
        Button {
            section = item
        } label: {
            HStack(spacing: Spacing.comfortable) {
                Image(systemName: item.icon)
                    .font(.system(size: IconSize.standard))
                    .frame(width: IconSize.row)
                    // The label text alone names this row; without this the
                    // icon contributes its own SF Symbol identifier to the
                    // combined accessibility description.
                    .accessibilityHidden(true)
                Text(item.title).font(Typography.body)
                Spacer(minLength: Spacing.none)
            }
            .padding(.horizontal, Spacing.comfortable).padding(.vertical, Spacing.standard)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                section == item ? Theme.selectedRowSurface : .clear,
                in: .rect(cornerRadius: CornerRadius.control)
            )
            .foregroundStyle(
                section == item ? Theme.selectedRowForeground : Theme.foregroundSecondary
            )
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .help(item.title)
        .accessibilityLabel(item.title)
    }
}

private struct RailSectionHeader: View {
    let title: String
    init(_ title: String) { self.title = title }

    var body: some View {
        Text(title.uppercased())
            .font(Typography.detail.weight(.semibold))
            .foregroundStyle(Theme.foregroundTertiary)
            .padding(.horizontal, Spacing.comfortable).padding(.top, Spacing.comfortable)
            .padding(.bottom, Spacing.tight)
            // A heading, not a control — VoiceOver should announce it once
            // as a group label, not treat it as another focusable row.
            .accessibilityAddTraits(.isHeader)
    }
}

private struct StartingView: View {
    let workspace: URL
    var body: some View {
        VStack(spacing: Spacing.comfortable) {
            ProgressView()
            Text("Starting the harness for \(workspace.lastPathComponent)…")
                .font(Typography.body).foregroundStyle(Theme.foregroundTertiary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// A startup failure is otherwise invisible — the app would just sit there — so
/// the child's own stderr is shown verbatim.
private struct StartupFailureView: View {
    let message: String
    let retry: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label(
                "The harness server could not start", systemImage: "exclamationmark.triangle.fill"
            )
            .font(Typography.heading).foregroundStyle(.orange)
            ScrollView {
                Text(message)
                    .font(Typography.code).textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .frame(maxHeight: 260)
            .padding(Spacing.comfortable)
            .compactElevatedSurface()
            Button("Try Again", action: retry).buttonStyle(.borderedProminent)
        }
        .padding(Spacing.page)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }
}
