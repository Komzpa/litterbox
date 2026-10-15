import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    id: testCase
    name: "ScrollResilience"
    when: windowShown
    QtObject { id: api; property string baseUrl: ""; property string token: "" }
    QtObject {
        id: updater
        property bool supported: false
        property bool busy: false
        signal noUpdateAvailable()
        signal installConsentRequired()
        signal errorOccurred(string message)
        signal updateReady(string version, string sha256)
    }
    QtObject { id: timeRules; function display(value) { return value } }
    Component { id: inboxComponent; App.InboxView {} }
    SignalSpy { id: presentationChanges; signalName: "modelChanged" }
    SignalSpy { id: sourceMoves; target: bundleStore; signalName: "rowsMoved" }

    function card(id, bundle, extra) {
        return Object.assign({id: id, title: id, source: "mail", section: "now",
            state: "open", bundle_id: bundle, bundle_title: "GitHub", pinned_rank: null,
            important: false, timed: false, has_body: false, summary: "", note: "",
            account_name: "test@example.test"}, extra || {})
    }
    // R68: archiving or a fresh server snapshot must keep the visual anchor:
    // the first visible card stays first; if it is the removed one, the next
    // card takes its place. Never jump to the top or elsewhere.
    function test_snapshotKeepsVisualAnchor() {
        const ids = []
        const cards = []
        for (let i = 0; i < 200; ++i) { ids.push("row-" + i); cards.push(card("row-" + i, "", {title: "Row " + i})) }
        verify(bundleStore.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules, width: 549, height: 1280
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        list.positionViewAtIndex(100, ListView.Beginning)
        list.forceLayout()
        wait(50)
        function topId() {
            list.forceLayout()
            const index = list.indexAt(list.width / 2, list.contentY + 2)
            const item = index >= 0 ? list.itemAtIndex(index) : null
            return item ? item.cardId : ""
        }
        const anchor = topId()
        verify(anchor.length > 0 && anchor !== "row-0", "scrolled to the middle: " + anchor)
        const anchorIndex = ids.indexOf(anchor)
        function snapshotWithout(removed) {
            const next = cards.filter(function(c) { return removed.indexOf(c.id) < 0 })
            verify(bundleStore.applyRemoteCards({now: next, later: [], missed: []}))
            wait(100)
        }
        // A card above the viewport disappears: the anchor stays on top.
        snapshotWithout(["row-10"])
        compare(topId(), anchor)
        // The same snapshot again (stale refresh) changes nothing.
        snapshotWithout(["row-10"])
        compare(topId(), anchor)
        // The top card itself is archived: the next card takes its place.
        snapshotWithout(["row-10", anchor])
        compare(topId(), ids[anchorIndex + 1])
    }
    function test_expand308AndWheelScroll() {
        const cards = []
        // Non-adjacent members reproduce the installed 86eb78c path, not an
        // already-grouped input on which the old move loop happens to be cheap.
        for (let i = 0; i < 308; ++i) {
            cards.push(card("github-" + i, "github"))
            cards.push(card("other-" + i, ""))
        }
        for (let i = cards.length; i < 1100; ++i) cards.push(card("other-" + i, ""))
        verify(bundleStore.applyRemoteCards({now: cards, later: [], missed: []}))
        const sourceIds = bundleStore.cardIds()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules,
            width: 520, height: 800
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        wait(100)
        compare(list.count, 1100)
        const presentation = findChild(inbox, "bundlePresentation")
        presentationChanges.signalName = presentation ? "rowsChanged" : "modelChanged"
        presentationChanges.target = presentation || list
        presentationChanges.clear()
        sourceMoves.clear()
        const beforeCalls = inbox.groupingInvocations === undefined ? 0 : inbox.groupingInvocations
        const beforeMoves = inbox.groupingMoves === undefined ? 0 : inbox.groupingMoves
        const start = Date.now()
        inbox.toggleBundle("github")
        wait(0)
        list.forceLayout()
        const expandedMs = Date.now() - start
        console.log("SCROLL_EXPAND", expandedMs, "ms", "presentationChanges", presentationChanges.count,
                    "groupBundles", inbox.groupingInvocations === undefined ? "removed" : inbox.groupingInvocations - beforeCalls,
                    "moves", inbox.groupingMoves === undefined ? sourceMoves.count : inbox.groupingMoves - beforeMoves)
        for (let i = 0; i < 308; ++i) compare(inbox.cardIdAt(i), "github-" + i)
        compare(bundleStore.cardIds(), sourceIds, "Presentation must not reorder CardStore")
        compare(sourceMoves.count, 0)
        const changes = presentationChanges.count
        wait(100)
        compare(presentationChanges.count, changes, "Presentation must not re-trigger itself")
        console.log("SCROLL_SETTLED", "presentationChanges", presentationChanges.count,
                    "groupBundles", inbox.groupingInvocations === undefined ? "removed" : inbox.groupingInvocations - beforeCalls,
                    "moves", inbox.groupingMoves === undefined ? sourceMoves.count : inbox.groupingMoves - beforeMoves)
        const oldY = list.contentY
        mouseWheel(list, list.width / 2, list.height / 2, 0, -120, Qt.NoButton)
        tryVerify(function() { return list.contentY > oldY })
        console.log("INBOX_WHEEL", oldY, "->", list.contentY)
        mouseWheel(list, list.width / 2, list.height / 2, 0, 120, Qt.NoButton)
        tryVerify(function() { return list.contentY === oldY }, 5000, "Reverse wheel must restore the inbox position")
        const collapseStart = Date.now()
        inbox.toggleBundle("github")
        wait(0)
        list.forceLayout()
        const collapsedMs = Date.now() - collapseStart
        console.log("SCROLL_COLLAPSE", collapsedMs, "ms", "moves", sourceMoves.count)
        list.positionViewAtBeginning()
        list.forceLayout()
        const member = list.itemAtIndex(2)
        verify(member && member.cardId === "github-1")
        compare(member.visible, false)
        compare(member.height, 0, "Collapsed members contribute no height")
        compare(bundleStore.cardIds(), sourceIds)
        verify(expandedMs < 100, "308-member expand took " + expandedMs + " ms (limit <100 ms)")
        verify(collapsedMs < 100, "308-member collapse took " + collapsedMs + " ms (limit <100 ms)")
        inbox.close()
    }
    function test_reclusterPreservesScrollAndExpansion() {
        const cards = []
        for (let i = 0; i < 1100; ++i) {
            const bundle = i >= 55 && i <= 70 ? "anchor" : i >= 500 && i < 900 ? "old-cluster" : ""
            cards.push(card("scroll-" + i, bundle))
        }
        verify(bundleStore.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules,
            width: 520, height: 800
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.count === 1100 })
        list.contentY = 5000
        list.forceLayout()
        wait(100)
        const topIndex = list.indexAt(20, list.contentY + 20)
        verify(topIndex >= 0)
        const topItem = list.itemAtIndex(topIndex)
        verify(topItem)
        const rowHeight = topItem.height
        inbox.toggleBundle("anchor")
        wait(0)
        list.forceLayout()
        verify(inbox.expandedBundles.anchor)
        const scrollBeforeRefresh = list.contentY
        const visibleIndexBeforeRefresh = list.indexAt(20, list.contentY + 20)
        const visibleIdBeforeRefresh = inbox.cardIdAt(visibleIndexBeforeRefresh)
        const reclustered = cards.map(function(value, index) {
            return index >= 500 && index < 900
                ? Object.assign({}, value, {bundle_id: "new-cluster"}) : value
        })
        verify(bundleStore.applyRemoteCards({now: reclustered, later: [], missed: []}))
        wait(100)
        list.forceLayout()
        verify(visibleIndexBeforeRefresh >= 0)
        verify(visibleIdBeforeRefresh.length > 0)
        verify(Math.abs(list.contentY - scrollBeforeRefresh) <= rowHeight,
               "Refresh must retain viewport position within one row height")
        const visibleIndexAfterRefresh = list.indexAt(20, list.contentY + 20)
        verify(visibleIndexAfterRefresh >= 0)
        compare(inbox.cardIdAt(visibleIndexAfterRefresh), visibleIdBeforeRefresh,
                "Same top card must remain at the viewport top")
        compare(inbox.expandedBundles.anchor, true)
        let anchorVisible = false
        for (let i = 0; i < list.count; ++i)
            if (inbox.cardIdAt(i) === "scroll-60") { anchorVisible = true; break }
        verify(anchorVisible, "Expanded bundle remains in the presentation")
        inbox.close()
    }
}
