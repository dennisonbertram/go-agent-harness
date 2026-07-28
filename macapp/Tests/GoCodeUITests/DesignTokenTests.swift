import Testing

@testable import GoCodeUI

@Suite("Non-colour design tokens")
struct DesignTokenTests {

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

    /// The environment inspector is an overlay card, not a window-height
    /// split. Keeping its measured footprint in the token layer prevents a
    /// future layout pass from quietly turning it back into a sidebar.
    @Test("environment inspector retains its compact card footprint")
    func environmentInspectorFootprint() {
        #expect(Layout.inspectorCardWidth == 361)
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
