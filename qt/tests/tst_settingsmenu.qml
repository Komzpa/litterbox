import QtQuick
import QtTest
import litterbox 1.0 as App
import "../pages" as Pages

TestCase {
    id: proof
    name: "SettingsMenu"
    width: 412
    height: 915
    visible: true
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    property string evidence: "/home/kom/Dropbox/Reemxy/tasks-loop/memory/task-evidence/settings-menu-20261014/"
    ListModel {
        id: store
        property bool online: true
        function refresh() {}
        function cardIds() { return [] }
        function pinnedCardIds() { return [] }
    }
    QtObject {
        id: api
        property string baseUrl: "https://inbox.example.test"
        property string token: "stored-enrollment-token"
        function get(path, callback) { callback(null, {body: [{address: "owner@example.test", status: "active"}]}) }
        function post(path, body, callback) { callback(null, {body: {token: "new-token", device_id: "device-123", tenant_id: "tenant-456"}}) }
    }
    QtObject {
        id: updater
        property bool supported: true
        property bool busy: false
        property int checks: 0
        function checkForUpdates() { checks++ }
        signal noUpdateAvailable()
        signal installConsentRequired()
        signal errorOccurred(string message)
        signal updateReady(string version, string sha256)
    }
    QtObject { id: timeRules; function display(value) { return value } }
    Component { id: inboxComponent; App.InboxView {} }
    Component { id: enrollmentComponent; Pages.EnrollmentPage {} }
    function initTestCase() {
        Qt.application.organization = "LitterboxSettingsProof"
        Qt.application.domain = "settings-proof.example.test"
    }

    function inbox(width) {
        const view = createTemporaryObject(inboxComponent, proof, {
            api: api, updater: updater, store: store, timeRules: timeRules,
            width: width, height: 915
        })
        verify(view)
        view.show()
        wait(200)
        return view
    }
    function shot(view, name) {
        wait(200)
        grabImage(view.contentItem.parent).save(evidence + name + ".png")
    }
    function test_capture_data() { return [{tag: "412dp", width: 412}, {tag: "480dp", width: 480}] }
    function test_capture(data) {
        const view = inbox(data.width)
        const entry = findChild(view, "headerMenuSettings")
        const phase = entry ? "after" : "before"
        const menu = findChild(view, "headerMenu")
        mouseClick(findChild(view, "inboxCommands"))
        tryCompare(menu, "opened", true)
        shot(view, phase + "-menu-" + data.width)
        menu.close()
        view.openPage(entry ? "SettingsPage" : "EnrollmentPage")
        const stack = findChild(view, "pageStack")
        tryCompare(stack, "busy", false)
        shot(view, phase + "-page-" + data.width)
        view.close()
    }
    function test_enrolledSkipsInvite() {
        const page = createTemporaryObject(enrollmentComponent, proof, {api: api, width: 412, height: 800})
        verify(page)
        wait(100)
        compare(findChild(page, "inviteField").visible, false)
        compare(findChild(page, "tokenField"), null)
    }
    function test_settingsMenuAndControls() {
        const view = inbox(412)
        const entry = findChild(view, "headerMenuSettings")
        verify(entry, "Settings entry must exist")
        mouseClick(findChild(view, "inboxCommands"))
        tryCompare(entry.menu, "opened", true)
        mouseClick(entry)
        const stack = findChild(view, "pageStack")
        tryCompare(stack, "busy", false)
        compare(stack.currentItem.objectName, "settingsPage")
        const page = stack.currentItem
        compare(findChild(page, "enrollmentStatus").text, "Enrolled")
        verify(findChild(page, "accountSummary").text.indexOf("owner@example.test") >= 0)
        const field = findChild(page, "settingsServerField")
        field.text = "https://changed.example.test"
        mouseClick(findChild(page, "saveServerButton"))
        compare(api.baseUrl, "https://changed.example.test")
        field.text = "invalid"
        compare(findChild(page, "saveServerButton").enabled, false)
        const checks = updater.checks
        mouseClick(findChild(page, "settingsCheckUpdates"))
        compare(updater.checks, checks + 1)
        updater.busy = true
        compare(findChild(page, "settingsCheckUpdates").enabled, false)
        updater.busy = false
        view.close()
        api.baseUrl = "https://inbox.example.test"
    }
    function test_zIdentitySurvivesPageReopen() {
        api.token = ""
        const page = createTemporaryObject(enrollmentComponent, proof, {api: api, width: 412, height: 800})
        verify(page)
        findChild(page, "inviteField").text = "fixture-invite"
        findChild(page, "deviceNameField").text = "Review phone"
        verify(page.enroll())
        compare(api.token, "new-token")
        compare(findChild(page, "inviteField").visible, false)
        const view = inbox(412)
        view.openPage("SettingsPage")
        const stack = findChild(view, "pageStack")
        tryCompare(stack, "busy", false)
        const summary = findChild(stack.currentItem, "settingsDeviceSummary")
        verify(summary.text.indexOf("Review phone") >= 0)
        verify(summary.text.indexOf("device-123") >= 0)
        verify(summary.text.indexOf("tenant-456") >= 0)
        shot(view, "after-enrolled-identity-412")
        view.close()
    }
}
