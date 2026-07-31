import SwiftUI

/// Shared layout rhythm. These are the distances that recur between controls,
/// rows, and containers; keeping them here lets a later visual pass tune the
/// rhythm without having to rediscover every use.
enum Spacing {
    static let none: CGFloat = 0
    /// 0.5, not 1. Every rule and border in the app measured 2 raw px against
    /// the reference's 1 — composer border, header rule, footer rule and the
    /// sidebar divider, four for four. On a 2x display 0.5pt is the one-pixel
    /// line the reference actually draws.
    static let hairline: CGFloat = 0.5
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
    /// Vertical room for the window controls when the header shares their row.
    /// The reference centres its title at y=28 with the lights at y=23–35.
    static let trafficLightClearance: CGFloat = 14
    static let page: CGFloat = 28
    /// Header height keeps the conversation title in the content pane, below
    /// window chrome and aligned with the transcript column.
    /// 40, not 62. With vertical padding the band measured 103pt against the
    /// reference's 55pt — 87% taller. The reference fits its title, folder
    /// icon and overflow in 55pt total.
    static let conversationHeaderHeight: CGFloat = 40
    /// The first message needs deliberate breathing room below the header.
    static let transcriptTop: CGFloat = 65.5
    /// Message action glyphs follow Codex's measured visual pitch.
    static let messageActionPitch: CGFloat = 34
}
