import Foundation
import Testing

@testable import HarnessKit

/// Replays the captured real-run stream through the reducer.
private func replayGolden(into transcript: inout Transcript) throws {
    let url = try #require(
        Bundle.module.url(
            forResource: "run-toolcall-golden", withExtension: "sse", subdirectory: "Fixtures"))
    var parser = SSEParser()
    for frame in parser.consume(try Data(contentsOf: url)) {
        if let event = try? HarnessEvent(frame: frame) {
            transcript.apply(event)
        }
    }
}

@Suite("Transcript reducer")
struct TranscriptTests {

    @Test("builds a transcript from a real run stream")
    func buildsFromGoldenStream() throws {
        var transcript = Transcript()
        transcript.appendUserPrompt("list the workspace")
        try replayGolden(into: &transcript)

        #expect(transcript.runState == .completed)

        // user prompt, the provider-fallback warning this stream carries, one
        // tool activity, one assistant message
        #expect(transcript.items.count == 4)
        guard case .userPrompt(let prompt) = transcript.items[0].kind else {
            Issue.record("expected a user prompt first")
            return
        }
        #expect(prompt == "list the workspace")

        // The stream reports that the run was routed to a provider other than
        // the model implies. Dropping it made the substitution invisible.
        guard case .notice(let notice) = transcript.items[1].kind else {
            Issue.record("expected the provider-fallback warning to be shown")
            return
        }
        #expect(notice.contains("falling back to default provider"))

        guard case .toolActivity(let activity) = transcript.items[2].kind else {
            Issue.record("expected a tool activity row")
            return
        }
        #expect(activity.tool == "ls")
        #expect(activity.callID == "c1")
        #expect(activity.status == .completed)

        guard case .assistantMessage(let message) = transcript.items[3].kind else {
            Issue.record("expected an assistant message")
            return
        }
        #expect(message.text == "I listed the workspace and it contains the project files.")
        #expect(message.isStreaming == false)
    }

    @Test("accumulates streamed assistant deltas into one message")
    func accumulatesDeltas() {
        var transcript = Transcript()
        for chunk in ["Hel", "lo ", "world"] {
            transcript.apply(event(.assistantMessageDelta, ["content": chunk]))
        }

        #expect(transcript.items.count == 1)
        guard case .assistantMessage(let message) = transcript.items[0].kind else {
            Issue.record("expected a streaming assistant message")
            return
        }
        #expect(message.text == "Hello world")
        #expect(message.isStreaming)
    }

    /// Providers that stream deltas also send a final `assistant.message` with
    /// the complete text. Appending it would duplicate the whole reply.
    @Test("does not duplicate text when a final message follows its deltas")
    func finalMessageDoesNotDuplicateDeltas() {
        var transcript = Transcript()
        transcript.apply(event(.assistantMessageDelta, ["content": "Hello "]))
        transcript.apply(event(.assistantMessageDelta, ["content": "world"]))
        transcript.apply(event(.assistantMessage, ["content": "Hello world"]))

        #expect(transcript.items.count == 1)
        guard case .assistantMessage(let message) = transcript.items[0].kind else {
            Issue.record("expected one assistant message")
            return
        }
        #expect(message.text == "Hello world")
        #expect(message.isStreaming == false)
    }

    @Test("tracks concurrent tool calls independently by call id")
    func tracksConcurrentToolCalls() {
        var transcript = Transcript()
        transcript.apply(
            event(.toolCallStarted, ["call_id": "a", "tool": "read"]))
        transcript.apply(
            event(.toolCallStarted, ["call_id": "b", "tool": "bash"]))
        // Completed out of order, as concurrent tools do.
        transcript.apply(event(.toolCallCompleted, ["call_id": "b"]))
        transcript.apply(event(.toolCallCompleted, ["call_id": "a"]))

        let activities = transcript.items.compactMap { item -> ToolActivity? in
            if case .toolActivity(let activity) = item.kind { return activity }
            return nil
        }
        #expect(activities.count == 2)
        #expect(activities.allSatisfy { $0.status == .completed })
        #expect(activities.map(\.tool) == ["read", "bash"])
    }

    @Test("streams tool output into the matching row")
    func streamsToolOutput() {
        var transcript = Transcript()
        transcript.apply(
            event(.toolCallStarted, ["call_id": "a", "tool": "bash"]))
        transcript.apply(
            event(.toolOutputDelta, ["call_id": "a", "content": "line1\n"]))
        transcript.apply(
            event(.toolOutputDelta, ["call_id": "a", "content": "line2"]))

        guard case .toolActivity(let activity) = transcript.items[0].kind else {
            Issue.record("expected a tool activity row")
            return
        }
        #expect(activity.output == "line1\nline2")
        #expect(activity.status == .running)
    }

