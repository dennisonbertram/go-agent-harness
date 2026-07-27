import AppKit
import GoCodeUI
import SwiftUI

@main
struct GoCodeApp: App {
    private static let environment = ProcessInfo.processInfo.environment

    /// Skips the project picker when a workspace is supplied — handy for
    /// development and for opening a project from the command line.
    private static var initialWorkspace: URL? {
        environment["HARNESS_WORKSPACE"].map { URL(fileURLWithPath: $0) }
    }

    /// `HARNESS_BASE_URL` attaches to a harnessd someone else runs (and never
    /// terminates it); otherwise the app supervises its own, one per project.
    private static var externalBaseURL: URL? {
        environment["HARNESS_BASE_URL"].flatMap(URL.init(string:))
    }

    init() {
        // An SPM executable launches as an accessory process; without this the
        // window never takes focus and there is no Dock icon.
        NSApplication.shared.setActivationPolicy(.regular)
        NSApplication.shared.activate(ignoringOtherApps: true)
    }

    var body: some Scene {
        WindowGroup("GoCode") {
            AppShell(
                initialWorkspace: Self.initialWorkspace,
                externalBaseURL: Self.externalBaseURL,
                initialPrompt: Self.environment["GOCODE_INITIAL_PROMPT"])
        }
        .windowToolbarStyle(.unified)
    }
}
