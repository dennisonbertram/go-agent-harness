import Foundation

/// Up/Down prompt-history recall for the composer, as a plain value type
/// (pattern: `MarkdownBlock`) so navigation logic is unit-testable without a
/// view. `cursor == nil` means "not currently navigating" -- the normal state
/// while the operator is typing a fresh draft.
///
/// "Cursor-aware" per #998 would mean only recalling when the text caret sits
/// on the composer's first/last line, which needs a SwiftUI caret binding
/// (`TextSelection`, macOS 15) unavailable on this package's `.macOS(.v14)`
/// floor (see plan KTD-7). This approximates the same intent -- never
/// destroy an in-progress draft -- via draft state instead: `recallPrevious`
/// only starts a new recall from an empty draft, and any non-recall edit
/// resets navigation so the next Up starts fresh.
public struct PromptHistory: Sendable, Equatable {
    private var entries: [String] = []
    private var cursor: Int?
    private var stashedDraft: String?

    public init() {}

    /// Appends a submitted prompt and ends any in-progress navigation, so the
    /// next Up starts from this newest entry.
    public mutating func record(_ prompt: String) {
        entries.append(prompt)
        cursor = nil
        stashedDraft = nil
    }

    /// Walks one entry further into the past, or declines. Declines (returns
    /// `nil`, leaving the cursor unmoved) when navigation has not yet started
    /// and `currentDraft` holds an in-progress thought -- the guarantee that
    /// prompted #998: an Up press must never overwrite unsaved typing.
    /// Already-navigating calls ignore `currentDraft` and never wrap around
    /// past the oldest entry.
    public mutating func recallPrevious(currentDraft: String) -> String? {
        if let currentCursor = cursor {
            // Already navigating: KTD-7's approximation of caret-awareness.
            // Continue only while the draft is empty or still matches the
            // entry last recalled -- if it diverged, an edit is in progress
            // and Up must decline rather than silently replace it.
            guard currentDraft.isEmpty || currentDraft == entries[currentCursor] else {
                return nil
            }
            guard currentCursor > 0 else { return entries[currentCursor] }
            cursor = currentCursor - 1
            return entries[currentCursor - 1]
        }
        guard currentDraft.trimmed.isEmpty, !entries.isEmpty else { return nil }
        stashedDraft = currentDraft
        cursor = entries.count - 1
        return entries[entries.count - 1]
    }

    /// Walks one entry back toward the present. Past the newest entry,
    /// restores the exact draft that was in progress when navigation
    /// started (which may itself have been empty) and ends navigation. A
    /// call while not navigating declines.
    public mutating func recallNext() -> String? {
        guard let currentCursor = cursor else { return nil }
        let next = currentCursor + 1
        guard next < entries.count else {
            let stash = stashedDraft ?? ""
            cursor = nil
            stashedDraft = nil
            return stash
        }
        cursor = next
        return entries[next]
    }

    /// Ends navigation without discarding recorded entries -- called on any
    /// draft edit that is not itself a recall, so typing after a recall
    /// starts a fresh navigation next time.
    public mutating func reset() {
        cursor = nil
        stashedDraft = nil
    }
}
