import SwiftUI

/// Motion durations are semantic so loading affordances can remain subdued
/// together and honor Reduce Motion at their call sites.
enum Motion {
    static let loadingFadeDuration: TimeInterval = 0.16
    static let loadingPulseDuration: TimeInterval = 1.2
}
