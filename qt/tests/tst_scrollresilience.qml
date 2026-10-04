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
}
