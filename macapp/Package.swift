// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "GoCode",
    platforms: [.macOS(.v14)],
    products: [
        .library(name: "HarnessKit", targets: ["HarnessKit"]),
        .library(name: "GoCodeUI", targets: ["GoCodeUI"]),
        .executable(name: "GoCode", targets: ["GoCodeApp"]),
        .executable(name: "ToolWalk", targets: ["ToolWalk"]),
    ],
    targets: [
        // Transport + domain model for the harnessd HTTP/SSE contract.
        // No SwiftUI import here: this target must stay headlessly testable.
        .target(name: "HarnessKit"),
        .target(name: "GoCodeUI", dependencies: ["HarnessKit"]),
        .executableTarget(name: "GoCodeApp", dependencies: ["GoCodeUI"]),
        // Drives ProjectSession/RunSession headlessly (no window needed) to
        // walk every registered tool through the app's own client and
        // transcript code — see scripts/ui-walk-tools.txt.
        .executableTarget(name: "ToolWalk", dependencies: ["HarnessKit", "GoCodeUI"]),
        .testTarget(
            name: "HarnessKitTests",
            dependencies: ["HarnessKit"],
            resources: [.copy("Fixtures")]
        ),
        .testTarget(name: "GoCodeUITests", dependencies: ["GoCodeUI"]),
        .testTarget(name: "ToolWalkTests", dependencies: ["ToolWalk"]),
    ]
)
