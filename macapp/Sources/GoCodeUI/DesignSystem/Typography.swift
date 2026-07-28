import SwiftUI

/// Semantic type roles preserve today's Dynamic Type-aware metrics while
/// separating content purpose from the system style that happens to implement
/// it. The roles leave room for the later 16pt primary-text / deeper Codex-like
/// hierarchy pass without silently changing this pass's appearance.
enum Typography {
    static let display = Font.title2
    static let title = Font.title2
    static let heading = Font.headline
    static let body = Font.callout
    static let caption = Font.caption
    static let detail = Font.caption2
    static let code = Font.callout.monospaced()
    static let codeCaption = Font.caption.monospaced()
    static let codeDetail = Font.caption2.monospaced()
    static let numericCaption = Font.caption.monospacedDigit()
}
