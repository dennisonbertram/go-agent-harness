import Foundation
import GoCodeUI
import HarnessKit

struct RunnerConfig: Sendable {
    var timeoutPerTool: Duration
    var pollInterval: Duration

    static let `default` = RunnerConfig(
        timeoutPerTool: .seconds(240), pollInterval: .milliseconds(200))
}

/// Drives every `ToolSpec` through the app's own `ProjectSession`/`RunSession`
/// — the same `submit()` the composer's Send button calls — one fresh
/// conversation per tool, and judges each on its transcript.
@MainActor
enum Runner {
    static func walk(
        project: ProjectSession, specs: [ToolSpec], config: RunnerConfig = .default
    ) async -> [ToolResult] {
        var results: [ToolResult] = []
        for (index, spec) in specs.enumerated() {
            print("[toolwalk] [\(index + 1)/\(specs.count)] \(spec.name) ...", terminator: "")

            // A fresh conversation per tool: TierDeferred tools are activated
            // by find_tool per-run, and every prompt in ui-walk-tools.txt
            // already assumes it starts from nothing activated. It also keeps
            // one tool's failure (or a huge nested agent transcript) from
            // polluting the context every later tool runs in.
            project.newConversation()
            guard let run = project.run else {
                let result = ToolResult(
                    name: spec.name, verdict: "fail", reply: "no active RunSession on project")
                results.append(result)
                print(" FAIL (no run session)")
                continue
            }

            run.draft = spec.prompt
            guard let submission = project.submit() else {
                let result = ToolResult(
                    name: spec.name, verdict: "fail", reply: "submission was not accepted")
                results.append(result)
                print(" FAIL (submission was not accepted)")
                continue
            }

            // Capture the run this walk submission owns before its generic
            // polling loop can observe a later scheduled continuation. The
            // timeout is for this tool's A, never for whichever run is current
            // at the deadline.
            let started = await waitForStartedSubmission(submission, run: run, config: config)
            guard case .started = started else {
                if case .terminal = started {
                    let result = judge(
                        tool: spec.name, observed: observe(submission, timedOut: false))
                    results.append(result)
                    print(" \(result.verdict.uppercased())")
                    continue
                }
                let result = failedResult(tool: spec.name, state: started)
                results.append(result)
                print(" FAIL (\(result.reply))")
                continue
            }
            let finished = await waitForTerminal(run: run, submission: submission, config: config)
            if !finished {
                if submission.isDisplaced {
                    let result = failedResult(tool: spec.name, state: .displaced)
                    results.append(result)
                    print(" FAIL (\(result.reply))")
                    continue
                }
                run.cancelTimedOutRun(expectedRunID: submission.runID)
                // Give the cooperative cancel a moment to land before moving
                // on, or the next tool's newConversation() races its teardown.
                try? await Task.sleep(for: .seconds(1))
            }

            let result = judge(
                tool: spec.name, observed: observe(submission, timedOut: !finished))
            results.append(result)
            print(" \(result.verdict.uppercased())")
        }
        return results
    }

    private static func waitForStartedSubmission(
        _ submission: RunSubmission, run: RunSession, config: RunnerConfig
    ) async -> RunSubmission.State {
        let deadline = ContinuousClock.now.advanced(by: config.timeoutPerTool)
        while ContinuousClock.now < deadline {
            switch submission.state {
            case .started(let runID):
                return run.currentRunID == runID ? submission.state : .displaced
            case .failed, .terminal, .displaced:
                return submission.state
            case .starting:
                break
            }
            try? await Task.sleep(for: config.pollInterval)
        }
        return .failed("timed out waiting for startRun response")
    }

    /// Polls until the run reaches a terminal state, answering any pending
    /// question or approval exactly as the composer's own controls would.
    /// Without this, AskUserQuestion (and any tool a permission rule gates)
    /// would simply hang every walk until the timeout.
    private static func waitForTerminal(
        run: RunSession, submission: RunSubmission, config: RunnerConfig
    ) async -> Bool {
        let deadline = ContinuousClock.now.advanced(by: config.timeoutPerTool)
        while ContinuousClock.now < deadline {
            if submission.isDisplaced || submission.failure != nil { return false }
            guard let runID = submission.runID, run.currentRunID == runID else { return false }
            if let prompt = run.pendingQuestions {
                guard prompt.runID == runID else { return false }
                var answers: [String: String] = [:]
                for question in prompt.questions {
                    answers[question.id] = question.options?.first?.label ?? "yes"
                }
                run.answer(answers, expectedRunID: prompt.runID)
            }
            if let approval = run.transcript.pendingApproval {
                guard approval.runID == runID else { return false }
                run.approve(expectedRunID: approval.runID)
            }
            if let plan = run.transcript.pendingPlan {
                guard plan.runID == runID else { return false }
                run.approve(expectedRunID: plan.runID, option: plan.options.first?.id)
            }
            if submission.isTerminal { return true }
            try? await Task.sleep(for: config.pollInterval)
        }
        return false
    }

    /// Reduces a real transcript to the primitives `judge` reasons over.
    static func observe(_ submission: RunSubmission, timedOut: Bool) -> ObservedRun {
        var completed: [String] = []
        var blocked: [String] = []
        var failed: [String] = []
        var replies: [String] = []

        for item in submission.transcript.items {
            switch item.kind {
            case .toolActivity(let activity):
                switch activity.status {
                case .completed: completed.append(activity.tool)
                case .blocked: blocked.append(activity.tool)
                case .failed: failed.append(activity.tool)
                case .running: break
                }
            case .assistantMessage(let message):
                if !message.text.isEmpty { replies.append(message.text) }
            default:
                break
            }
        }

        return ObservedRun(
            toolCompleted: completed, toolBlocked: blocked, toolFailed: failed,
            finalReply: replies.last ?? "",
            runFailed: submission.transcript.runState == .failed,
            runCancelled: submission.transcript.runState == .cancelled,
            connectionError: submission.failure,
            timedOut: timedOut)
    }

    private static func failedResult(tool: String, state: RunSubmission.State) -> ToolResult {
        let reply: String
        switch state {
        case .displaced:
            reply = "submission was displaced by another run; no action was sent to that run"
        case .failed(let message):
            reply = "submission failed before it started: \(message)"
        case .terminal:
            reply = "submission reached terminal state before ToolWalk could observe it"
        case .starting, .started:
            reply = "submission did not reach a controllable started state"
        }
        return ToolResult(name: tool, verdict: "fail", reply: reply)
    }
}
