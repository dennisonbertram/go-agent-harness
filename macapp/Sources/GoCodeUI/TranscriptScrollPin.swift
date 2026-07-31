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

    /// Restores the operator's explicit intent to follow the live end of the
    /// transcript. The view calls this before its programmatic scroll, so new
    /// stream deltas continue to follow after the operator chooses Jump to
    /// Latest.
    mutating func followLatest() {
        isPinned = true
    }
}

/// Owns the generation of the view's programmatic transcript scroll. Geometry
/// is ignored only for the matching animated scroll, so a completion from an
/// earlier streamed delta cannot expose mid-animation geometry from a later
/// one and accidentally unpin the transcript.
struct TranscriptAutoscrollState {
    private(set) var generation = 0
    private(set) var suppressesGeometryUpdates = false

    /// Starts a new programmatic scroll and invalidates any earlier completion.
    /// Reduce Motion scrolls directly, so it has no animation interval to
    /// suppress.
    mutating func begin(animated: Bool) -> Int {
        generation &+= 1
        suppressesGeometryUpdates = animated
        return generation
    }

    /// Ends suppression only when this is still the latest scroll generation.
    mutating func finish(generation: Int) {
        guard generation == self.generation else { return }
        suppressesGeometryUpdates = false
    }

    /// Invalidates a pending completion when the transcript view leaves the
    /// hierarchy.
    mutating func cancel() {
        generation &+= 1
        suppressesGeometryUpdates = false
    }
}
