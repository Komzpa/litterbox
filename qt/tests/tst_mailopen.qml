import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "MailCardOpening"
    width: 700
    height: 800
    property url pagesDir: Qt.resolvedUrl("../pages/")

    ListModel {
        id: store
        property bool online: false
        property var cached: ({})
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) {}
        function cardIds() { return Array.from({length: count}, (_, i) => get(i).cardId) }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function refresh() {}
        function enqueueOp(cardId, operation, args) { fail("Opening a card must not change it") }
    }
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

    function init() {
        store.clear()
        store.cached = ({"mail-8": {html: "<p>Hello <b>reader</b></p>"}})
        for (let i = 0; i < 14; ++i) {
            const id = "mail-" + i
            store.append({cardId: id, title: "Message " + i, section: "now", card: {
                id: id, source: "mail", section: "now", account_name: "fixture@example.test",
                summary: "Click this preview to read the message", has_body: true,
                pinned_rank: null, bundle_id: "", important: false, timed: false,
                note: "", source_url: ""
            }})
        }
    }

    function test_previewOpensMailAndBackPreservesScroll() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 14)
        wait(100)
        list.forceLayout()
        list.positionViewAtIndex(8, ListView.Beginning)
        wait(100)
        const before = list.contentY
        verify(before > 0)
        const row = list.itemAtIndex(8)
        verify(row)
        function preview(item) {
            if (item.text === "Click this preview to read the message") return item
            for (const child of item.children || []) {
                const found = preview(child)
                if (found) return found
            }
            return null
        }
        const summary = preview(row)
        verify(summary)
        mouseClick(summary, summary.width / 2, summary.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(stack.currentItem, "html", "<p>Hello <b>reader</b></p>")
        tryCompare(stack, "busy", false)
        const back = findChild(stack.currentItem, "backToInbox")
        verify(back)
        verify(back.width >= 48 && back.height >= 48)
        mouseClick(back)
        tryCompare(stack, "depth", 1)
        tryCompare(stack, "busy", false)
        compare(list.contentY, before)
        compare(list.itemAtIndex(8).cardId, "mail-8")
        inbox.close()
    }

    function test_bodylessCardDoesNotOpenEmptyDetail() {
        store.clear()
        store.append({cardId: "manual-1", title: "Plain task", section: "now", card: {
            id: "manual-1", source: "manual", section: "now", has_body: false,
            pinned_rank: null, bundle_id: "", important: false, timed: false,
            summary: "", note: "", account_name: "", source_url: ""
        }})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        list.forceLayout()
        const row = list.itemAtIndex(0)
        verify(row)
        mouseClick(row, row.width / 2, row.height / 2)
        compare(findChild(inbox, "pageStack").depth, 1)
        inbox.close()
    }
}
