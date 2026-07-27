import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Exercises the whole app path against a real harnessd — composer state →
/// HTTP client → live SSE → transcript reducer → the state the views render.
///
/// This is the "watch it work" check. It is deliberately at the view-model
/// layer rather than the pixel layer: it asserts the same state the SwiftUI
/// views bind to, and unlike a screenshot it runs in CI.
///
///     macapp/scripts/live-harnessd.sh
///     HARNESS_TEST_BASE_URL=http://127.0.0.1:8899 swift test
/// File scope, not a static member: `@MainActor` isolation on the suite would
/// otherwise put it out of reach of the `.enabled(if:)` trait.
private let liveBaseURL: URL? = ProcessInfo.processInfo
    .environment["HARNESS_TEST_BASE_URL"].flatMap(URL.init(string:))

@Suite("RunSession against a live harnessd", .serialized)
@MainActor
struct RunSessionLiveTests {

    /// Polls until `condition` holds, so the test tracks the real stream rather
    /// than sleeping for a guessed duration.
    private func wait(
        timeout: Duration = .seconds(20), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(50))
        }
        Issue.record("timed out waiting for condition")
    }

    @Test(
        "submitting a prompt drives a real run to a rendered transcript",
        .enabled(if: liveBaseURL != nil))
    func submitProducesTranscript() async throws {
        let session = RunSession(baseURL: try #require(liveBaseURL))

        #expect(session.canSubmit == false, "empty composer must not submit")
        session.draft = "list the workspace"
        #expect(session.canSubmit)

        session.submit()

        // The composer clears immediately so the user can keep typing.
        #expect(session.draft.isEmpty)

        try await wait { session.transcript.runState == .completed }

        // Exact transcript shape is not asserted: the fake provider's turn
        // cursor is global to the server, so a run's scripted content depends
        // on test order. Deterministic shape is covered by the golden-fixture
        // replay tests. Here we assert the invariants a user would notice.
        let items = session.transcript.items
        #expect(items.count >= 2, "expected at least a prompt and a reply")

        guard case .userPrompt(let prompt) = items.first?.kind else {
            Issue.record("first item should be the user prompt")
            return
        }
        #expect(prompt == "list the workspace")

        let tools = items.compactMap { item -> ToolActivity? in
            if case .toolActivity(let activity) = item.kind { return activity }
            return nil
        }
        // No tool row may be left spinning once the run is over.
        #expect(tools.allSatisfy { $0.status != .running }, "orphaned tool row")
        #expect(tools.allSatisfy { !$0.tool.isEmpty })

        let streaming = items.contains { item in
            if case .assistantMessage(let message) = item.kind { return message.isStreaming }
            return false
        }
        #expect(streaming == false, "a stuck streaming flag spins forever")

        // Spinner and composer must both free up once the run ends.
        #expect(session.isBusy == false)
        #expect(session.connectionError == nil)
        #expect(session.transcript.usage.totalTokens > 0)
    }

    /// If harnessd is unreachable the UI must land in a terminal state with a
    /// visible reason — not spin forever on a dead socket.
    @Test("a dead server surfaces an error instead of spinning")
    func deadServerFailsVisibly() async throws {
        // Port 1 is reserved and never listening.
        let session = RunSession(baseURL: URL(string: "http://127.0.0.1:1")!)
        session.draft = "hello"
        session.submit()

        // The session goes busy synchronously on submit, so waiting on
        // `!isBusy` alone would pass instantly at t=0 without proving anything.
        #expect(session.isBusy, "composer must lock as soon as a run is submitted")
        try await wait(timeout: .seconds(15)) {
            session.connectionError != nil || session.transcript.runState == .failed
        }

        #expect(session.transcript.runState == .failed)
        #expect(session.connectionError != nil, "the failure reason must be shown")
    }
}
