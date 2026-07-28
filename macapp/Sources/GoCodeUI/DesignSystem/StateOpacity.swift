import SwiftUI

/// State fills share a restrained transparency ramp. Names retain each current
/// visual level while keeping state strength from becoming a scattered number.
enum StateOpacity {
    static let subtle: Double = 0.08
    static let feedback: Double = 0.10
    static let emphasis: Double = 0.12
    static let selected: Double = 0.16
}
