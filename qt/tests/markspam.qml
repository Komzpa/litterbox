import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "MarkSpam"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    readonly property string mailId: "11111111-1111-4111-8111-111111111111"
    property var posted: []
    property bool acceptJournal: false
    Connections {
        target: bundleStore
        function onRequestPost(opId, path, body) {
            posted = posted.concat([{opId: opId, path: path, body: body}])
            bundleStore.reportPostResult(opId, body.type === "create_card" && acceptJournal ? 200 : 422,
                                         {ok: body.type === "create_card" && acceptJournal})
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
    function test_rejectedJournalRetry() {
        bundleStore.online = false
        posted = []
        acceptJournal = false
        verify(bundleStore.applyRemoteCards({now: [], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules,
            width: 412, height: 915
        })
        verify(inbox)
        inbox.show()
        verify(bundleStore.createCard("Retain this journal\nAll of this text", "", "journal"))
        const journalId = bundleStore.cardIds()[0]
        bundleStore.online = true
        tryCompare(bundleStore, "saveFailed", true)
        compare(bundleStore.pendingOps, 1)
        const line = findChild(inbox, "connectionStatusRow")
        tryCompare(line, "visible", true)
        const status = findChild(inbox, "connectionStatus")
        verify(status.text.indexOf("Couldn't save to server") >= 0)
        compare(inbox.bundleArchiveStatus, "", "No raw HTTP error toast for a retained write")
        tryVerify(function() { return inbox.captureActions[journalId] !== undefined })
        verify(inbox.captureActions[journalId].visible, "Journal remains rendered in the inbox")
        acceptJournal = true
        const retry = findChild(inbox, "offlineRetry")
        verify(retry.visible && retry.enabled)
        mouseClick(retry)
        tryCompare(bundleStore, "pendingOps", 0)
        compare(bundleStore.saveFailed, false)
        tryCompare(line, "visible", false)
        compare(posted.length, 2)
        compare(posted[1].body.op_id, posted[0].body.op_id)
        compare(bundleStore.cardIds()[0], journalId)
        inbox.close()
    }
    function test_menu_data() {
        return [{tag: "desktop", width: 1440, height: 900},
                {tag: "phone", width: 412, height: 915}]
            .filter(row => !bundleProofWidth || row.width === bundleProofWidth)
    }
    function test_menu(row) {
        bundleStore.online = false
        posted = []
        verify(bundleStore.applyRemoteCards({now: [{id: mailId, source: "mail", state: "open",
            title: "Выиграйте аренду автомобиля в Hertz", sender_name: "Flying Blue",
            account_name: "fixture@example.test", pinned_rank: null, summary: "Flying Blue", has_body: true}], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules,
            width: row.width, height: row.height, x: 0, y: 0
        })
        verify(inbox)
        tryVerify(function() { return inbox.captureActions[mailId] !== undefined })
        verify(inbox.captureOpenActions(mailId))
        const actions = inbox.captureActions[mailId]
        const sheet = findChild(actions, "cardActionSheet-" + mailId)
        tryCompare(sheet, "opened", true)
        const archive = findChild(actions, "actionRow-primary-" + mailId)
        const spam = findChild(actions, "actionRow-spam-" + mailId)
        verify(spam && spam.visible)
        compare(spam.text, "Mark as spam · fixture@example.test")
        compare(spam.Accessible.name, spam.text)
        compare(spam.icon.name, "mail-mark-junk")
        verify(spam.height >= 48 && spam.width >= 48)
        verify(spam.y >= archive.y + archive.height)
        fuzzyCompare(spam.y - archive.y - archive.height, spam.parent.spacing, 1,
                     "Spam must be the next distinct action, with only the action list's normal gap")
        if (bundleProofDirectory) {
            console.log("SPAM_MENU_READY", row.width)
            wait(6000)
        }
        mouseClick(spam)
        tryVerify(function() { return bundleStore.cardIds().length === 0 })
        compare(bundleStore.pendingOps, 1)
        compare(inbox.bundleArchiveStatus, "Marked as spam")
        verify(inbox.undoBundleArchive())
        compare(bundleStore.cardIds().length, 1)
        compare(bundleStore.pendingOps, 0)
        bundleStore.online = true
        wait(100)
        compare(posted.length, 0, "Undo before flush sends nothing")
        const rejectedOp = bundleStore.enqueueOp(mailId, "spam", {})
        verify(rejectedOp.length > 0)
        tryVerify(function() { return bundleStore.pendingOps === 0 })
        compare(posted.length, 1)
        compare(posted[0].path, "/v1/ops")
        compare(posted[0].body.type, "spam")
        compare(bundleStore.cardIds().length, 1)
        compare(inbox.bundleArchiveStatus, "Action failed (HTTP 422).")
        verify(!inbox.archiveUndoAvailable)
        inbox.close()
    }
}
