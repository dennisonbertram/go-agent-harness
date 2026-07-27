// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "GoCode",
    platforms: [.macOS(.v14)],
    products: [
        .library(name: "HarnessKit", targets: ["HarnessKit"]),
        .library(name: "GoCodeUI", targets: ["GoCodeUI"]),
        .executable(name: "GoCode", targets: ["GoCodeApp"]),
    ],
    targets: [
        // Transport + domain model for the harnessd HTTP/SSE contract.
        // No SwiftUI import here: this target must stay headlessly testable.
        .target(name: "HarnessKit"),
        .target(name: "GoCodeUI", dependencies: ["HarnessKit"]),
        .executableTarget(name: "GoCodeApp", dependencies: ["GoCodeUI"]),
        .testTarget(
            name: "HarnessKitTests",
            dependencies: ["HarnessKit"],
            resources: [.copy("Fixtures")]
        ),
        .testTarget(name: "GoCodeUITests", dependencies: ["GoCodeUI"]),
    ]
)
