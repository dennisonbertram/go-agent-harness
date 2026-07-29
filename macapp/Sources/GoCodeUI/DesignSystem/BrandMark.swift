import SwiftUI

/// GoCode's mark: a ring opened at the right with a chevron reaching into it.
///
/// Drawn rather than shipped as an image so one definition serves the dock
/// icon, the wordmark and empty states, stays sharp at every size, and needs no
/// asset catalog — this target has none.
///
/// Geometry is expressed as fractions of the drawing square so the mark is
/// resolution-independent; the only absolute is the stroke ratio, which is
/// tuned so the counters stay open at 16pt where a dock icon actually lives.
public struct BrandMark: Shape {
    /// Stroke width as a fraction of the square's side.
    public static let strokeRatio: CGFloat = 0.082
    /// Ring radius as a fraction of the square's side.
    public static let radiusRatio: CGFloat = 0.235

    public init() {}

    public func path(in rect: CGRect) -> Path {
        let side = min(rect.width, rect.height)
        let center = CGPoint(x: rect.midX, y: rect.midY)
        let radius = side * Self.radiusRatio

        var path = Path()
        // The ring, opened on the right. Angles are measured the SwiftUI way
        // (clockwise from three o'clock, y down), so the gap sits where the
        // chevron enters.
        path.addArc(
            center: center,
            radius: radius,
            startAngle: .degrees(-32),
            endAngle: .degrees(292),
            clockwise: false)

        // The chevron: the G's bar, pointing inward. Deliberately not a
        // straight horizontal crossbar — that reads as a different well-known
        // mark entirely.
        let barOuter = center.x + radius * 1.02
        let barInner = center.x + radius * 0.30
        let barSpread = side * 0.075
        path.move(to: CGPoint(x: barInner, y: center.y - barSpread))
        path.addLine(to: CGPoint(x: barOuter, y: center.y))
        path.addLine(to: CGPoint(x: barInner, y: center.y + barSpread))

        return path
    }
}

/// The mark as a view, stroked in the current foreground style.
public struct BrandMarkView: View {
    private let side: CGFloat

    public init(side: CGFloat) {
        self.side = side
    }

    public var body: some View {
        BrandMark()
            .stroke(
                style: StrokeStyle(
                    lineWidth: side * BrandMark.strokeRatio,
                    lineCap: .round,
                    lineJoin: .round)
            )
            .frame(width: side, height: side)
            .accessibilityHidden(true)
    }
}

/// The mark on its tile, as the app icon. Separate from `BrandMarkView`
/// because in-app uses (wordmark, empty state) sit on the app's own surface
/// and must not carry a second background.
public struct BrandTileView: View {
    private let side: CGFloat

    public init(side: CGFloat) {
        self.side = side
    }

    public var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: side * 0.22, style: .continuous)
                .fill(Theme.surfaceElevated)
            BrandMarkView(side: side)
                .foregroundStyle(Theme.foreground)
        }
        .frame(width: side, height: side)
        .accessibilityHidden(true)
    }
}

#if canImport(AppKit)
import AppKit

extension BrandMarkView {
    /// Renders the tile as an NSImage for the Dock.
    ///
    /// An SPM executable has no bundle and therefore no icon slot, so the
    /// Dock shows a generic placeholder unless the icon is set at runtime.
    /// Rendering from the same Shape as the in-app mark means the two
    /// cannot drift.
    @MainActor
    public static func appIcon(side: CGFloat = 256) -> NSImage? {
        let renderer = ImageRenderer(content: BrandTileView(side: side))
        renderer.scale = 2
        guard let cgImage = renderer.cgImage else { return nil }
        return NSImage(
            cgImage: cgImage, size: NSSize(width: side, height: side))
    }
}
#endif
