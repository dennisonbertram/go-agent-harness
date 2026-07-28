import SwiftUI

/// Measures a transcript element at its readable maximum, then returns its
/// intrinsic width when it is shorter. A flexible frame would always accept
/// the transcript column's proposal and turn user prompts back into bands.
///
/// `SwiftUI.Layout` is spelled out because this module's design tokens include
/// an enum also named `Layout`, which otherwise wins name resolution here and
/// fails with the misleading "inheritance from non-protocol type".
struct ContentHuggingWidthLayout: SwiftUI.Layout {
    let maximumWidth: CGFloat

    func sizeThatFits(
        proposal: ProposedViewSize,
        subviews: LayoutSubviews,
        cache _: inout ()
    ) -> CGSize {
        guard let subview = subviews.first else { return .zero }
        let proposedWidth = min(proposal.width ?? maximumWidth, maximumWidth)
        return subview.sizeThatFits(
            ProposedViewSize(width: proposedWidth, height: proposal.height))
    }

    func placeSubviews(
        in bounds: CGRect,
        proposal _: ProposedViewSize,
        subviews: LayoutSubviews,
        cache _: inout ()
    ) {
        guard let subview = subviews.first else { return }
        subview.place(
            at: bounds.origin,
            anchor: .topLeading,
            proposal: ProposedViewSize(width: bounds.width, height: bounds.height))
    }
}