    @Test("marks blocked tool calls distinctly from failures")
    func marksBlockedCalls() {
        var transcript = Transcript()
        transcript.apply(
            event(.toolCallStarted, ["call_id": "a", "tool": "bash"]))
        transcript.apply(event(.toolCallBlocked, ["call_id": "a"]))

        guard case .toolActivity(let activity) = transcript.items[0].kind else {
            Issue.record("expected a tool activity row")
            return
        }
        #expect(activity.status == .blocked)
    }

    /// A spinner that only clears on `run.completed` hangs forever on failure
    /// or cancellation — the most common streaming-UI bug.
    @Test(
        "leaves no active run state after any terminal event",
        arguments: [
            (HarnessEventType.runCompleted, RunState.completed),
            (.runFailed, .failed),
            (.runCancelled, .cancelled),
        ])
    func clearsOnEveryTerminalEvent(type: HarnessEventType, expected: RunState) {
        var transcript = Transcript()
        transcript.apply(event(.runStarted, [:]))
        #expect(transcript.runState == .running)

        transcript.apply(event(type, [:]))
        #expect(transcript.runState == expected)
        #expect(transcript.runState.isActive == false)
    }

    @Test("surfaces a failure message in the transcript")
    func surfacesFailure() {
        var transcript = Transcript()
        transcript.apply(event(.runStarted, [:]))
        transcript.apply(event(.runFailed, ["error": "provider exploded"]))

        let errors = transcript.items.compactMap { item -> String? in
            if case .error(let message) = item.kind { return message }
            return nil
        }
        #expect(errors == ["provider exploded"])
    }

    @Test("records a pending approval and clears it once decided")
    func tracksApproval() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .toolApprovalRequired,
                ["call_id": "a", "tool": "bash", "arguments": "{}"]))
        #expect(transcript.pendingApproval?.tool == "bash")
        #expect(transcript.runState == .waitingForUser)

        transcript.apply(event(.toolApprovalGranted, ["call_id": "a"]))
        #expect(transcript.pendingApproval == nil)
    }

    /// Leaving plan mode is an approval with named approaches; approving sends
    /// the chosen option id back.
    @Test("captures a pending plan and its approaches")
    func capturesPendingPlan() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .planApprovalRequired,
                [
                    "tool": "plan_exit",
                    "plan": "# Plan\n\nDo the thing.",
                    "options": [
                        ["id": "a", "label": "Incremental", "description": "Smaller steps"],
                        ["id": "b", "label": "All at once"],
                    ],
                ]))

        let plan = transcript.pendingPlan
        #expect(plan?.plan.contains("Do the thing") == true)
        #expect(plan?.options.map(\.id) == ["a", "b"])
        #expect(plan?.options.first?.description == "Smaller steps")
        #expect(transcript.runState == .waitingForUser)

        transcript.apply(event(.planApprovalGranted, [:]))
        #expect(transcript.pendingPlan == nil)
    }

    @Test("handles a plan with no parsed approaches")
    func planWithoutOptions() {
        var transcript = Transcript()
        transcript.apply(event(.planApprovalRequired, ["plan": "just do it"]))
        #expect(transcript.pendingPlan?.options.isEmpty == true)
    }

    /// Field names here are taken from a real `usage.delta` payload, not
    /// guessed: totals are nested under `cumulative_usage`, and the cost is a
    /// sibling flat field.
    @Test("accumulates cost and token usage")
    func accumulatesUsage() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .usageDelta,
                [
                    "cumulative_usage": [
                        "prompt_tokens": 120, "completion_tokens": 30, "total_tokens": 150,
                    ],
                    "cumulative_cost_usd": 0.0025,
                    "cost_status": "available",
                ]))
        #expect(transcript.usage.totalTokens == 150)
        #expect(transcript.usage.promptTokens == 120)
        #expect(transcript.usage.completionTokens == 30)
        #expect(transcript.usage.costUSD == 0.0025)
        #expect(transcript.usage.costIsKnown)
    }

    /// The golden run's first turn is unpriced and its second is priced, so
    /// replaying it pins the real end state: totals accumulate and cost becomes
    /// known only once the server says so.
    @Test("tracks usage across a real run's priced and unpriced turns")
    func tracksUsageAcrossGoldenRun() throws {
        var transcript = Transcript()
        try replayGolden(into: &transcript)

        #expect(transcript.usage.totalTokens == 282)
        #expect(transcript.usage.promptTokens == 260)
        #expect(transcript.usage.completionTokens == 22)
        #expect(transcript.usage.costUSD == 0.0025)
        #expect(transcript.usage.costIsKnown)
    }

    /// A stream may reach the terminal snapshot without the earlier
    /// `usage.delta` frame being available to this client. The terminal event
    /// is still a complete, authoritative accounting snapshot from harnessd,
    /// so rendering completion with zero usage is incorrect.
    @Test("reconciles terminal accounting before completed state is observable")
    func terminalEventReconcilesUsage() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .runCompleted,
                [
                    "usage_totals": [
                        "prompt_tokens_total": 260,
                        "completion_tokens_total": 22,
                        "total_tokens": 282,
                    ],
                    "cost_totals": [
                        "cost_usd_total": 0.0025,
                        "cost_status": "available",
                    ],
                ]))

        #expect(transcript.runState == .completed)
        #expect(transcript.usage.promptTokens == 260)
        #expect(transcript.usage.completionTokens == 22)
        #expect(transcript.usage.totalTokens == 282)
        #expect(transcript.usage.costUSD == 0.0025)
        #expect(transcript.usage.costIsKnown)
    }

    @Test("incomplete terminal accounting preserves earlier usage")
    func incompleteTerminalAccountingDoesNotEraseUsage() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .usageDelta,
                [
                    "cumulative_usage": ["total_tokens": 150],
                    "cumulative_cost_usd": 0.001,
                    "cost_status": "available",
                ]))
        transcript.apply(event(.runCompleted, ["usage_totals": [:], "cost_totals": [:]]))

        #expect(transcript.runState == .completed)
        #expect(transcript.usage.totalTokens == 150)
        #expect(transcript.usage.costUSD == 0.001)
        #expect(transcript.usage.costIsKnown)
    }

    /// Conversation-stream terminal handling immediately reconciles durable
    /// messages. That rebuild must preserve the accounting the just-applied
    /// event made visible; otherwise a real run flashes completed then loses
    /// its usage before the UI can render it.
    @Test("durable message reconciliation retains terminal accounting")
    func reconcileRetainsTerminalUsage() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .runCompleted,
                [
                    "usage_totals": ["total_tokens": 282],
                    "cost_totals": ["cost_usd_total": 0.0025, "cost_status": "available"],
                ]))

        transcript.reconcile(
            messages: [
                StoredMessage(
                    role: "user", content: "list the workspace", step: nil, toolCalls: nil),
                StoredMessage(
                    role: "assistant", content: "I listed it.", step: nil, toolCalls: nil),
            ])

        #expect(transcript.runState == .completed)
        #expect(transcript.usage.totalTokens == 282)
        #expect(transcript.usage.costUSD == 0.0025)
        #expect(transcript.usage.costIsKnown)
    }

    /// An unpriced model reports zero cost with `cost_status: "unpriced_model"`.
    /// Rendering that as "$0.00" reads as free, which is wrong.
    @Test("does not present unpriced cost as zero")
    func unpricedCostIsNotKnown() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .usageDelta,
                [
                    "cumulative_usage": ["total_tokens": 130],
                    "cumulative_cost_usd": 0,
                    "cost_status": "unpriced_model",
                ]))
        #expect(transcript.usage.totalTokens == 130)
        #expect(transcript.usage.costIsKnown == false)
    }
}

