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
    /// A wait result is deliberately richer than a boolean. A terminal A
    /// which raced B selection is not a timeout, and neither displacement nor
    /// failure authorizes a control request against whatever run is selected.
    enum SubmissionWaitOutcome: Equatable {
        case started
        case terminal
        case failed(String)
        case displaced
        case timedOut
    }

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
            let started = await waitForStartedSubmission(submission, config: config)
            guard started == .started else {
                if started == .terminal {
                    let result = judge(
                        tool: spec.name, observed: observe(submission, timedOut: false))
                    results.append(result)
                    print(" \(result.verdict.uppercased())")
                    continue
                }
                let result = failedResult(tool: spec.name, outcome: started)
                results.append(result)
                print(" FAIL (\(result.reply))")
                continue
            }
            let finished = await waitForTerminal(run: run, submission: submission, config: config)
            switch finished {
            case .terminal:
                break
            case .timedOut:
                if shouldCancel(for: finished) {
                    run.cancelTimedOutRun(expectedRunID: submission.runID)
                }
                // Give the cooperative cancel a moment to land before moving
                // on, or the next tool's newConversation() races its teardown.
                try? await Task.sleep(for: .seconds(1))
            case .failed, .displaced:
                let result = failedResult(tool: spec.name, outcome: finished)
                results.append(result)
                print(" FAIL (\(result.reply))")
                continue
            case .started:
                // `waitForTerminal` cannot return started; retain an explicit
                // failure if a future implementation violates that contract.
                let result = failedResult(tool: spec.name, outcome: .failed("invalid wait outcome"))
                results.append(result)
                print(" FAIL (\(result.reply))")
                continue
            }

            let result = judge(
                tool: spec.name, observed: observe(submission, timedOut: finished == .timedOut))
            results.append(result)
            print(" \(result.verdict.uppercased())")
        }
        return results
    }

    static func waitForStartedSubmission(
        _ submission: RunSubmission, config: RunnerConfig
    ) async -> SubmissionWaitOutcome {
        let deadline = ContinuousClock.now.advanced(by: config.timeoutPerTool)
        while ContinuousClock.now < deadline {
            let outcome = outcome(for: submission)
            if outcome != .started || submission.runID != nil { return outcome }
            try? await Task.sleep(for: config.pollInterval)
        }
        return .timedOut
    }

    /// Polls until the run reaches a terminal state, answering any pending
    /// question or approval exactly as the composer's own controls would.
    /// Without this, AskUserQuestion (and any tool a permission rule gates)
    /// would simply hang every walk until the timeout.
    static func waitForTerminal(
        run: RunSession, submission: RunSubmission, config: RunnerConfig
    ) async -> SubmissionWaitOutcome {
        let deadline = ContinuousClock.now.advanced(by: config.timeoutPerTool)
        while ContinuousClock.now < deadline {
            let outcome = outcome(for: submission)
            switch outcome {
            case .terminal, .failed, .displaced: return outcome
            case .started: break
            case .timedOut: return .timedOut
            }
            guard let runID = submission.runID, run.currentRunID == runID else { return .displaced }
            if let prompt = run.pendingQuestions {
                guard prompt.runID == runID else { return .displaced }
                var answers: [String: String] = [:]
                for question in prompt.questions {
                    answers[question.id] = question.options?.first?.label ?? "yes"
                }
                run.answer(answers, expectedRunID: prompt.runID)
            }
            if let approval = run.transcript.pendingApproval {
                guard approval.runID == runID else { return .displaced }
                run.approve(expectedRunID: approval.runID)
            }
            if let plan = run.transcript.pendingPlan {
                guard plan.runID == runID else { return .displaced }
                run.approve(expectedRunID: plan.runID, option: plan.options.first?.id)
            }
            try? await Task.sleep(for: config.pollInterval)
        }
        return .timedOut
    }

    /// Lifecycle has priority over displacement. A selected B must prevent
    /// further A controls, but cannot make a completed/failed A look timed
    /// out to the tool-verdict layer.
    static func outcome(for submission: RunSubmission) -> SubmissionWaitOutcome {
        outcome(for: submission.lifecycle, isDisplaced: submission.isDisplaced)
    }

    static func outcome(
        for lifecycle: RunSubmission.Lifecycle, isDisplaced: Bool
    ) -> SubmissionWaitOutcome {
        switch lifecycle {
        case .terminal: return .terminal
        case .failed(let message): return .failed(message)
        case .starting, .started:
            return isDisplaced ? .displaced : .started
        }
    }

    static func shouldCancel(for outcome: SubmissionWaitOutcome) -> Bool {
        outcome == .timedOut
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

    private static func failedResult(tool: String, outcome: SubmissionWaitOutcome) -> ToolResult {
        let reply: String
        switch outcome {
        case .displaced:
            reply = "submission was displaced by another run; no action was sent to that run"
        case .failed(let message):
            reply = "submission failed: \(message)"
        case .terminal:
            reply = "submission reached terminal state before ToolWalk could observe it"
        case .timedOut:
            reply = "submission timed out waiting for a terminal outcome"
        case .started:
            reply = "submission did not reach a controllable started state"
        }
        return ToolResult(name: tool, verdict: "fail", reply: reply)
    }
}
