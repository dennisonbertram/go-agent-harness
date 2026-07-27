import Foundation
import Testing

@testable import HarnessKit

/// Integration tests against a real harnessd process.
///
/// Skipped unless `HARNESS_TEST_BASE_URL` is set, so `swift test` stays
/// hermetic by default. Start a key-free server and point the tests at it:
///
///     scripts/live-harnessd.sh          # starts on 127.0.0.1:8899
///     HARNESS_TEST_BASE_URL=http://127.0.0.1:8899 swift test
///
/// These exist because stubbed transport proves the client is self-consistent,
/// not that it agrees with the server.
@Suite("live harnessd", .serialized)
struct LiveHarnessTests {

    private static var baseURL: URL? {
        ProcessInfo.processInfo.environment["HARNESS_TEST_BASE_URL"].flatMap(URL.init(string:))
    }

    private func client() throws -> HarnessClient {
        let url = try #require(Self.baseURL, "set HARNESS_TEST_BASE_URL to run live tests")
        return HarnessClient(baseURL: url)
    }

    @Test("drives a real run to completion over SSE", .enabled(if: baseURL != nil))
    func realRunToCompletion() async throws {
        let client = try client()

        var request = HarnessClient.StartRunRequest(prompt: "list the workspace")
        // The key-free fake provider is only reachable via default-provider
        // fallback, since model->provider resolution otherwise wins.
        request.allowFallback = true
        let started = try await client.startRun(request)
        #expect(started.runID.hasPrefix("run_"))

        var types: [HarnessEventType] = []
        var toolNames: [String] = []
        var assistantText = ""

        for try await event in client.events(runID: started.runID) {
            types.append(event.type)
            if event.type == .toolCallStarted,
                let tool = event.payload["tool"]?.stringValue
            {
                toolNames.append(tool)
            }
            if event.type == .assistantMessage,
                let content = event.payload["content"]?.stringValue
            {
                assistantText = content
            }
        }

        // The stream must terminate on its own — a client that never sees a
        // terminal event hangs the UI spinner forever.
        #expect(types.last?.isTerminal == true)
        #expect(types.contains(.runStarted))
        #expect(types.contains(.runCompleted), "run did not complete: \(types)")

        // Exact turn content is NOT asserted here: the fake provider advances a
        // single global turn cursor shared by every run on the server, so which
        // scripted turn a given run receives depends on test order. Content
        // assertions live in the golden-fixture replay tests, which are
        // deterministic. What must hold here is the integration contract:
        // every tool call the server announces is one the client can name.
        #expect(toolNames.allSatisfy { !$0.isEmpty })
        _ = assistantText
    }

    @Test("cancels an in-flight run", .enabled(if: baseURL != nil))
    func cancelsRun() async throws {
        let client = try client()
        var request = HarnessClient.StartRunRequest(prompt: "list the workspace")
        request.allowFallback = true
        let started = try await client.startRun(request)

        // Cancel is idempotent and safe even if the run already finished.
        try await client.cancel(runID: started.runID)
    }

    @Test("reports the server's error for an invalid run", .enabled(if: baseURL != nil))
    func rejectsEmptyPrompt() async throws {
        let client = try client()
        do {
            _ = try await client.startRun(.init(prompt: ""))
            Issue.record("expected harnessd to reject an empty prompt")
        } catch let error as HarnessError {
            #expect(error.statusCode == 400)
            #expect(error.message.contains("prompt"))
        }
    }
}
