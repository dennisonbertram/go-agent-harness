import SwiftUI

/// Shared layout rhythm. These are the distances that recur between controls,
/// rows, and containers; keeping them here lets a later visual pass tune the
/// rhythm without having to rediscover every use.
enum Spacing {
    static let none: CGFloat = 0
    static let hairline: CGFloat = 1
    static let tight: CGFloat = 2
    static let compact: CGFloat = 4
    static let small: CGFloat = 6
    /// Icon-to-label gap inside a composer chip. Pinned so adjacent chips
    /// cannot drift apart, which they had by 85%.
    static let chipLabelGap: CGFloat = 8.5
    static let standard: CGFloat = 8
    static let comfortable: CGFloat = 10
    static let inset: CGFloat = 12
    /// Vertical breathing room for a one-line user prompt.
    static let userMessageVertical: CGFloat = 12
    static let large: CGFloat = 16
    static let section: CGFloat = 18
    static let page: CGFloat = 28
    /// Header height keeps the conversation title in the content pane, below
    /// window chrome and aligned with the transcript column.
    static let conversationHeaderHeight: CGFloat = 52
    /// The first message needs deliberate breathing room below the header.
    static let transcriptTop: CGFloat = 65.5
    /// Message action glyphs follow Codex's measured visual pitch.
    static let messageActionPitch: CGFloat = 34
}
