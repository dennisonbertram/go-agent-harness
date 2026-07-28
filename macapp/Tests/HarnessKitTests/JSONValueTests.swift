import Foundation
import Testing

@testable import HarnessKit

/// `JSONValue.intValue` used to round doubles, silently turning `2.5` into
/// `3` — a fabricated integer harnessd never sent. `Int(exactly:)` is the fix:
/// only a double with no fractional part converts (#951 finding 12).
@Suite("JSONValue")
struct JSONValueTests {

    @Test("intValue keeps an exact integral double")
    func intValueKeepsIntegralDouble() {
        #expect(JSONValue.double(4.0).intValue == 4)
    }

    @Test("intValue rejects a fractional double instead of rounding it")
    func intValueRejectsFractionalDouble() {
        #expect(JSONValue.double(2.5).intValue == nil)
    }

    /// Regression angle distinct from the two above: a double so large it
    /// cannot fit in `Int` must also fail closed, the same way a fractional
    /// value does — proving the fix is "only exact values convert", not just
    /// "reject halves".
    @Test("intValue rejects a double outside Int's range")
    func intValueRejectsOutOfRangeDouble() {
        #expect(JSONValue.double(1e30).intValue == nil)
    }
}
