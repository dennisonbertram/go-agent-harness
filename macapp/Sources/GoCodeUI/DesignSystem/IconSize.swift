import SwiftUI

/// Icon measurements are roles because a status dot, row glyph, and empty
/// state communicate at different distances. They retain existing metrics.
enum IconSize {
    static let rule: CGFloat = 3
    static let status: CGFloat = 7
    static let detail: CGFloat = 14
    static let standard: CGFloat = 15
    static let row: CGFloat = 18
    /// Composer chip icons. Fixed so adjacent chips cannot differ in symbol
    /// width, which they did by 48%.
    static let chip: CGFloat = 13.5
    static let emptyState: CGFloat = 30
    static let launch: CGFloat = 44
}
