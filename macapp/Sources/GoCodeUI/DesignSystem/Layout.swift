import SwiftUI

/// Fixed dimensions describe product constraints, not spacing. Naming them
/// prevents unrelated panes and fields from becoming accidental substitutes.
enum Layout {
    static let appMinimumWidth: CGFloat = 1_040
    static let appMinimumHeight: CGFloat = 600
    static let railWidth: CGFloat = 220
    /// Codex's environment surface is a compact overlay card, not a sidebar.
    static let inspectorCardWidth: CGFloat = 361
    static let chatMinimumWidth: CGFloat = 400
    static let chatIdealWidth: CGFloat = 520
    /// Keep the transcript and composer on the same readable column instead
    /// of turning either into a near-edge-to-edge control on wide windows.
    static let chatContentMaximumWidth: CGFloat = 883

    /// User prompts read as authored content, not a full-column alert. This
    /// caps a wrapped prompt while allowing short prompts to retain their
    /// intrinsic width.
    static let userMessageMaximumWidth: CGFloat = 374.5

    /// A one-line body prompt plus its semantic vertical padding. Keeping this
    /// formula coupled to the same roles used by `UserBubble` lets type or
    /// spacing changes update its minimum rhythm together.
    static let userMessageMinimumHeight: CGFloat =
        Typography.bodyLineHeight + (Spacing.userMessageVertical * 2)
    static let providerMinimumWidth: CGFloat = 240
    static let providerIdealWidth: CGFloat = 280
    static let modelMinimumWidth: CGFloat = 380
    static let costFieldWidth: CGFloat = 54
    static let diffGutterWidth: CGFloat = 38
    static let emptyStateTextWidth: CGFloat = 320
    static let providerSheetWidth: CGFloat = 460
}
