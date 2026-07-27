import Foundation

/// One `text/event-stream` frame.
public struct SSEFrame: Sendable, Hashable {
    /// The `id:` field. Sent back as `Last-Event-ID` to resume a dropped stream.
    public let id: String
    /// The `event:` field, e.g. `run.started`.
    public let event: String
    /// The `data:` field, with multi-line values joined by newlines.
    public let data: String

    public init(id: String = "", event: String = "", data: String = "") {
        self.id = id
        self.event = event
        self.data = data
    }
}

/// Incremental `text/event-stream` parser.
///
/// Network reads split the stream at arbitrary byte offsets, so the parser only
/// ever acts on lines it has seen terminated, and only emits a frame once its
/// blank separator line arrives. Feeding the same bytes in any chunking
/// produces the same frames.
public struct SSEParser: Sendable {
    private var pending = Data()
    private var id = ""
    private var event = ""
    private var dataLines: [String] = []
    private var hasFields = false

    public init() {}

    /// Appends `chunk` and returns every frame completed by it.
    public mutating func consume(_ chunk: Data) -> [SSEFrame] {
        pending.append(chunk)

        var frames: [SSEFrame] = []
        // Only consume up to the last newline; any trailing partial line stays
        // buffered for the next chunk.
        while let newline = pending.firstIndex(of: UInt8(ascii: "\n")) {
            let lineBytes = pending[pending.startIndex..<newline]
            pending = pending[pending.index(after: newline)...]
            let line = stripCR(String(decoding: lineBytes, as: UTF8.self))
            if let frame = handle(line: line) {
                frames.append(frame)
            }
        }
        return frames
    }

    private func stripCR(_ line: String) -> String {
        line.hasSuffix("\r") ? String(line.dropLast()) : line
    }

    /// Returns a frame when `line` is the blank separator closing a frame.
    private mutating func handle(line: String) -> SSEFrame? {
        guard !line.isEmpty else {
            // A blank line with nothing accumulated is just padding between
            // frames, not an empty event.
            guard hasFields else { return nil }
            let frame = SSEFrame(id: id, event: event, data: dataLines.joined(separator: "\n"))
            id = ""
            event = ""
            dataLines = []
            hasFields = false
            return frame
        }

        // Lines beginning with a colon are comments (used for keepalives).
        guard !line.hasPrefix(":") else { return nil }

        let field: String
        var value: String
        if let colon = line.firstIndex(of: ":") {
            field = String(line[line.startIndex..<colon])
            value = String(line[line.index(after: colon)...])
            // A single leading space after the colon is part of the framing.
            if value.hasPrefix(" ") { value.removeFirst() }
        } else {
            field = line
            value = ""
        }

        switch field {
        case "id":
            id = value
            hasFields = true
        case "event":
            event = value
            hasFields = true
        case "data":
            dataLines.append(value)
            hasFields = true
        case "retry":
            // Reconnection hint; the client owns its own backoff policy.
            break
        default:
            break
        }
        return nil
    }
}
