import HarnessKit

/// Presentation-only grouping collapses adjacent tool events without changing
/// transcript storage, clipboard output, or the inspector's source data.
struct TranscriptDisplayItem: Identifiable {
    enum Kind {
        case item(TranscriptItem)
        case toolActivities([ToolActivity])
    }

    let id: String
    let kind: Kind
}

enum TranscriptPresentation {
    static func rows(for items: [TranscriptItem]) -> [TranscriptDisplayItem] {
        var rows: [TranscriptDisplayItem] = []
        var index = 0

        while index < items.count {
            let item = items[index]
            guard case .toolActivity(let first) = item.kind else {
                rows.append(.init(id: item.id.uuidString, kind: .item(item)))
                index += 1
                continue
            }

            var activities = [first]
            var lastID = item.id.uuidString
            index += 1
            while index < items.count, case .toolActivity(let activity) = items[index].kind {
                activities.append(activity)
                lastID = items[index].id.uuidString
                index += 1
            }
            rows.append(.init(id: "tool-group-\(lastID)", kind: .toolActivities(activities)))
        }
        return rows
    }
}
