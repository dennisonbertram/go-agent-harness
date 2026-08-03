import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

private final class ExternalRunControlStub: URLProtocol, @unchecked Sendable {
    private nonisolated(unsafe) static var recorded: [URLRequest] = []
    private static let lock = NSLock()

    static func reset() {
        lock.withLock { recorded = [] }
    }

    static var requests: [URLRequest] {
        lock.withLock { recorded }
    }

    override class func canInit(with _: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        let request = request
        Self.lock.withLock { Self.recorded.append(request) }
        let path = request.url?.path ?? ""
        let body: Data
        if path.hasSuffix("/input"), request.httpMethod == "GET" {
            let runID = path.split(separator: "/").dropLast().last.map(String.init) ?? "unknown"
            body = Data(
                #"{"run_id":"\#(runID)","call_id":"call_\#(runID)","questions":[{"question":"Continue?","options":[]}]}"#
                    .utf8
            )
        } else {
            body = Data("{}".utf8)
        }
        let response = HTTPURLResponse(
            url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"]
        )!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: body)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}

@Suite("RunSession external scheduled-run controls", .serialized)
@MainActor
struct RunSessionExternalControlTests {
    private func makeSession() -> RunSession {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [ExternalRunControlStub.self]
        return RunSession(
            client: HarnessClient(
                baseURL: URL(string: "http://127.0.0.1:8898")!,
                session: URLSession(configuration: configuration)
            )
        )
    }

    private func event(
        _ id: String, _ runID: String, _ type: String, timestamp: String? = nil,
        payload: String = "{}"
    ) throws -> HarnessEvent {
        let timestampField = timestamp.map { ",\"timestamp\":\"\($0)\"" } ?? ""
        return try HarnessEvent(
            frame: SSEFrame(
                id: id, event: type,
                data:
                    #"{"id":"\#(id)","run_id":"\#(runID)","type":"\#(type)"\#(timestampField),"payload":\#(payload)}"#
            )
        )
    }

    private func wait(
        timeout: Duration = .seconds(2), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("timed out waiting for condition")
    }

