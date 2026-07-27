import AppKit
import HarnessKit
import SwiftUI

public enum Section: String, CaseIterable, Identifiable {
    case chat, sessions, checkpoints, settings

    public var id: String { rawValue }

    var title: String {
        switch self {
        case .chat: return "Chat"
        case .sessions: return "Sessions"
        case .checkpoints: return "Checkpoints"
        case .settings: return "Settings"
        }
    }

    var icon: String {
        switch self {
        case .chat: return "bubble.left.and.text.bubble.right"
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
        .frame(minWidth: 960, minHeight: 600)
    }
}

private struct ProjectPicker: View {
    let onOpen: (URL) -> Void

    var body: some View {
        VStack(spacing: 18) {
            Image(systemName: "chevron.left.forwardslash.chevron.right")
                .font(.system(size: 44, weight: .light))
                .foregroundStyle(.tint)
            VStack(spacing: 6) {
                Text("Open a project").font(.title2.weight(.semibold))
                Text("Each project runs its own harness server.")
                    .font(.callout).foregroundStyle(.secondary)
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
        HStack(spacing: 0) {
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
    }

    private var header: some View {
        HStack(spacing: 6) {
            Text(project.name).font(.headline)
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

private struct IconRail: View {
    @Binding var section: Section
    let project: ProjectSession
    let onClose: () -> Void

    var body: some View {
        VStack(spacing: 4) {
            ForEach(Section.allCases) { item in
                Button {
                    section = item
                } label: {
                    Image(systemName: item.icon)
                        .font(.system(size: 16))
                        .frame(width: 38, height: 34)
                        .background(
                            section == item ? Color.accentColor.opacity(0.16) : .clear,
                            in: .rect(cornerRadius: 8)
                        )
                        .foregroundStyle(section == item ? Color.accentColor : .secondary)
                }
                .buttonStyle(.plain)
                .help(item.title)
            }
            Spacer()
            Button(action: onClose) {
                Image(systemName: "xmark.circle")
                    .font(.system(size: 15))
                    .frame(width: 38, height: 34)
                    .foregroundStyle(.secondary)
            }
            .buttonStyle(.plain)
            .help("Close project and stop its server")
        }
        .padding(.vertical, 10)
        .frame(width: 50)
        .background(.quaternary.opacity(0.25))
    }
}

private struct StartingView: View {
    let workspace: URL
    var body: some View {
        VStack(spacing: 10) {
            ProgressView()
            Text("Starting the harness for \(workspace.lastPathComponent)…")
                .font(.callout).foregroundStyle(.secondary)
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
            .font(.headline).foregroundStyle(.orange)
            ScrollView {
                Text(message)
                    .font(.callout.monospaced()).textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .frame(maxHeight: 260)
            .padding(10)
            .background(.quaternary.opacity(0.35), in: .rect(cornerRadius: 8))
            Button("Try Again", action: retry).buttonStyle(.borderedProminent)
        }
        .padding(28)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }
}
