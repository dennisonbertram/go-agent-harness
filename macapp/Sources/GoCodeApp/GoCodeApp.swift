import AppKit
import GoCodeUI
import SwiftUI

@main
struct GoCodeApp: App {
    /// Point the app at a harnessd. One server serves one project directory
    /// (`HARNESS_WORKSPACE`), so this is per-project — see design doc §2.
    private static var baseURL: URL {
        ProcessInfo.processInfo.environment["HARNESS_BASE_URL"]
            .flatMap(URL.init(string:)) ?? URL(string: "http://127.0.0.1:8080")!
    }

    init() {
        // An SPM executable launches as an accessory process; without this the
        // window never takes focus and there is no Dock icon.
        NSApplication.shared.setActivationPolicy(.regular)
        NSApplication.shared.activate(ignoringOtherApps: true)
    }

    var body: some Scene {
        WindowGroup("GoCode") {
            ContentView(baseURL: Self.baseURL)
        }
        .windowToolbarStyle(.unified)
    }
}
