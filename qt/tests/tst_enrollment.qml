import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "EnrollmentPage"
    width: 500
    height: 500
    Component { id: pageComponent; Pages.EnrollmentPage {} }

    function test_enrollAndEmitToken() {
        var call = null
        var api = { baseUrl: "http://compiled-default:8081", token: "", post: function (path, body, cb) {
            call = {path: path, body: body}
            cb(null, {status: 201, body: {token: "bearer-123", device_id: "dev-1", tenant_id: "tenant-1"}})
        }}
        var page = createTemporaryObject(pageComponent, this, {api: api})
        verify(page)
        // The Server field is prefilled with the current api base URL.
        compare(findChild(page, "serverField").text, "http://compiled-default:8081")
        var emitted = null
        page.enrolled.connect(function (device) { emitted = device })
        findChild(page, "serverField").text = " http://enrolled-server:8081 "
        findChild(page, "inviteField").text = " invite-code "
        findChild(page, "deviceNameField").text = " My phone "
        findChild(page, "platformBox").currentIndex = 1
        verify(page.enroll())
        compare(call.path, "/v1/devices/enroll")
        compare(call.body.invite_code, "invite-code")
        compare(call.body.device_name, "My phone")
        compare(call.body.platform, "android")
        // Enrolling points the api at the typed server (main.cpp persists
        // baseUrl to QSettings server_url) and installs the token (main.cpp
        // persists token and takes the app online), so no hand copy is needed.
        compare(page.api.baseUrl, "http://enrolled-server:8081")
        compare(page.token, "bearer-123")
        compare(page.api.token, "bearer-123")
        compare(emitted.device_id, "dev-1")
    }


    function test_enrollButtonIsVisibleAndSized() {
        const page = createTemporaryObject(pageComponent, this, {
            api: { baseUrl: "http://server:8081", post: function (path, body, cb) {} }
        })
        page.visible = true
        page.width = width
        page.height = height
        verify(page)
        const button = findChild(page, "enrollButton")
        verify(button, "missing enrollment button")
        verify(button.width >= 48, "enrollment button must have a non-zero usable width")
        verify(button.height >= 48, "enrollment button must meet the 48px touch target")
        verify(Qt.colorEqual(button.background.color, "#397d73"),
               "enrollment button background must be visible at rest")
    }
    function test_enrollmentInputsUseLightPalette() {
        var api = { baseUrl: "http://server:8081", post: function (path, body, cb) {} }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        verify(page)
        for (const name of ["serverField", "inviteField", "deviceNameField", "platformBox"]) {
            const control = findChild(page, name)
            verify(control, "missing enrollment control " + name)
            verify(Qt.colorEqual(control.palette.base, "#ffffff"), name + " does not use a white base")
            verify(Qt.colorEqual(control.palette.text, "#263b3a"), name + " does not use ink text")
        }
    }

    function test_emptyServerIsRequired() {
        var posted = false
        var api = { baseUrl: "", token: "", post: function (path, body, cb) { posted = true } }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        findChild(page, "inviteField").text = "invite-code"
        findChild(page, "deviceNameField").text = "device"
        verify(!page.enroll())
        verify(!posted)
        compare(page.token, "")
        verify(page.errorText.length > 0)
    }

    function test_invalidInviteShowsError() {
        var api = { baseUrl: "http://server:8081", post: function (path, body, cb) { cb(new Error("invalid invite"), {status: 410, body: null}) } }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        findChild(page, "inviteField").text = "bad"
        findChild(page, "deviceNameField").text = "device"
        page.enroll()
        compare(page.token, "")
        compare(page.errorText, "Invite is used, expired, or invalid.")
    }

    function test_networkErrorShowsServerAddress() {
        var api = { baseUrl: "", post: function (path, body, cb) { cb(new Error("connection refused"), null) } }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        findChild(page, "serverField").text = "http://wrong-address:8081"
        findChild(page, "inviteField").text = "invite-code"
        findChild(page, "deviceNameField").text = "device"
        page.enroll()
        compare(page.errorText, "Can't reach the server at http://wrong-address:8081. Check the address and that you're on the home Wi-Fi.")
    }
}
