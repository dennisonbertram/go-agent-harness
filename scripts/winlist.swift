import CoreGraphics
import Foundation
// CGWindowList is a different API from accessibility. If it sees a window while
// AX does not, the window exists and only AX is gated — a materially different
// problem than "the session composites nothing".
let opts: CGWindowListOption = [.optionAll]
guard let list = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String: Any]] else {
    print("no window list"); exit(1)
}
let target = CommandLine.arguments.count > 1 ? Int(CommandLine.arguments[1]) ?? -1 : -1
var mine = 0
for w in list {
    let owner = w[kCGWindowOwnerName as String] as? String ?? ""
    let pid = w[kCGWindowOwnerPID as String] as? Int ?? -1
    if pid == target || owner == "GoCode" {
        mine += 1
        let b = w[kCGWindowBounds as String] as? [String: Any] ?? [:]
        print("window: owner=\(owner) pid=\(pid) id=\(w[kCGWindowNumber as String] ?? "?") bounds=\(b) layer=\(w[kCGWindowLayer as String] ?? "?")")
    }
}
print("total windows on system: \(list.count), belonging to target: \(mine)")
