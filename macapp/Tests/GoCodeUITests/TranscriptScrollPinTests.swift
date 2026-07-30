import Foundation
import Testing

@testable import GoCodeUI

@Suite("Transcript scroll pin")
struct TranscriptScrollPinTests {

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
}
