import SwiftUI

/// Semantic type roles separate content purpose from the system style that
/// implements it. Their metrics track the readable Codex transcript scale, so
/// a density change reaches every screen through one hierarchy.
enum Typography {
    static let display = Font.system(size: 22)
    static let title = Font.system(size: 20)
    static let heading = Font.system(size: 18)
    /// Primary transcript text measures 16.5pt in the Codex reference.
    static let bodyPointSize: CGFloat = 16.5
    static let body = Font.system(size: bodyPointSize)
    /// The nominal one-line height of the `body` role on macOS.
    static let bodyLineHeight: CGFloat = 21.5
    static let caption = Font.system(size: 14)
    static let detail = Font.system(size: 12)
    static let code = Font.system(size: 15).monospaced()
    static let codeCaption = Font.system(size: 14).monospaced()
    static let codeDetail = Font.system(size: 12).monospaced()
    static let numericCaption = Font.system(size: 14).monospacedDigit()
}
