import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "MarkSpam"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    readonly property string mailId: "11111111-1111-4111-8111-111111111111"
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
    function test_menu_data() {
        return [{tag: "desktop", width: 1440, height: 900},
                {tag: "phone", width: 412, height: 915}]
            .filter(row => !bundleProofWidth || row.width === bundleProofWidth)
    }
    function test_menu(row) {
        bundleStore.online = false
        verify(bundleStore.applyRemoteCards({now: [{id: mailId, source: "mail", state: "open",
            title: "Выиграйте аренду автомобиля в Hertz", sender_name: "Flying Blue",
            account_name: "fixture@example.test", summary: "Flying Blue", has_body: true}], later: [], missed: []}))
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
        verify(spam.y < archive.y + archive.height + 2)
        if (bundleProofDirectory) {
            console.log("SPAM_MENU_READY", row.width)
            wait(6000)
        }
        mouseClick(spam)
        tryVerify(function() { return bundleStore.cardIds().length === 0 })
        compare(bundleStore.pendingOps, 1)
        compare(inbox.bundleArchiveStatus, "Marked as spam. You can undo this action.")
        verify(inbox.undoBundleArchive())
        compare(bundleStore.cardIds().length, 1)
        compare(bundleStore.pendingOps, 0)
        inbox.showOperationFailure("fixture-op", 422)
        compare(inbox.bundleArchiveStatus, "Action failed (HTTP 422). The card was restored.")
        verify(!inbox.archiveUndoAvailable)
        inbox.close()
    }
}
