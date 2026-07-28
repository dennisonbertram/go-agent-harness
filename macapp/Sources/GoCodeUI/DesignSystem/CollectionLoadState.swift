import SwiftUI

/// The result state of one fetched collection.
///
/// An empty array has no meaning until the request has completed successfully:
/// before then, rendering an empty message would make a temporary absence look
/// like a server fact. Views use `showsEmptyState(itemCount:)` rather than
/// reimplementing that distinction at every collection boundary.
public enum CollectionLoadState: Sendable, Equatable {
    case idle
    case loading
    case loaded
    case failed

    public func showsEmptyState(itemCount: Int) -> Bool {
        self == .loaded && itemCount == 0
    }
}

/// A quiet loading shape that keeps a region's eventual geometry in place.
/// It deliberately contains no progress affordance: collection refreshes are
/// background work, and a low-contrast pulse communicates that without
/// competing with the content the operator is already reading.
struct LoadingPlaceholder: View {
    let height: CGFloat
    let cornerRadius: CGFloat

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var isPulsing = false

    init(
        height: CGFloat = Layout.loadingRowHeight,
        cornerRadius: CGFloat = CornerRadius.control
    ) {
        self.height = height
        self.cornerRadius = cornerRadius
    }

    var body: some View {
        RoundedRectangle(cornerRadius: cornerRadius)
            .fill(Theme.foregroundQuaternary)
            .opacity(
                isPulsing && !reduceMotion
                    ? StateOpacity.loadingHighlight : StateOpacity.loadingBase
            )
            .frame(maxWidth: .infinity, minHeight: height, maxHeight: height)
            .accessibilityHidden(true)
            .onAppear(perform: updatePulse)
            .onChange(of: reduceMotion) { _, _ in updatePulse() }
    }

    private func updatePulse() {
        guard !reduceMotion else {
            isPulsing = false
            return
        }
        withAnimation(
            .easeInOut(duration: Motion.loadingPulseDuration)
                .repeatForever(autoreverses: true)
        ) {
            isPulsing = true
        }
    }
}
