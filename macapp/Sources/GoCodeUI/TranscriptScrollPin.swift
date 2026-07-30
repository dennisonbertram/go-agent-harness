import SwiftUI

/// Decides whether the transcript should auto-scroll to the bottom as new
/// content streams in.
///
/// A fresh transcript starts pinned. Once the operator scrolls away from the
/// bottom by more than `Layout.autoscrollPinThreshold`, autoscroll stops so a
/// stream does not yank the view away mid-read; returning to the bottom
/// re-pins it. This is a pure decision — the view supplies the measured
/// distance and only reads `isPinned`.
struct TranscriptScrollPin {
    private(set) var isPinned = true

    mutating func update(distanceFromBottom: CGFloat) {
        isPinned = distanceFromBottom <= Layout.autoscrollPinThreshold
    }
}
