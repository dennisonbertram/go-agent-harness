import Foundation
import Testing

@testable import ToolWalk

/// Two angles the behavioral tests in ToolSpecTests/VerdictTests don't cover:
/// the on-disk JSON contract the task promises, and the ordering guarantee
/// Runner.walk silently depends on.
@Suite("ToolWalk regressions")
struct RegressionTests {

    /// `/tmp/toolwalk-results.json` is the deliverable: "per tool: name,
    /// verdict, reply". If `ToolResult`'s coding keys are ever renamed or its
    /// Codable conformance loses `verdict`/`reply` (e.g. a refactor adds a
    /// `detail` field instead of reusing `reply`), the results file silently
    /// stops matching what was promised and any downstream reader breaks.
    @Test("ToolResult encodes to JSON with exactly name, verdict, reply")
    func toolResultJSONContract() throws {
        let result = ToolResult(name: "bash", verdict: "pass", reply: "UIWALK_BASH_OK")
        let data = try JSONEncoder().encode(result)
        let object = try #require(
            try JSONSerialization.jsonObject(with: data) as? [String: Any])

        #expect(Set(object.keys) == ["name", "verdict", "reply"])
        #expect(object["name"] as? String == "bash")
        #expect(object["verdict"] as? String == "pass")
        #expect(object["reply"] as? String == "UIWALK_BASH_OK")
    }

    /// Several tools in ui-walk-tools.txt depend on an earlier tool's
    /// server-side state: cron_create (line 13) must run before cron_get/
    /// cron_list/cron_pause/cron_resume/cron_delete reference its "uiwalk"
    /// job, and create_skill/create_workflow must precede verify_skill/
    /// run_workflow. Runner.walk executes specs in array order with no
    /// reordering step, so parsing must preserve file order exactly — a
    /// parser that sorted or grouped entries (e.g. by name) would silently
    /// break every one of those dependent tools.
    @Test("parseToolSpecs preserves the file's line order")
    func preservesFileOrder() throws {
        let specs = try parseToolSpecs(
            """
            cron_create|create the uiwalk job
            cron_get|get the uiwalk job
            cron_delete|delete the uiwalk job
            """)
        #expect(specs.map(\.name) == ["cron_create", "cron_get", "cron_delete"])
    }
}
