import SwiftUI

/// GoCode's selected D0 mark: a vertically symmetric ring with an
/// inward-pointing chevron. The ring opens at the right center rather than in
/// the upper-right, which is the defining geometry of the reference.
///
/// Drawn rather than shipped as an image so the project picker and Dock icon
/// share one resolution-independent source of truth.
public struct BrandMark: Shape {
    /// Stroke width as a fraction of the drawing square's side.
    public static let strokeRatio: CGFloat = 0.082
    /// D0's ring radius as a fraction of the drawing square's side.
    public static let radiusRatio: CGFloat = 0.235
    /// Half-angle from the horizontal to D0's right-side ring endpoints.
    public static let ringEndpointAngleDegrees: Double = 17.5
    /// Chevron positions as fractions of the ring radius / drawing square.
    public static let chevronInnerRatio: CGFloat = 7.0 / 23.5
    public static let chevronOuterRatio: CGFloat = 24.0 / 23.5
    public static let chevronSpreadRatio: CGFloat = 0.075

    public init() {}

    public func path(in rect: CGRect) -> Path {
        let side = min(rect.width, rect.height)
        let center = CGPoint(x: rect.midX, y: rect.midY)
        let radius = side * Self.radiusRatio

        var path = Path()
        // SwiftUI's clockwise arc direction produces D0's long ring arc:
        // upper-right endpoint -> around the left -> lower-right endpoint.
        path.addArc(
            center: center,
            radius: radius,
            startAngle: .degrees(-Self.ringEndpointAngleDegrees),
            endAngle: .degrees(Self.ringEndpointAngleDegrees),
            clockwise: true)

        let barOuter = center.x + radius * Self.chevronOuterRatio
        let barInner = center.x + radius * Self.chevronInnerRatio
        let barSpread = side * Self.chevronSpreadRatio
        path.move(to: CGPoint(x: barInner, y: center.y - barSpread))
        path.addLine(to: CGPoint(x: barOuter, y: center.y))
        path.addLine(to: CGPoint(x: barInner, y: center.y + barSpread))

        return path
    }
}

/// The D0 mark as a view, stroked in the current foreground style.
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

/// The D0 mark on its tile, used for the Dock icon.
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
    /// Renders the same D0 tile used by the app as its runtime Dock icon.
    @MainActor
    public static func appIcon(side: CGFloat = 256) -> NSImage? {
        let renderer = ImageRenderer(content: BrandTileView(side: side))
        renderer.scale = 2
        guard let cgImage = renderer.cgImage else { return nil }
        return NSImage(cgImage: cgImage, size: NSSize(width: side, height: side))
    }
}
#endif
