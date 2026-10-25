import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "TouchDefects"
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
    Component { id: pushedPage; Item {} }

    function init() {
        store.clear()
        store.append({cardId: "card-1", title: "Touch card", section: "now", card: {
            id: "card-1", title: "Touch card", source: "manual", section: "now",
            has_body: false, pinned_rank: null, bundle_id: "", important: false,
            timed: false, note: "", summary: "", account_name: ""
        }})
    }

    function createInbox() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules
        })
        inbox.show()
        return inbox
    }

    SignalSpy { id: dialogOpened; signalName: "opened" }

    function test_doneActionIsVisibleWithoutHover() {
        const inbox = createInbox()
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        verify(list)
        list.forceLayout()
        compare(list.count, 1)
        const row = list.itemAtIndex(0)
        verify(row)
        const done = findChild(row, "doneButton-card-1")
        verify(done)
        compare(done.opacity, 1)
    }
    function test_addCardOpensOnceWithPointerTap() {
        const inbox = createInbox()
        verify(inbox)
        const button = findChild(inbox, "addCardButton")
        const dialog = findChild(inbox, "createDialog")
        verify(button)
        verify(dialog)
        dialogOpened.target = dialog
        dialogOpened.clear()
        mouseClick(button)
        tryCompare(dialog, "visible", true)
        tryCompare(dialogOpened, "count", 1)
    }

    function test_createJournalFitsReducedPhoneViewport_data() {
        return [{tag: "phone", width: 412}, {tag: "narrow", width: 360}]
    }

    function test_createJournalFitsReducedPhoneViewport(data) {
        const inbox = createInbox()
        inbox.width = data.width
        inbox.height = 915
        const dialog = findChild(inbox, "createDialog")
        findChild(inbox, "createKind").currentIndex = 1
        const note = findChild(inbox, "createJournal")
        note.text = "Private journal entry. ".repeat(80)
        dialog.open()
        tryCompare(dialog, "opened", true)
        // Android adjustResize leaves this much usable space above the IME.
        inbox.height = 420
        wait(250)
        const top = dialog.y
        verify(top >= 0, "Create dialog header must stay above the keyboard")
        verify(top + dialog.height <= dialog.parent.height,
               "Create dialog footer must fit the keyboard-reduced viewport")
        const save = findChild(inbox, "createSaveButton")
        const cancel = findChild(inbox, "createCancelButton")
        verify(save.visible && cancel.visible)
        verify(save.height >= 48 && cancel.height >= 48)
        verify(note.text.length > 1000, "Resizing must preserve the draft")
        mouseClick(cancel)
        tryCompare(dialog, "visible", false)
        inbox.close()
    }

    function test_backPopsEnrollmentPageToInbox() {
        const inbox = createInbox()
        verify(inbox)
        const stack = findChild(inbox, "pageStack")
        verify(stack)
        stack.push(pushedPage)
        tryCompare(stack, "depth", 2)
        verify(inbox.handleBack())
        tryCompare(stack, "depth", 1)
    }
}
