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
        property var operations: []
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) {}
        function cardIds() { return Array.from({length: count}, (_, i) => get(i).cardId) }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function refresh() {}
        function enqueueOp(cardId, operation, args) {
            operations = operations.concat([{cardId: cardId, type: operation, args: args}])
            if (operation === "archive" || operation === "done" || operation === "snooze") {
                const row = cardIds().indexOf(cardId)
                if (row >= 0) remove(row)
            }
            return "fixture-operation"
        }
        function dismiss(cardId) { return enqueueOp(cardId, "done", {}) }
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

    function populateStore(source) {
        store.clear()
        store.operations = []
        store.cached = ({"mail-8": {html: "<p>Hello <b>reader</b></p>"}})
        for (let i = 0; i < 14; ++i) {
            const id = "mail-" + i
            store.append({cardId: id, title: "Message " + i, section: "now", card: {
                id: id, source: source, section: "now", account_name: "fixture@example.test",
                summary: "Click this preview to read the message",
                snippet: "Click this preview to read the message", has_body: true,
                pinned_rank: null, bundle_id: "", important: false, timed: false,
                note: "", source_url: ""
            }})
        }
    }
    function init() { populateStore("mail") }

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
        compare(store.operations, [])
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
    function test_escapeAndMouseBackRestoreInboxPosition_data() {
        return [{tag: "escape", action: "escape"},
                {tag: "mouse-back", action: "back"}]
    }

    function test_escapeAndMouseBackRestoreInboxPosition(row) {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 14)
        list.forceLayout()
        list.positionViewAtIndex(8, ListView.Beginning)
        wait(100)
        const before = list.contentY - list.originY
        verify(before > 0)

        const preview = findChild(list.itemAtIndex(8), "mailSnippet-mail-8")
        verify(preview)
        mouseClick(preview, preview.width / 2, preview.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(stack.currentItem, "html", "<p>Hello <b>reader</b></p>")

        if (row.action === "escape") {
            stack.currentItem.forceActiveFocus()
            keyClick(Qt.Key_Escape)
        } else {
            mouseClick(inbox.contentItem, inbox.width / 2, inbox.height / 2, Qt.BackButton)
        }
        tryCompare(stack, "depth", 1)
        tryCompare(list, "contentY", list.originY + before)
        compare(list.itemAtIndex(8).cardId, "mail-8")
        compare(store.operations, [])
        inbox.close()
    }

    function test_keyboardOpensAndClosesMail() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        list.forceLayout()
        const card = findChild(list.itemAtIndex(0), "openCard-mail-0")
        verify(card)
        list.forceActiveFocus()
        for (let i = 0; i < 20 && !card.activeFocus; ++i)
            keyClick(Qt.Key_Tab)
        tryVerify(function() { return card.activeFocus })
        keyClick(Qt.Key_Return)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        keyClick(Qt.Key_Escape)
        tryCompare(stack, "depth", 1)
        compare(store.operations, [])
        inbox.close()
    }


    function test_actionReturnsToSameInboxOffset_data() {
        return [{tag: "archive-button", action: "archive"},
                {tag: "archive-key", action: "key"},
                {tag: "snooze", action: "snooze"},
                {tag: "done-body", action: "done"}]
    }

    function test_actionReturnsToSameInboxOffset(row) {
        if (row.action === "done")
            populateStore("manual")
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 14)
        list.forceLayout()
        list.positionViewAtIndex(6, ListView.Beginning)
        wait(100)
        const before = list.contentY - list.originY
        const anchorY = list.itemAtIndex(6).mapToItem(list, 0, 0).y
        verify(before > 0)
        const preview = findChild(list.itemAtIndex(8), "openCard-mail-8")
        verify(preview)
        mouseClick(preview, preview.width / 2, preview.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(stack, "busy", false)
        const archive = findChild(stack.currentItem, "archiveMail")
        verify(archive && archive.visible, "Archive must be available while reading")
        if (row.action === "key") {
            archive.forceActiveFocus()
            keyClick(Qt.Key_E)
        } else if (row.action === "snooze") {
            const actions = findChild(stack.currentItem, "mailActions")
            verify(actions)
            actions.chooseSnoozeDateTime()
            const dateTime = findChild(actions, "snoozeDateTime")
            tryVerify(function() { return dateTime.activeFocus })
            keyClick(Qt.Key_E)
            compare(store.operations, [], "Typing in a dialog must not archive")
            compare(stack.depth, 2)
            const tomorrow = new Date(Date.now() + 86400000)
            verify(actions.captureSnooze(actions.localDateTime(tomorrow)))
        } else {
            mouseClick(archive)
        }
        tryCompare(stack, "depth", 1)
        tryCompare(stack, "busy", false)
        compare(list.contentY - list.originY, before)
        compare(Math.round(list.itemAtIndex(6).mapToItem(list, 0, 0).y), Math.round(anchorY))
        verify(store.cardIds().indexOf("mail-8") < 0)
        compare(store.operations.length, 1)
        compare(store.operations[0].type, row.action === "snooze" || row.action === "done" ? row.action : "archive")
        compare(store.operations[0].cardId, "mail-8")
        inbox.close()
    }

    function test_backRestoresExactCardPosition_data() {
        return [{tag: "w1440", width: 1440},
                {tag: "w598", width: 598}]
    }
    function test_backRestoresExactCardPosition(row) {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: row.width, height: 900
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 14)
        list.forceLayout()
        list.positionViewAtIndex(6, ListView.Beginning)
        wait(200)
        list.forceLayout()
        verify(list.contentY - list.originY > 0)
        // The reflow inserts a row above the opened card, so index lookups are
        // stale afterwards; track the card by id throughout.
        function rowOf(cardId) {
            for (let i = 0; i < list.count; ++i) {
                const item = list.itemAtIndex(i)
                if (item && item.cardId === cardId) return item
            }
            return null
        }
        const openedCard = rowOf("mail-8")
        verify(openedCard)
        const beforeY = openedCard.mapToItem(list, 0, 0).y
        const preview = findChild(openedCard, "openCard-mail-8")
        verify(preview)
        mouseClick(preview, preview.width / 2, preview.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(stack, "busy", false)
        // The inbox reflows while the mail is open: a new row arrives above the
        // opened card (a refresh can also re-estimate row heights), which
        // invalidates the raw contentY that the old Back path restored.
        store.insert(0, {cardId: "mail-reflow", title: "Message reflow", section: "now", card: {
            id: "mail-reflow", source: "mail", section: "now", account_name: "fixture@example.test",
            summary: "Arrived while reading", snippet: "Arrived while reading", has_body: true,
            pinned_rank: null, bundle_id: "", important: false, timed: false,
            note: "", source_url: ""
        }})
        tryCompare(list, "count", 15, 2000, "the inbox must relayout while the mail is open")
        const back = findChild(stack.currentItem, "backToInbox")
        verify(back)
        mouseClick(back)
        tryCompare(stack, "depth", 1)
        tryCompare(stack, "busy", false)
        wait(200)
        list.forceLayout()
        const restoredCard = rowOf("mail-8")
        verify(restoredCard)
        const afterY = restoredCard.mapToItem(list, 0, 0).y
        verify(afterY === beforeY,
               "Back must restore the opened card within 0 px: before=" + beforeY + " after=" + afterY)
        compare(store.operations, [])
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