    /// The external run must become the single action target. A terminal for
    /// the older run and a replayed old non-terminal event cannot steal or
    /// clear that identity while the newer scheduled run is live.
    @Test("external scheduled run binds every action to its own run ID")
    func externalScheduledRunBindsActions() async throws {
        ExternalRunControlStub.reset()
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_a")

        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started"), conversationID: "conversation_a"
        )
        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started"), conversationID: "conversation_a"
        )
        try await session.applyConversationEvent(
            event("run_a:1", "run_a", "assistant.message"), conversationID: "conversation_a"
        )
        try await session.applyConversationEvent(
            event("run_a:2", "run_a", "run.completed"), conversationID: "conversation_a"
        )

        #expect(session.currentRunID == "run_b")
        #expect(session.isBusy, "run_a terminal must not make live run_b inactive")
        #expect(session.scheduledRunStatus == "Scheduled run active")

        try await session.applyConversationEvent(
            event(
                "run_b:1", "run_b", "tool.approval_required",
                payload: #"{"call_id":"call_b_1","tool":"bash","arguments":"{}"}"#
            ),
            conversationID: "conversation_a"
        )
        try await session.applyConversationEvent(
            event("run_b:2", "run_b", "run.waiting_for_user"), conversationID: "conversation_a"
        )
        #expect(session.currentRunID == "run_b")
        try await wait { session.pendingQuestions?.runID == "run_b" }

        session.approve()
        session.deny()
        session.draft = "continue watching"
        session.steer()
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/approve" }
        }
        #expect(session.runControlInFlight)
        #expect(
            !ExternalRunControlStub.requests.contains { $0.url?.path.hasSuffix("/deny") == true }
        )
        #expect(
            !ExternalRunControlStub.requests.contains { $0.url?.path.hasSuffix("/steer") == true }
        )
        try await session.applyConversationEvent(
            event("run_b:3", "run_b", "tool.approval_granted"), conversationID: "conversation_a"
        )
        try await wait { !session.runControlInFlight }

        try await session.applyConversationEvent(
            event(
                "run_b:4", "run_b", "tool.approval_required",
                payload: #"{"call_id":"call_b_2","tool":"bash","arguments":"{}"}"#
            ),
            conversationID: "conversation_a"
        )
        session.deny()
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/deny" }
        }
        try await session.applyConversationEvent(
            event("run_b:5", "run_b", "tool.approval_denied"), conversationID: "conversation_a"
        )
        try await wait { !session.runControlInFlight }

        session.answer(["0:Continue?": "yes"])
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/input" }
        }
        session.draft = "continue watching"
        session.steer()
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/steer" }
        }
        session.cancel()

        try await wait {
            let paths = Set(ExternalRunControlStub.requests.compactMap(\.url?.path))
            return [
                "/v1/runs/run_b/approve", "/v1/runs/run_b/deny", "/v1/runs/run_b/input",
                "/v1/runs/run_b/steer", "/v1/runs/run_b/cancel",
            ].allSatisfy(paths.contains)
        }
        session.reset()
    }

    /// A cancelled old stream can finish delivering after the user selects a
    /// different conversation. It must have no transcript or control effect.
    @Test("foreign conversation event is ignored")
    func foreignConversationEventIsIgnored() async throws {
        ExternalRunControlStub.reset()
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_b")

        try await session.applyConversationEvent(
            event("run_old:0", "run_old", "run.started"), conversationID: "conversation_a"
        )

        #expect(session.currentRunID == nil)
        #expect(session.transcript.runState == .completed)
        session.reset()
    }

    @Test("each selected terminal event clears only that run")
    func selectedTerminalEventsClearTheirRun() async throws {
        let expectations: [(String, RunState)] = [
            ("run.completed", .completed),
            ("run.failed", .failed),
            ("run.cancelled", .cancelled),
        ]
        for (type, expectedState) in expectations {
            ExternalRunControlStub.reset()
            let session = makeSession()
            session.load(messages: [], conversationID: "conversation_terminal")
            try await session.applyConversationEvent(
                event("run_terminal:0", "run_terminal", "run.started"),
                conversationID: "conversation_terminal"
            )
            try await session.applyConversationEvent(
                event("run_terminal:1", "run_terminal", type),
                conversationID: "conversation_terminal"
            )
            #expect(session.currentRunID == nil)
            #expect(session.transcript.runState == expectedState)
            session.reset()
        }
    }

    @Test("terminal tombstone prevents late replay resurrection")
    func terminalTombstonePreventsResurrection() async throws {
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_tombstone")
        try await session.applyConversationEvent(
            event("run_old:0", "run_old", "run.started"), conversationID: "conversation_tombstone"
        )
        try await session.applyConversationEvent(
            event("run_old:1", "run_old", "run.completed"), conversationID: "conversation_tombstone"
        )
        try await session.applyConversationEvent(
            event("run_old:2", "run_old", "run.started"), conversationID: "conversation_tombstone"
        )
        #expect(session.currentRunID == nil)
        #expect(!session.isBusy)
        session.reset()
    }

    @Test("first external active evidence resumes visible conversation")
    func firstExternalActiveEvidenceResumesConversation() async throws {
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_resume")
        try await session.applyConversationEvent(
            event("run_old:0", "run_old", "run.completed"), conversationID: "conversation_resume"
        )
        try await session.applyConversationEvent(
            event("run_scheduled:0", "run_scheduled", "assistant.message"),
            conversationID: "conversation_resume"
        )
        #expect(session.currentRunID == "run_scheduled")
        #expect(session.isBusy)
        #expect(session.scheduledRunStatus == "Scheduled run active")
        session.reset()
    }

    @Test("timestamp ordering rejects an older replay start")
    func timestampOrderingRejectsOlderReplayStart() async throws {
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_ordering")
        try await session.applyConversationEvent(
            event("run_new:0", "run_new", "run.started", timestamp: "2026-08-03T18:00:02Z"),
            conversationID: "conversation_ordering"
        )
        try await session.applyConversationEvent(
            event("run_old:0", "run_old", "run.started", timestamp: "2026-08-03T18:00:01Z"),
            conversationID: "conversation_ordering"
        )
        #expect(session.currentRunID == "run_new")
        #expect(session.accountingRunID == "run_new")
        session.reset()
    }

    /// A newer selected continuation cannot inherit approval UI from the
    /// displaced run. The visible button must retain A's origin and never
    /// authorize B.
    @Test("newer run clears displaced approval before it can target B")
    func newerRunClearsDisplacedApproval() async throws {
        ExternalRunControlStub.reset()
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_approval_owner")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started", timestamp: "2026-08-03T18:00:01Z"),
            conversationID: "conversation_approval_owner"
        )
        try await session.applyConversationEvent(
            event(
                "run_a:1", "run_a", "tool.approval_required",
                payload: #"{"call_id":"call_a","tool":"bash","arguments":"{}"}"#
            ),
            conversationID: "conversation_approval_owner"
        )
        let approval = try #require(session.transcript.pendingApproval)
        #expect(approval.runID == "run_a")

        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started", timestamp: "2026-08-03T18:00:02Z"),
            conversationID: "conversation_approval_owner"
        )

        #expect(session.currentRunID == "run_b")
        #expect(session.transcript.pendingApproval == nil)
        session.approve(expectedRunID: approval.runID)
        try await Task.sleep(for: .milliseconds(50))
        #expect(
            !ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/approve" }
        )
        session.reset()
    }

    @Test("newer run clears displaced plan and input")
    func newerRunClearsDisplacedPlanAndInput() async throws {
        ExternalRunControlStub.reset()
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_plan_input_owner")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started", timestamp: "2026-08-03T18:00:01Z"),
            conversationID: "conversation_plan_input_owner"
        )
        try await session.applyConversationEvent(
            event(
                "run_a:1", "run_a", "plan.approval_required",
                payload: #"{"plan":"A plan","options":[]}"#
            ),
            conversationID: "conversation_plan_input_owner"
        )
        try await session.applyConversationEvent(
            event("run_a:2", "run_a", "run.waiting_for_user"),
            conversationID: "conversation_plan_input_owner"
        )
        try await wait { session.pendingQuestions?.runID == "run_a" }
        let plan = try #require(session.transcript.pendingPlan)
        let prompt = try #require(session.pendingQuestions)
        #expect(plan.runID == "run_a")

        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started", timestamp: "2026-08-03T18:00:02Z"),
            conversationID: "conversation_plan_input_owner"
        )

        #expect(session.currentRunID == "run_b")
        #expect(session.transcript.pendingPlan == nil)
        #expect(session.pendingQuestions == nil)
        session.approve(expectedRunID: plan.runID)
        session.answer(["0:Continue?": "yes"], expectedRunID: prompt.runID)
        try await Task.sleep(for: .milliseconds(50))
        let bPaths = ExternalRunControlStub.requests.compactMap(\.url?.path)
        #expect(!bPaths.contains("/v1/runs/run_b/approve"))
        #expect(!bPaths.contains("/v1/runs/run_b/input"))
        session.reset()
    }

    @Test("selected terminal clears every interaction when no fallback exists")
    func selectedTerminalClearsInteractionState() async throws {
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_terminal_owner")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started"), conversationID: "conversation_terminal_owner"
        )
        try await session.applyConversationEvent(
            event(
                "run_a:1", "run_a", "tool.approval_required",
                payload: #"{"call_id":"call_a","tool":"bash","arguments":"{}"}"#
            ),
            conversationID: "conversation_terminal_owner"
        )
        try await session.applyConversationEvent(
            event("run_a:2", "run_a", "run.waiting_for_user"),
            conversationID: "conversation_terminal_owner"
        )
        try await wait { session.pendingQuestions?.runID == "run_a" }

        try await session.applyConversationEvent(
            event("run_a:3", "run_a", "run.completed"),
            conversationID: "conversation_terminal_owner"
        )

        #expect(session.currentRunID == nil)
        #expect(session.transcript.pendingApproval == nil)
        #expect(session.transcript.pendingPlan == nil)
        #expect(session.pendingQuestions == nil)
        session.reset()
    }

    @Test("selected terminal clears interactions before selecting an active fallback")
    func selectedTerminalFallbackClearsInteractionState() async throws {
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_terminal_fallback_owner")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started"),
            conversationID: "conversation_terminal_fallback_owner"
        )
        try await session.applyConversationEvent(
            event(
                "run_a:1", "run_a", "plan.approval_required",
                payload: #"{"plan":"A plan","options":[]}"#
            ),
            conversationID: "conversation_terminal_fallback_owner"
        )
        // B is active but cannot take selection until A ends because neither
        // lifecycle frame supplies a comparable timestamp.
        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started"),
            conversationID: "conversation_terminal_fallback_owner"
        )
        #expect(session.currentRunID == "run_a")

        try await session.applyConversationEvent(
            event("run_a:2", "run_a", "run.completed"),
            conversationID: "conversation_terminal_fallback_owner"
        )

        #expect(session.currentRunID == "run_b")
        #expect(session.transcript.pendingApproval == nil)
        #expect(session.transcript.pendingPlan == nil)
        #expect(session.pendingQuestions == nil)
        session.reset()
    }

    @Test("foreign terminal preserves selected run interaction state")
    func foreignTerminalPreservesSelectedInteractionState() async throws {
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_foreign_terminal")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started", timestamp: "2026-08-03T18:00:01Z"),
            conversationID: "conversation_foreign_terminal"
        )
        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started", timestamp: "2026-08-03T18:00:02Z"),
            conversationID: "conversation_foreign_terminal"
        )
        try await session.applyConversationEvent(
            event(
                "run_b:1", "run_b", "tool.approval_required",
                payload: #"{"call_id":"call_b","tool":"bash","arguments":"{}"}"#
            ),
            conversationID: "conversation_foreign_terminal"
        )
        try await session.applyConversationEvent(
            event("run_a:1", "run_a", "run.completed"),
            conversationID: "conversation_foreign_terminal"
        )

        #expect(session.currentRunID == "run_b")
        #expect(session.transcript.pendingApproval?.runID == "run_b")
        session.reset()
    }

    @Test("a stale Stop captured for A cannot cancel or force-stop newer B")
    func staleStopDoesNotTargetNewerRun() async throws {
        ExternalRunControlStub.reset()
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_stale_stop")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started", timestamp: "2026-08-03T18:00:01Z"),
            conversationID: "conversation_stale_stop"
        )
        let renderedRunID = try #require(session.currentRunID)
        // First press is a real A cancel and receives its acknowledgement.
        // The retained second-press closure is the force-stop hazard: it must
        // not locally abandon B after a continuation selects it.
        session.cancel(expectedRunID: renderedRunID)
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_a/cancel" }
        }
        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started", timestamp: "2026-08-03T18:00:02Z"),
            conversationID: "conversation_stale_stop"
        )
        #expect(session.currentRunID == "run_b")

        session.cancel(expectedRunID: renderedRunID)
        try await Task.sleep(for: .milliseconds(50))
        #expect(
            !ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/cancel" }
        )
        #expect(session.currentRunID == "run_b")
        #expect(session.transcript.runState != .cancelled)

        session.cancel(expectedRunID: "run_b")
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/cancel" }
        }
        #expect(session.connectionError == nil)
        session.reset()
    }

    @Test("a stale Composer steer captured for A leaves newer B and its draft untouched")
    func staleSteerDoesNotTargetNewerRun() async throws {
        ExternalRunControlStub.reset()
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation_stale_steer")
        try await session.applyConversationEvent(
            event("run_a:0", "run_a", "run.started", timestamp: "2026-08-03T18:00:01Z"),
            conversationID: "conversation_stale_steer"
        )
        let renderedRunID = try #require(session.currentRunID)
        session.draft = "keep watching"
        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started", timestamp: "2026-08-03T18:00:02Z"),
            conversationID: "conversation_stale_steer"
        )
        #expect(session.currentRunID == "run_b")

        session.steer(expectedRunID: renderedRunID)
        try await Task.sleep(for: .milliseconds(50))
        #expect(
            !ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/steer" }
        )
        #expect(session.draft == "keep watching")
        #expect(session.currentRunID == "run_b")

        session.steer(expectedRunID: "run_b")
        try await wait {
            ExternalRunControlStub.requests.contains { $0.url?.path == "/v1/runs/run_b/steer" }
        }
        #expect(session.draft.isEmpty)
        session.reset()
    }
}
