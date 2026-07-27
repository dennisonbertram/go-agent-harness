import Foundation
import Testing

@testable import HarnessKit

/// The golden fixture is a byte-for-byte capture of a real harnessd run stream
/// (`GET /v1/runs/{id}/events`) for a two-step run that calls the `ls` tool and
/// then produces a final assistant message. Recapture it with
/// `scripts/capture-sse-golden.sh` if the wire format changes.
private func goldenStream() throws -> Data {
    let url = try #require(
        Bundle.module.url(
            forResource: "run-toolcall-golden", withExtension: "sse", subdirectory: "Fixtures"),
        "golden SSE fixture missing from test bundle"
    )
    return try Data(contentsOf: url)
}

@Suite("SSEParser")
struct SSEParserTests {

    @Test("parses every frame of a real run stream in order")
    func parsesGoldenStreamInOrder() throws {
        var parser = SSEParser()
        let frames = parser.consume(try goldenStream())

        let names = frames.map(\.event)
        #expect(
            names == [
                "run.started",
                "prompt.warning",
                "provider.resolved",
                "prompt.resolved",
                "run.step.started",
                "llm.turn.requested",
                "usage.delta",
                "llm.turn.completed",
                "tool.call.started",
                "tool.call.completed",
                "memory.observe.started",
                "memory.observe.completed",
                "run.step.completed",
                "run.step.started",
                "llm.turn.requested",
                "usage.delta",
                "llm.turn.completed",
                "assistant.message",
                "memory.observe.started",
                "memory.observe.completed",
                "run.step.completed",
                "run.completed",
            ])
    }

    /// The `id:` field is what we send back as `Last-Event-ID` to resume a
    /// dropped stream without replaying or dropping events, so it must survive
    /// parsing.
    @Test("captures the event id used for stream resumption")
    func capturesEventID() throws {
        var parser = SSEParser()
        let frames = parser.consume(try goldenStream())
        let first = try #require(frames.first)
        #expect(first.id.hasSuffix(":0"))
        #expect(try #require(frames.last).id.hasSuffix(":21"))
    }

    /// A real network stream delivers arbitrary byte boundaries. Splitting the
    /// same input into single bytes must yield an identical frame sequence —
    /// this is the property that broke naive line-splitting implementations.
    @Test("is invariant to chunk boundaries")
    func invariantToChunkBoundaries() throws {
        let data = try goldenStream()

        var whole = SSEParser()
        let expected = whole.consume(data).map(\.event)

        var drip = SSEParser()
        var actual: [String] = []
        for byte in data {
            actual += drip.consume(Data([byte])).map(\.event)
        }

        #expect(actual == expected)
        #expect(actual.count == 22)
    }

    @Test("does not emit a frame until its terminating blank line arrives")
    func withholdsIncompleteFrames() {
        var parser = SSEParser()
        #expect(parser.consume(Data("event: run.started\ndata: {}\n".utf8)).isEmpty)
        #expect(parser.consume(Data("\n".utf8)).count == 1)
    }

    @Test("joins multi-line data fields with newlines per the SSE spec")
    func joinsMultiLineData() {
        var parser = SSEParser()
        let frames = parser.consume(Data("event: x\ndata: line1\ndata: line2\n\n".utf8))
        #expect(frames.count == 1)
        #expect(frames.first?.data == "line1\nline2")
    }

    @Test("accepts CRLF line endings and comment lines")
    func acceptsCRLFAndComments() {
        var parser = SSEParser()
        let frames = parser.consume(Data(": keepalive\r\nevent: ping\r\ndata: {}\r\n\r\n".utf8))
        #expect(frames.count == 1)
        #expect(frames.first?.event == "ping")
    }
}

@Suite("HarnessEvent decoding")
struct HarnessEventDecodingTests {

    @Test("decodes the run envelope and typed payload accessors")
    func decodesEnvelope() throws {
        var parser = SSEParser()
        let frames = parser.consume(try goldenStream())
        let events = frames.compactMap { try? HarnessEvent(frame: $0) }
        #expect(events.count == 22)

        let toolStart = try #require(events.first { $0.type == .toolCallStarted })
        #expect(toolStart.payload["tool"]?.stringValue == "ls")
        #expect(toolStart.payload["call_id"]?.stringValue == "c1")
        #expect(toolStart.payload["step"]?.intValue == 1)
        #expect(toolStart.runID.hasPrefix("run_"))

        let message = try #require(events.first { $0.type == .assistantMessage })
        #expect(
            message.payload["content"]?.stringValue
                == "I listed the workspace and it contains the project files.")
    }

    /// harnessd adds event types faster than a client can adopt them; an
    /// unrecognised type must round-trip as `.other` rather than throwing away
    /// the frame, otherwise a server upgrade silently breaks the transcript.
    @Test("preserves unknown event types instead of discarding them")
    func preservesUnknownTypes() throws {
        let frame = SSEFrame(
            id: "run_x:1", event: "some.future.event",
            data:
                #"{"id":"run_x:1","run_id":"run_x","type":"some.future.event","payload":{"k":"v"}}"#
        )
        let event = try HarnessEvent(frame: frame)
        #expect(event.type == .other("some.future.event"))
        #expect(event.payload["k"]?.stringValue == "v")
    }

    @Test("terminal events are identified for stream shutdown")
    func identifiesTerminalEvents() {
        #expect(HarnessEventType.runCompleted.isTerminal)
        #expect(HarnessEventType.runFailed.isTerminal)
        #expect(HarnessEventType.runCancelled.isTerminal)
        #expect(!HarnessEventType.assistantMessage.isTerminal)
        #expect(!HarnessEventType.other("x").isTerminal)
    }
}
