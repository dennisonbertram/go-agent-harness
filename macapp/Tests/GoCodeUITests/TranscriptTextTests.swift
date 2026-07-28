import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

@Suite("TranscriptText")
struct TranscriptTextTests {

    @Test("completed activity label rounds milliseconds into Codex-style seconds")
    func completedActivityLabel() {
        #expect(TranscriptActivityLabel.completed(durationMS: 10_501) == "Worked for 11s")
        #expect(TranscriptActivityLabel.completed(durationMS: nil) == "Worked")
    }

    /// Builds transcript items the way the app does — by replaying events —
    /// rather than constructing them directly, so the test cannot drift from
    /// the shapes harnessd actually produces.
    private func transcript(_ events: [(String, String)]) throws -> Transcript {
        var transcript = Transcript()
        for (type, payload) in events {
            let data = #"{"id":"r:1","run_id":"r","type":"\#(type)","payload":\#(payload)}"#
            transcript.apply(try HarnessEvent(frame: SSEFrame(event: type, data: data)))
        }
        return transcript
    }

    @Test("every transcript kind reaches the clipboard text")
    func coversAllKinds() throws {
        var built = try transcript([
            ("assistant.thinking.delta", #"{"content":"weighing options"}"#),
            (
                "tool.call.started",
                #"{"call_id":"c1","tool":"read","arguments":"{\"path\":\"main.go\"}"}"#
            ),
            ("tool.call.completed", #"{"call_id":"c1","duration_ms":12}"#),
            ("assistant.message", #"{"content":"Done."}"#),
            ("auto_compact.completed", #"{"summary":"earlier work","messages_removed":4}"#),
            ("run.failed", #"{"error":"provider failed"}"#),
        ])
        built.appendUserPrompt("build it")

        let text = TranscriptText.plain(built.items)

        #expect(text.contains("You: build it"))
        #expect(text.contains("Thinking: weighing options"))
        // The tool line carries the readable argument, not raw JSON.
        #expect(text.contains("[read] main.go"))
        #expect(text.contains("Done."))
        #expect(text.contains("Error: provider failed"))
        #expect(text.contains("4 messages folded"))
    }

    @Test("messages stay separated so a paste is readable")
    func separatesMessages() throws {
        var built = Transcript()
        built.appendUserPrompt("one")
        built.apply(
            try HarnessEvent(
                frame: SSEFrame(
                    event: "assistant.message",
                    data:
                        #"{"id":"r:1","run_id":"r","type":"assistant.message","payload":{"content":"two"}}"#
                )))

        #expect(TranscriptText.plain(built.items) == "You: one\n\ntwo")
    }

    @Test("an empty transcript copies as nothing rather than whitespace")
    func emptyTranscript() {
        #expect(TranscriptText.plain([]).isEmpty)
    }
}

@Suite("Prompt warnings")
struct PromptWarningTests {

    /// harnessd reports a silent provider substitution as prompt.warning. The
    /// app used to drop the event, so the swap only showed up later as an
    /// error naming a provider the user never chose.
    @Test("a prompt warning becomes a visible notice")
    func warningBecomesNotice() throws {
        var built = Transcript()
        let data =
            #"{"id":"r:1","run_id":"r","type":"prompt.warning","payload":"#
            + #"{"code":"provider_fallback","message":"model \"kimi-k2.5\" provider unavailable, falling back to default provider"}}"#
        built.apply(try HarnessEvent(frame: SSEFrame(event: "prompt.warning", data: data)))

        guard case .notice(let message) = built.items.last?.kind else {
            #expect(
                Bool(false),
                "expected a notice item, got \(String(describing: built.items.last?.kind))")
            return
        }
        #expect(message.contains("kimi-k2.5"))
        #expect(TranscriptText.plain(built.items).hasPrefix("Note: "))
    }

    @Test("an empty warning is not shown as a blank row")
    func emptyWarningIgnored() throws {
        var built = Transcript()
        let data = #"{"id":"r:1","run_id":"r","type":"prompt.warning","payload":{"message":""}}"#
        built.apply(try HarnessEvent(frame: SSEFrame(event: "prompt.warning", data: data)))
        #expect(built.items.isEmpty)
    }
}
