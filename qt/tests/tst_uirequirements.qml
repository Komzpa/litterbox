import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "UiRequirements"
    width: 520
    height: 800

    ListModel {
        id: store
        property bool online: false
        function cardIds() { return ["card-1"] }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function dismiss(cardId) {}
        function refresh() {}
        function createCard(title, summary) { return true }
        function saveNote(cardId, note) {}
        function moveCardTo(cardId, index) { return true }
        function enqueueOp(cardId, operation, args) {}
    }
    QtObject {
        id: api
        property string baseUrl: ""
        property string token: ""
    }
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
        store.append({cardId: "card-1", title: "UI requirements card", section: "now", card: {
            id: "card-1", title: "UI requirements card", source: "manual", section: "now",
            has_body: false, pinned_rank: null, bundle_id: "", important: false,
            timed: false, note: "", summary: "", account_name: ""
        }})
    }

    function createInbox() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules
        })
        verify(inbox)
        inbox.show()
        return inbox
    }


    function test_realInboxActionsAreNamedAndLargeAndSheetPaletteIsLight() {
        const inbox = createInbox()
        const list = findChild(inbox, "inboxList")
        verify(list)
        list.forceLayout()
        compare(list.count, 1)
        const row = list.itemAtIndex(0)
        verify(row)
        const moreActions = findChild(row, "inboxActions")
        verify(moreActions)
        compare(moreActions.icon.name, "overflow-menu")
        verify(moreActions.implicitWidth >= 48)
        verify(moreActions.implicitHeight >= 48)

        const done = findChild(row, "doneButton-card-1")
        verify(done)
        verify(done.icon.name.length > 0 || done.text.length > 0)
        verify(done.implicitWidth >= 48)
        verify(done.implicitHeight >= 48)

        mouseClick(moreActions)
        const sheet = findChild(moreActions, "cardActionSheet-card-1")
        verify(sheet)
        tryVerify(function() { return sheet.visible && sheet.contentItem !== null })
        verify(Qt.colorEqual(sheet.contentItem.palette.window, "#ffffff"))
        verify(Qt.colorEqual(sheet.contentItem.palette.base, "#ffffff"))

        const close = findChild(sheet, "actionSheetClose-card-1")
        verify(close)
        verify(close.text.length > 0)
        verify(close.implicitWidth >= 48)
        verify(close.implicitHeight >= 48)
        let checkedActions = 0
        for (const name of ["primary", "open", "cached", "note", "snooze", "pin", "bundle", "takeout"]) {
            const control = findChild(sheet, "actionRow-" + name + "-card-1")
            verify(control, "missing real action row " + name)
            if (!control.visible) continue
            checkedActions++
            verify(String(control.icon.name || "").length > 0 || String(control.text || "").trim().length > 0)
            verify(control.width >= 48 && control.height >= 48,
                   "action row target below 48px: " + name + " " + control.width + "x" + control.height)
        }
        verify(checkedActions > 0)
    }
}