/// Builds a synthetic event for reducer tests from a plain JSON payload.
private func event(_ type: HarnessEventType, _ payload: [String: Any]) -> HarnessEvent {
    let envelope: [String: Any] = [
        "id": "run_t:0", "run_id": "run_t", "type": type.rawValue, "payload": payload,
    ]
    let data = try! JSONSerialization.data(withJSONObject: envelope)
    return try! HarnessEvent(
        frame: SSEFrame(
            id: "run_t:0", event: type.rawValue, data: String(decoding: data, as: UTF8.self)))
}

extension TranscriptTests {
    /// A background job finishes after its run has ended, so this event is the
    /// only signal the transcript gets that it ran at all. Before it existed, a
    /// job that ran and printed correctly was reported to the user as never
    /// having fired.
    @Test("a finished background job appears in the transcript with its output")
    func backgroundJobCompletionSurfaces() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .backgroundJobCompleted,
                [
                    "command": "sleep 15; echo hello dennison",
                    "output": "hello dennison",
                    "exit_code": 0,
                ]))

        let notices = transcript.items.compactMap { item -> String? in
            if case .notice(let text) = item.kind { return text }
            return nil
        }
        #expect(notices.count == 1)
        #expect(notices.first?.contains("hello dennison") == true)
        #expect(notices.first?.contains("sleep 15") == true)
    }

    @Test("a background job that timed out says so")
    func backgroundJobTimeoutSurfaces() {
        var transcript = Transcript()
        transcript.apply(
            event(
                .backgroundJobCompleted,
                ["command": "sleep 999", "timed_out": true, "exit_code": -1]))
        let notices = transcript.items.compactMap { item -> String? in
            if case .notice(let text) = item.kind { return text }
            return nil
        }
        #expect(notices.first?.contains("timed out") == true)
    }
}
