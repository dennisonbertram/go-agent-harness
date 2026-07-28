import Foundation
import Testing

@testable import GoCodeUI

@Suite("MarkdownBlock parsing")
struct MarkdownBlockTests {

    @Test("headings parse with their level, one per line")
    func headings() {
        let blocks = MarkdownBlock.parse("# Title\n## Subtitle\n###### Tiny")
        #expect(
            blocks == [
                .heading(level: 1, text: "Title"),
                .heading(level: 2, text: "Subtitle"),
                .heading(level: 6, text: "Tiny"),
            ])
    }

    @Test("a line of only hashes with no space is not a heading")
    func notAHeadingWithoutSpace() {
        let blocks = MarkdownBlock.parse("#nospace")
        #expect(blocks == [.paragraph("#nospace")])
    }

    @Test("unordered list markers -, *, + all produce list items")
    func unorderedList() {
        let blocks = MarkdownBlock.parse("- first\n* second\n+ third")
        #expect(
            blocks == [
                .unorderedListItem("first"),
                .unorderedListItem("second"),
                .unorderedListItem("third"),
            ])
    }

    @Test("ordered list keeps the given numbers, not a recount")
    func orderedList() {
        let blocks = MarkdownBlock.parse("1. one\n2. two\n7. seven")
        #expect(
            blocks == [
                .orderedListItem(number: 1, text: "one"),
                .orderedListItem(number: 2, text: "two"),
                .orderedListItem(number: 7, text: "seven"),
            ])
    }

    @Test("block quotes strip the marker and leading space")
    func blockQuote() {
        let blocks = MarkdownBlock.parse("> a wise remark")
        #expect(blocks == [.quote("a wise remark")])
    }

    @Test("thematic breaks in --- *** or ___ form become a rule")
    func horizontalRule() {
        #expect(MarkdownBlock.parse("---") == [.rule])
        #expect(MarkdownBlock.parse("***") == [.rule])
        #expect(MarkdownBlock.parse("___") == [.rule])
    }

    @Test("a heading immediately followed by a list keeps both blocks distinct")
    func headingThenList() {
        let blocks = MarkdownBlock.parse("## Steps\n- one\n- two")
        #expect(
            blocks == [
                .heading(level: 2, text: "Steps"),
                .unorderedListItem("one"),
                .unorderedListItem("two"),
            ])
    }

    @Test("a fenced code block interrupts a list and resumes after it")
    func listInterruptedByFence() {
        let blocks = MarkdownBlock.parse("- before\n```swift\nlet x = 1\n```\n- after")
        #expect(
            blocks == [
                .unorderedListItem("before"),
                .code("let x = 1", "swift"),
                .unorderedListItem("after"),
            ])
    }

    @Test("an unterminated fence mid-stream still renders as code")
    func unterminatedFenceMidStream() {
        let blocks = MarkdownBlock.parse("```swift\nlet x = 1")
        #expect(blocks == [.code("let x = 1", "swift")])
    }

    @Test("a paragraph's internal line break is preserved, not collapsed")
    func paragraphKeepsLineBreak() {
        let blocks = MarkdownBlock.parse("first line\nsecond line")
        // Two trailing spaces before the newline is CommonMark's explicit hard
        // break, so Text(.init:) renders it as a real line break instead of
        // reflowing the two source lines into one.
        #expect(blocks == [.paragraph("first line  \nsecond line")])
    }

    @Test("a blank line ends a paragraph rather than merging into the next one")
    func blankLineEndsParagraph() {
        let blocks = MarkdownBlock.parse("first paragraph\n\nsecond paragraph")
        #expect(
            blocks == [
                .paragraph("first paragraph"),
                .paragraph("second paragraph"),
            ])
    }
}

@Suite("MarkdownBlock parsing — regression")
struct MarkdownBlockRegressionTests {

    /// A realistic streamed reply mixing every block type in one pass. If
    /// block detection reverts to the old single-`.text`-blob behavior (or
    /// any one branch stops being checked before the plain-paragraph
    /// fallback), this collapses back into far fewer blocks than expected —
    /// unlike the narrower per-type tests above, which would still each pass
    /// individually against a parser that only handles one case at a time.
    @Test(
        "a full reply mixing heading, list, quote, rule, code and prose parses as distinct blocks")
    func mixedDocument() {
        let markdown = """
            ## Plan

            - step one
            - step two

            > a caveat worth reading

            ---

            ```swift
            let x = 1
            ```

            Done.
            """
        let blocks = MarkdownBlock.parse(markdown)
        #expect(
            blocks == [
                .heading(level: 2, text: "Plan"),
                .unorderedListItem("step one"),
                .unorderedListItem("step two"),
                .quote("a caveat worth reading"),
                .rule,
                .code("let x = 1", "swift"),
                .paragraph("Done."),
            ])
    }

    /// `blockQuote(_:)` and the `.quote` case used to share the name `quote`,
    /// and that ambiguity made every quote silently fall through to the
    /// plain-paragraph branch instead of raising a compiler error — the kind
    /// of regression a rename or refactor could reintroduce without an
    /// obvious build failure. Asserting `.quote` shows up correctly inside a
    /// larger document (not just in isolation) guards against that class of
    /// silent-wrong-overload bug recurring.
    @Test("a block quote surrounded by other blocks is not swallowed into a paragraph")
    func quoteNotSwallowedByParagraph() {
        let blocks = MarkdownBlock.parse("intro line\n> quoted line\noutro line")
        #expect(
            blocks == [
                .paragraph("intro line"),
                .quote("quoted line"),
                .paragraph("outro line"),
            ])
    }
}
