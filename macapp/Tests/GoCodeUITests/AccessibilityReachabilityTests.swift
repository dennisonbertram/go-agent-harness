import Foundation
import Testing

@testable import GoCodeUI

/// Exercises the fix for #999 (F8, R9): conversation rows and model rows were
/// `onTapGesture` targets rather than controls, the per-model exposure toggle
/// had no accessibility name, and provider removal fired immediately. Follows
/// `TranscriptFeatureReachabilityTests`' pattern: scan production source for
/// the shape of the defect and the shape of the fix, since a `Button` versus
/// an `onTapGesture`, an `.accessibilityLabel`, and an alert presentation are
/// none of them assertable through a headless view-rendering test on this
/// stack.
@Suite("Accessibility and settings-feedback reachability")
struct AccessibilityReachabilityTests {

    @Test("the conversation row is a real control, not an onTapGesture target")
    func conversationRowIsARealControl() throws {
        let contents = try ReachabilitySource.file("SessionsView.swift")
        #expect(!contents.contains(".onTapGesture"))
        #expect(contents.contains(".accessibilityLabel("))
    }

    @Test("the model row is a real control, not an onTapGesture target")
    func modelRowIsARealControl() throws {
        let contents = try ReachabilitySource.file("SettingsView.swift")
        #expect(!contents.contains(".onTapGesture { project.selectedModel = model.id }"))
        #expect(contents.contains(".accessibilityLabel("))
    }

    /// Regression angle distinct from the "no .onTapGesture" check above:
    /// that check alone would still pass if the row were left an inert,
    /// unfocusable view with only a bystander `.accessibilityLabel`
    /// somewhere else in the file. This pins that the label is attached to
    /// an actual `Button` wrapping `ConversationRow`, which is what restores
    /// keyboard focus and Return activation -- the actual defect (#999
    /// finding 1), not merely the absence of the old gesture.
    @Test(
        "the conversation row's accessibility label is attached to a real Button, not a bystander"
    )
    func conversationRowAccessibilityLabelIsOnARealButton() throws {
        let contents = try ReachabilitySource.file("SessionsView.swift")
        let pattern =
            #"Button \{[\s\S]*?\}\s*label:\s*\{\s*ConversationRow\(conversation: conversation\)\s*\}\s*\n\s*\.buttonStyle\(\.plain\)\s*\n\s*\.accessibilityLabel\("#
        #expect(contents.range(of: pattern, options: .regularExpression) != nil)
    }

    /// Same reasoning as above, for the model row.
    @Test("the model row's accessibility label is attached to a real Button, not a bystander")
    func modelRowAccessibilityLabelIsOnARealButton() throws {
        let contents = try ReachabilitySource.file("SettingsView.swift")
        let pattern =
            #"Button \{\s*project\.selectedModel = model\.id\s*\}\s*label:\s*\{[\s\S]*?\}\s*\n\s*\.buttonStyle\(\.plain\)\s*\n\s*\.accessibilityLabel\("#
        #expect(contents.range(of: pattern, options: .regularExpression) != nil)
    }

    @Test("the exposure toggle carries an accessibility label naming the model")
    func exposureToggleIsNamed() throws {
        let contents = try ReachabilitySource.file("ModelSettingsView.swift")
        #expect(contents.contains(".accessibilityLabel(\"Show \\(entry.modelID) in the picker\")"))
        // `.labelsHidden()` is a layout choice; it must not also drop the name.
        #expect(contents.contains(".labelsHidden()"))
    }

    /// #999's finding was that `Remove` fired `model.delete` the instant it
    /// was tapped. Mirrors `DestructiveConfirmationTests`' per-callsite check:
    /// counting occurrences of the helper catches a reverted call site that
    /// left a now-dangling helper declaration behind, which a bare
    /// `contains("confirmRemove")` check would miss.
    @Test("provider Remove routes through confirmRemove, not an immediate delete")
    func providerRemoveRoutesThroughConfirmRemove() throws {
        let contents = try ReachabilitySource.file("ModelSettingsView.swift")
        #expect(occurrences(of: "confirmRemove(", in: contents) >= 2)
        #expect(contents.contains(".destructiveConfirmation("))
    }

    /// Distinct from the two file-specific tests above: this is the exact
    /// shape of the defect (#991 finding 8) rather than a named file, so a
    /// third instance introduced anywhere in the module is still caught.
    @Test("no row in the module pairs .contentShape(.rect) with .onTapGesture")
    func noRowPairsContentShapeWithTapGesture() throws {
        let source = try ReachabilitySource.wholeModule()
        let pattern = #"\.contentShape\(\.rect\)\s*\n\s*\.onTapGesture"#
        #expect(source.range(of: pattern, options: .regularExpression) == nil)
    }

    // MARK: - Helpers

    private func occurrences(of needle: String, in haystack: String) -> Int {
        haystack.components(separatedBy: needle).count - 1
    }
}
