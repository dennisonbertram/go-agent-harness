import SwiftUI
import Testing

@testable import GoCodeUI

@Suite("Non-colour design tokens")
struct DesignTokenTests {

    @Test("the brand mark uses the selected D0 ring and inward chevron")
    func brandMarkUsesD0Geometry() {
        let path = BrandMark().path(in: CGRect(x: 0, y: 0, width: 100, height: 100))
        var elements: [Path.Element] = []
        path.forEach { elements.append($0) }

        guard elements.count == 8,
            case let .move(to: ringStart) = elements[0],
            case let .curve(to: ringEnd, control1: _, control2: _) = elements[4],
            case let .move(to: chevronStart) = elements[5],
            case let .line(to: chevronPoint) = elements[6],
            case let .line(to: chevronEnd) = elements[7]
        else {
            Issue.record("BrandMark path no longer has the expected D0 arc and chevron elements")
            return
        }

        // D0's SVG reference uses a 23.5pt ring centered at (50, 50), with
        // right-side endpoints at approximately (72.3, 43) and (72.3, 57).
        #expect(abs(ringStart.x - 72.4) < 0.15)
        #expect(abs(ringStart.y - 43.0) < 0.15)
        #expect(abs(ringEnd.x - 72.4) < 0.15)
        #expect(abs(ringEnd.y - 57.0) < 0.15)

        // The chevron is mirrored around the vertical centerline and points
        // inward from the right edge of the ring.
        #expect(chevronStart == CGPoint(x: 57, y: 42.5))
        #expect(chevronPoint == CGPoint(x: 74, y: 50))
        #expect(chevronEnd == CGPoint(x: 57, y: 57.5))
    }

    @Test("spacing keeps the established layout rhythm")
    func spacingRhythm() {
        #expect(Spacing.none == 0)
        #expect(Spacing.tight == 2)
        #expect(Spacing.compact == 4)
        #expect(Spacing.small == 6)
        #expect(Spacing.standard == 8)
        #expect(Spacing.comfortable == 10)
        #expect(Spacing.inset == 12)
        #expect(Spacing.large == 16)
        #expect(Spacing.section == 18)
    }

    @Test("shape and icon roles preserve their current measurements")
    func shapeAndIconMeasurements() {
        #expect(CornerRadius.tag == 4)
        #expect(CornerRadius.code == 6)
        #expect(CornerRadius.control == 8)
        #expect(CornerRadius.card == 10)
        #expect(CornerRadius.composer == 20)
        #expect(IconSize.status == 7)
        #expect(IconSize.detail == 14)
        #expect(IconSize.standard == 15)
        #expect(IconSize.row == 18)
        #expect(IconSize.emptyState == 30)
        #expect(IconSize.launch == 44)
    }

    /// The environment inspector is a compact card, not a window-height split.
    /// Keeping its footprint in the token layer prevents a future layout pass
    /// from quietly turning it back into a sidebar.
    ///
    /// 361pt was the reference's own width, but the reference floats its card
    /// over a pane that is only 64% filled. This app fills 94%, so at that
    /// width the card covered the transcript it describes. It is now a sibling
    /// column at 60% of that width.
    @Test("environment inspector retains its compact card footprint")
    func environmentInspectorFootprint() {
        #expect(Layout.inspectorCardWidth == 217)
        // Narrow enough that opening it leaves the transcript readable.
        #expect(Layout.inspectorCardWidth < Layout.chatContentMaximumWidth / 3)
    }

    /// Five compact rows once needed 249pt inside the rail's 204pt content
    /// width. The overflow widened the column and pushed every row off-centre,
    /// so the selected pill rendered 1pt from the left edge and 15pt from the
    /// right. This pins the arithmetic that keeps the footer inside its column.
    @Test("the compact rail footer fits inside the rail's content width")
    func compactRailFooterFits() {
        let iconRow = IconSize.row
        let compactRowWidth = iconRow + (Spacing.small * 2)
        let footer = (compactRowWidth * 5) + (Spacing.tight * 4)
        let available = Layout.railWidth - (Spacing.standard * 2)
        #expect(footer <= available)
    }

