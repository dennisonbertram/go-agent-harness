import Testing

@testable import GoCodeUI

@Suite("Collection load states")
struct CollectionLoadStateTests {

    @Test("an empty state is only truthful after a successful empty load")
    func emptyStateRequiresLoadedCollection() {
        #expect(!CollectionLoadState.idle.showsEmptyState(itemCount: 0))
        #expect(!CollectionLoadState.loading.showsEmptyState(itemCount: 0))
        #expect(!CollectionLoadState.failed("boom").showsEmptyState(itemCount: 0))
        #expect(CollectionLoadState.loaded.showsEmptyState(itemCount: 0))
        #expect(!CollectionLoadState.loaded.showsEmptyState(itemCount: 1))
    }

    /// The core regression: `.failed` used to render identically to
    /// `.loading` (an endless skeleton, since neither was `.loaded`). A
    /// failure with nothing on screen yet must not claim a result is still
    /// pending.
    @Test("a failed load never shows a loading placeholder")
    func failedNeverShowsPlaceholder() {
        #expect(!CollectionLoadState.failed("boom").showsPlaceholder(itemCount: 0))
    }

    @Test("loading shows a placeholder only while nothing is on screen yet")
    func loadingShowsPlaceholderOnlyWhenEmpty() {
        #expect(CollectionLoadState.loading.showsPlaceholder(itemCount: 0))
        #expect(!CollectionLoadState.loading.showsPlaceholder(itemCount: 3))
    }

    @Test("idle shows a placeholder before the first load starts")
    func idleShowsPlaceholderBeforeFirstLoad() {
        #expect(CollectionLoadState.idle.showsPlaceholder(itemCount: 0))
        #expect(!CollectionLoadState.idle.showsPlaceholder(itemCount: 1))
    }

    @Test("a failed state carries its own message, verbatim")
    func failedCarriesItsMessage() {
        #expect(CollectionLoadState.failed("boom").errorMessage == "boom")
        #expect(CollectionLoadState.loading.errorMessage == nil)
    }

    @Test("showsError is true only for a failed state")
    func showsErrorOnlyForFailed() {
        #expect(CollectionLoadState.failed("boom").showsError)
        #expect(!CollectionLoadState.loaded.showsError)
        #expect(!CollectionLoadState.loading.showsError)
        #expect(!CollectionLoadState.idle.showsError)
    }

    @Test("a failed first load is a blocking error")
    func failedFirstLoadShowsBlockingError() {
        #expect(CollectionLoadState.failed("boom").showsBlockingError(itemCount: 0))
        #expect(!CollectionLoadState.failed("boom").showsBlockingError(itemCount: 2))
        #expect(!CollectionLoadState.loading.showsBlockingError(itemCount: 0))
    }

    @Test("a failed refresh preserves stale rows with a nonblocking error")
    func failedRefreshShowsNonblockingError() {
        #expect(CollectionLoadState.failed("boom").showsRefreshError(itemCount: 2))
        #expect(!CollectionLoadState.failed("boom").showsRefreshError(itemCount: 0))
        #expect(!CollectionLoadState.loaded.showsRefreshError(itemCount: 2))
    }
}
