import SwiftUI

/// Motion durations are semantic so loading affordances can remain subdued
/// together and honor Reduce Motion at their call sites.
enum Motion {
    static let loadingFadeDuration: TimeInterval = 0.16
    static let loadingPulseDuration: TimeInterval = 1.2
    /// The transcript's autoscroll-to-bottom animation, named so the view
    /// can also size the delay before it clears the flag that suppresses
    /// scroll-geometry updates during that same animation.
    static let autoscrollDuration: TimeInterval = 0.12
}
