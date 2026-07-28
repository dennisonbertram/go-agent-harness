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
        #expect(CornerRadius.composer == 14)
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
}
