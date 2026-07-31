import Foundation
import Testing

@testable import GoCodeUI

@Suite("Transcript scroll pin")
struct TranscriptScrollPinTests {

    @Test("a stale autoscroll completion cannot expose geometry during a newer scroll")
    func staleAutoscrollCompletionDoesNotEndNewerSuppression() {
        var state = TranscriptAutoscrollState()
        let firstGeneration = state.begin(animated: true)
        let secondGeneration = state.begin(animated: true)

        state.finish(generation: firstGeneration)
        #expect(state.suppressesGeometryUpdates)

        state.finish(generation: secondGeneration)
        #expect(!state.suppressesGeometryUpdates)
    }

    @Test("a Reduce Motion scroll does not suppress geometry updates")
    func reduceMotionAutoscrollDoesNotSuppressGeometry() {
        var state = TranscriptAutoscrollState()
        _ = state.begin(animated: false)

        #expect(!state.suppressesGeometryUpdates)
    }

    @Test("a fresh pin starts pinned to the bottom")
    func startsPinned() {
        let pin = TranscriptScrollPin()
        #expect(pin.isPinned)
    }

    @Test("staying at distance 0 keeps the pin pinned")
    func staysPinnedAtBottom() {
        var pin = TranscriptScrollPin()
        pin.update(distanceFromBottom: 0)
        #expect(pin.isPinned)
    }

    @Test("scrolling past the threshold unpins autoscroll")
    func unpinsPastThreshold() {
        var pin = TranscriptScrollPin()
        pin.update(distanceFromBottom: Layout.autoscrollPinThreshold + 1)
        #expect(!pin.isPinned)
    }

    @Test("distance exactly at the threshold is still pinned")
    func boundaryIsInclusive() {
        var pin = TranscriptScrollPin()
        pin.update(distanceFromBottom: Layout.autoscrollPinThreshold)
        #expect(pin.isPinned)
    }

    @Test("scrolling back to the bottom re-pins autoscroll")
    func rePinsOnReturnToBottom() {
        var pin = TranscriptScrollPin()
        pin.update(distanceFromBottom: Layout.autoscrollPinThreshold + 1)
        #expect(!pin.isPinned)
        pin.update(distanceFromBottom: 0)
        #expect(pin.isPinned)
    }

    @Test("overscroll bounce reports a negative distance and stays pinned")
    func negativeDistanceStaysPinned() {
        var pin = TranscriptScrollPin()
        pin.update(distanceFromBottom: -12)
        #expect(pin.isPinned)
    }

    @Test("following latest re-pins after the operator has scrolled away")
    func followingLatestRePins() {
        var pin = TranscriptScrollPin()
        pin.update(distanceFromBottom: Layout.autoscrollPinThreshold + 1)
        #expect(!pin.isPinned)

        pin.followLatest()
        #expect(pin.isPinned)
    }
}
