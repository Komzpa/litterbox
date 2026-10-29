import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "JournalDelete"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    property var store: null
    readonly property string noteId: "11111111-1111-4111-8111-111111111111"
    property var posted: []
    Connections {
        target: store
        function onRequestPost(opId, path, body) {
            posted = posted.concat([{path: path, body: body}])
            store.reportPostResult(opId, 200, {ok: true})
        }
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
    function test_delete_data() {
        return [{tag: "phone", width: 412}, {tag: "neighbor", width: 480}]
    }
    function test_delete(row) {
        store = bundleFixture.freshStore()
        store.online = false
        posted = []
        const snapshot = {now: [{id: noteId, source: "journal", state: "open", title: "Private journal note", summary: "A disposable local proof note", pinned_rank: null}], later: [], missed: []}
        verify(store.applyRemoteCards(snapshot))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: row.width, height: 915, x: 0, y: 0
        })
        verify(inbox)
        inbox.show()
        tryVerify(function() { return inbox.captureActions[noteId] !== undefined })
        verify(inbox.captureOpenActions(noteId))
        const actions = inbox.captureActions[noteId]
        const sheet = actions.actionSheetItem()
        tryCompare(sheet, "opened", true)
        const deletion = findChild(actions, "actionRow-delete-" + noteId)
        verify(deletion.visible)
        compare(deletion.text, "Delete")
        compare(deletion.Accessible.name, deletion.text)
        verify(deletion.width >= 48 && deletion.height >= 48)
        wait(200)
        if (bundleProofDirectory) grabImage(inbox.contentItem.parent).save(bundleProofDirectory + "/delete-menu-" + row.width + "x915.png")
        mouseClick(deletion)
        const confirmation = findChild(inbox, "deleteJournalDialog")
        tryCompare(confirmation, "opened", true)
        compare(store.pendingOps, 0)
        mouseClick(findChild(inbox, "deleteJournalCancel"))
        tryCompare(confirmation, "visible", false)
        compare(store.cardIds().length, 1)
        compare(store.pendingOps, 0)
        verify(inbox.captureOpenActions(noteId))
        tryCompare(sheet, "opened", true)
        mouseClick(deletion)
        tryCompare(confirmation, "opened", true)
        if (bundleProofDirectory) grabImage(inbox.contentItem.parent).save(bundleProofDirectory + "/delete-confirm-" + row.width + "x915.png")
        mouseClick(findChild(inbox, "deleteJournalConfirm"))
        tryCompare(confirmation, "visible", false)
        compare(store.pendingOps, 1)
        compare(store.cardIds().length, 0)
        verify(store.applyRemoteCards(snapshot))
        compare(store.cardIds().length, 0, "pending delete must mask stale snapshots")
        store.online = true
        tryCompare(store, "pendingOps", 0)
        compare(posted.length, 1)
        compare(posted[0].path, "/v1/ops")
        compare(posted[0].body.type, "delete_journal")
        compare(posted[0].body.card_id, noteId)
        inbox.close()
    }
}
