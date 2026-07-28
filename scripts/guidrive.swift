import CoreGraphics
import Foundation
import Vision
import AppKit

// Drive and read a window on a locked session.
//
// Accessibility is gated while locked, and the session event tap swallows
// synthetic input — but CGEventPostToPid delivers straight to the process, and
// the window still composites so its pixels can be captured and read with
// Vision. Together that is a complete GUI loop: real clicks, real typing, real
// rendering, and text read back off the screen rather than from a model's reply.

let args = CommandLine.arguments
let pid = pid_t(args[1])!
let mode = args[2]

func post(_ e: CGEvent?) { e?.postToPid(pid) }
let src = CGEventSource(stateID: .hidSystemState)

func click(_ x: Double, _ y: Double) {
    let p = CGPoint(x: x, y: y)
    post(CGEvent(mouseEventSource: src, mouseType: .mouseMoved, mouseCursorPosition: p, mouseButton: .left))
    usleep(80_000)
    post(CGEvent(mouseEventSource: src, mouseType: .leftMouseDown, mouseCursorPosition: p, mouseButton: .left))
    usleep(50_000)
    post(CGEvent(mouseEventSource: src, mouseType: .leftMouseUp, mouseCursorPosition: p, mouseButton: .left))
}

func type(_ text: String) {
    for ch in Array(text.utf16) {
        var u = ch
        let d = CGEvent(keyboardEventSource: src, virtualKey: 0, keyDown: true)
        d?.keyboardSetUnicodeString(stringLength: 1, unicodeString: &u); post(d)
        usleep(5_000)
        let up = CGEvent(keyboardEventSource: src, virtualKey: 0, keyDown: false)
        up?.keyboardSetUnicodeString(stringLength: 1, unicodeString: &u); post(up)
        usleep(5_000)
    }
}

func key(_ code: CGKeyCode, flags: CGEventFlags = []) {
    let d = CGEvent(keyboardEventSource: src, virtualKey: code, keyDown: true); d?.flags = flags; post(d)
    usleep(30_000)
    let u = CGEvent(keyboardEventSource: src, virtualKey: code, keyDown: false); u?.flags = flags; post(u)
}

/// Read every line of text the window is currently rendering.
func ocr(_ path: String) -> [String] {
    guard let img = NSImage(contentsOfFile: path),
          let cg = img.cgImage(forProposedRect: nil, context: nil, hints: nil) else { return [] }
    let req = VNRecognizeTextRequest()
    req.recognitionLevel = .accurate
    req.usesLanguageCorrection = false
    try? VNImageRequestHandler(cgImage: cg, options: [:]).perform([req])
    return (req.results ?? []).compactMap { $0.topCandidates(1).first?.string }
}

switch mode {
case "click": click(Double(args[3])!, Double(args[4])!); print("ok")
case "type":  type(args[3]); print("ok")
case "return": key(36); print("ok")
case "selectall": key(0, flags: .maskCommand); print("ok")   // cmd+A
case "delete": key(51); print("ok")
case "ocr":   ocr(args[3]).forEach { print($0) }
default: print("usage: guidrive <pid> click X Y | type T | return | selectall | delete | ocr PATH")
}