    @Test("loading placeholders use named row and inline-control geometry")
    func loadingGeometryIsTokenized() {
        #expect(Layout.loadingRowHeight > Spacing.large)
        #expect(Layout.inlineActivitySlot == IconSize.standard)
    }

    /// The user-message row tracks its text role and vertical rhythm instead
    /// of preserving an unexplained screenshot measurement.
    @Test("transcript message height derives from its type and spacing roles")
    func transcriptMessageSurface() {
        #expect(
            Layout.userMessageMinimumHeight
                == Typography.bodyLineHeight + (Spacing.userMessageVertical * 2))
        #expect(Layout.userMessageMinimumHeight == 45.5)
        #expect(Layout.userMessageMaximumWidth == 374.5)
        #expect(Typography.bodyPointSize == 16.5)
        #expect(Typography.bodyLineHeight == 21.5)
        #expect(Theme.messageSurfaceLevel.dark == RGB(r: 36, g: 36, b: 36))
        #expect(Theme.messageSurfaceLevel.dark.isNeutral)
    }

    @Test("selected rail rows use the neutral selected-row tokens")
    func selectedRailTokens() {
        #expect(Theme.selectedRowSurfaceLevel.dark == RGB(r: 51, g: 51, b: 51))
        #expect(Theme.selectedRowSurfaceLevel.dark.isNeutral)
        #expect(Theme.selectedRowForegroundLevel.dark == RGB(r: 255, g: 255, b: 255))
    }

    @Test("conversation layout tokens share the Codex column and quieter divider")
    func conversationLayout() {
        #expect(Layout.chatContentMaximumWidth == 883)
        #expect(Spacing.conversationHeaderHeight == 52)
        #expect(Spacing.transcriptTop == 65.5)
        #expect(Theme.separatorLevel.dark == RGB(r: 43, g: 43, b: 43))
    }
}

extension DesignTokenTests {
    /// The mark is drawn, not shipped as an image, so its geometry is the
    /// thing that can regress. These pin the two properties that decide
    /// whether it survives at 16pt, where a Dock icon actually lives.
    @Test("the brand mark keeps the stroke and radius that hold at small sizes")
    func brandMarkGeometry() {
        // A heavier stroke closes the ring's counter; a lighter one disappears.
        #expect(BrandMark.strokeRatio > 0.06)
        #expect(BrandMark.strokeRatio < 0.10)
        // The ring must leave room for the tile's corner radius around it.
        #expect(BrandMark.radiusRatio < 0.28)
    }

    /// Scale-independence is the reason for drawing it: the same Shape has to
    /// produce the same figure at a Dock icon's size and a menu glyph's.
    @Test("the brand mark scales without changing shape")
    func brandMarkIsScaleIndependent() {
        let smallRect = CGRect(x: 0, y: 0, width: 16, height: 16)
        let largeRect = CGRect(x: 0, y: 0, width: 512, height: 512)
        let small: CGRect = BrandMark().path(in: smallRect).boundingRect
        let large: CGRect = BrandMark().path(in: largeRect).boundingRect

        let smallWidthRatio: CGFloat = small.width / 16
        let largeWidthRatio: CGFloat = large.width / 512
        #expect(abs(smallWidthRatio - largeWidthRatio) < 0.01)
    }

    /// A non-square frame must not stretch it — the Dock and toolbars both
    /// hand views rectangles that are not exactly square.
    @Test("the brand mark stays square in a non-square frame")
    func brandMarkDoesNotStretch() {
        let rect = CGRect(x: 0, y: 0, width: 200, height: 80)
        let box: CGRect = BrandMark().path(in: rect).boundingRect
        let difference: CGFloat = abs(box.width - box.height)
        #expect(difference < 1.0)
    }
}
