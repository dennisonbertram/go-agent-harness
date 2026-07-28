import SwiftUI

/// Corner roles describe the container's job, rather than making callers
/// decide a radius independently. Values intentionally match the current UI.
enum CornerRadius {
    static let tag: CGFloat = 4
    static let code: CGFloat = 6
    static let control: CGFloat = 8
    static let card: CGFloat = 10
    static let composer: CGFloat = 14
}
