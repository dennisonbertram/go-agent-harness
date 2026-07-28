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
            project.submit()

            let finished = await waitForTerminal(run: run, config: config)
            if !finished {
                run.cancel()
                // Give the cooperative cancel a moment to land before moving
                // on, or the next tool's newConversation() races its teardown.
                try? await Task.sleep(for: .seconds(1))
            }

            let result = judge(tool: spec.name, observed: observe(run, timedOut: !finished))
            results.append(result)
            print(" \(result.verdict.uppercased())")
        }
        return results
    }

    /// Polls until the run reaches a terminal state, answering any pending
    /// question or approval exactly as the composer's own controls would.
    /// Without this, AskUserQuestion (and any tool a permission rule gates)
    /// would simply hang every walk until the timeout.
    private static func waitForTerminal(run: RunSession, config: RunnerConfig) async -> Bool {
        let deadline = ContinuousClock.now.advanced(by: config.timeoutPerTool)
        while ContinuousClock.now < deadline {
            if let prompt = run.pendingQuestions {
                var answers: [String: String] = [:]
                for question in prompt.questions {
                    answers[question.id] = question.options?.first?.label ?? "yes"
                }
                run.answer(answers)
            }
            if run.transcript.pendingApproval != nil {
                run.approve()
            }
            if let plan = run.transcript.pendingPlan {
                run.approve(option: plan.options.first?.id)
            }
            if !run.isBusy { return true }
            try? await Task.sleep(for: config.pollInterval)
        }
        return false
    }

    /// Reduces a real transcript to the primitives `judge` reasons over.
    static func observe(_ run: RunSession, timedOut: Bool) -> ObservedRun {
        var completed: [String] = []
        var blocked: [String] = []
        var failed: [String] = []
        var replies: [String] = []

        for item in run.transcript.items {
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
            runFailed: run.transcript.runState == .failed,
            runCancelled: run.transcript.runState == .cancelled,
            connectionError: run.connectionError,
            timedOut: timedOut)
    }
}
