import Foundation

// Entry point placeholder for the red commit: the real orchestration (seed a
// workspace, spawn harnessd via ProjectSession, walk every tool, judge the
// transcript) lands in the green commit. Named apart from `main.swift` so the
// module stays `@testable import`-able from ToolWalkTests.
@main
struct ToolWalkMain {
    static func main() async throws {
        fatalError("ToolWalk is not implemented yet")
    }
}
