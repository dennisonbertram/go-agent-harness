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
    case failed(String)

    public func showsEmptyState(itemCount: Int) -> Bool {
        self == .loaded && itemCount == 0
    }

    /// A skeleton is truthful only while a result is still pending and
    /// nothing is on screen yet. `.failed` is deliberately excluded: showing
    /// a skeleton for a failure looks identical to a slow load that will
    /// finish momentarily, when nothing further is coming until the
    /// operator retries (#991 finding 2).
    public func showsPlaceholder(itemCount: Int) -> Bool {
        (self == .loading || self == .idle) && itemCount == 0
    }

    public var showsError: Bool {
        if case .failed = self { return true }
        return false
    }

    /// A first-load failure owns the region only when there is no truthful
    /// content to keep showing.
    public func showsBlockingError(itemCount: Int) -> Bool {
        showsError && itemCount == 0
    }

    /// A refresh failure is nonblocking when the prior successful rows remain
    /// available. The operator keeps their context and gets an explicit retry.
    public func showsRefreshError(itemCount: Int) -> Bool {
        showsError && itemCount > 0
    }

    /// The server's own reason, verbatim — nil for every other state.
    public var errorMessage: String? {
        if case .failed(let message) = self { return message }
        return nil
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

/// A failed collection fetch: the server's own reason plus a way to try
/// again. Mirrors `StartupFailureView` (`AppShell.swift`), the app's
/// existing "this failed, here is why, retry" shape at full-window scale —
/// this is the inline, per-collection version, so one bad request never
/// renders as an endless skeleton.
struct CollectionErrorState: View {
    let message: String
    let retry: () -> Void

    var body: some View {
        VStack(spacing: Spacing.standard) {
            Label(message, systemImage: "exclamationmark.triangle.fill")
                .font(Typography.caption)
                .foregroundStyle(.orange)
                .multilineTextAlignment(.center)
            Button("Retry", action: retry)
        }
        .frame(maxWidth: .infinity, alignment: .center)
        .padding(Spacing.inset)
    }
}

/// A compact refresh failure that sits beside the last successful rows instead
/// of replacing them.
struct CollectionRefreshErrorState: View {
    let message: String
    let retry: () -> Void

    var body: some View {
        HStack(spacing: Spacing.standard) {
            Label(message, systemImage: "exclamationmark.triangle.fill")
                .font(Typography.caption)
                .foregroundStyle(.orange)
                .lineLimit(2)
            Spacer()
            Button("Retry", action: retry)
        }
        .padding(.vertical, Spacing.compact)
    }
}
