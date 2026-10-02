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
        store.append({cardId: "card-1", title: "Touch card", card: {
            id: "card-1", title: "Touch card", source: "manual", section: "now",
            has_body: false, pinned_rank: null, bundle_id: "", important: false,
            timed: false, note: "", summary: "", account_name: ""
        }})
    }

    function createInbox() {
        return createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules
        })
    }

    function test_doneActionIsVisibleWithoutHover() {
        const inbox = createInbox()
        verify(inbox)
        const done = findChild(inbox, "doneButton-card-1")
        verify(done)
        compare(done.opacity, 1)
    }

    function test_addCardOpensWithPointerTap() {
        const inbox = createInbox()
        verify(inbox)
        const button = findChild(inbox, "addCardButton")
        verify(button)
        mouseClick(button)
        const dialog = findChild(inbox, "createDialog")
        verify(dialog)
        tryCompare(dialog, "visible", true)
    }

    function test_backPopsEnrollmentPageToInbox() {
        const inbox = createInbox()
        verify(inbox)
        const stack = findChild(inbox, "pageStack")
        verify(stack)
        stack.push(pushedPage)
        tryCompare(stack, "depth", 2)
        keyClick(stack, Qt.Key_Back)
        tryCompare(stack, "depth", 1)
    }
}
