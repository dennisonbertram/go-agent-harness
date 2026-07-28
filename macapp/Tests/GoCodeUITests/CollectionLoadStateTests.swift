import Testing

@testable import GoCodeUI

@Suite("Collection load states")
struct CollectionLoadStateTests {

    @Test("an empty state is only truthful after a successful empty load")
    func emptyStateRequiresLoadedCollection() {
        #expect(!CollectionLoadState.idle.showsEmptyState(itemCount: 0))
        #expect(!CollectionLoadState.loading.showsEmptyState(itemCount: 0))
        #expect(!CollectionLoadState.failed.showsEmptyState(itemCount: 0))
        #expect(CollectionLoadState.loaded.showsEmptyState(itemCount: 0))
        #expect(!CollectionLoadState.loaded.showsEmptyState(itemCount: 1))
    }
}
