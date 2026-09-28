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
        var api = { post: function (path, body, cb) {
            call = {path: path, body: body}
            cb(null, {status: 201, body: {token: "bearer-123", device_id: "dev-1", tenant_id: "tenant-1"}})
        }}
        var page = createTemporaryObject(pageComponent, this, {api: api})
        verify(page)
        var emitted = null
        page.enrolled.connect(function (device) { emitted = device })
        findChild(page, "inviteField").text = " invite-code "
        findChild(page, "deviceNameField").text = " My phone "
        findChild(page, "platformBox").currentIndex = 1
        verify(page.enroll())
        compare(call.path, "/v1/devices/enroll")
        compare(call.body.invite_code, "invite-code")
        compare(call.body.device_name, "My phone")
        compare(call.body.platform, "android")
        compare(page.token, "bearer-123")
        compare(page.api.token, "bearer-123")
        compare(emitted.device_id, "dev-1")
    }

    function test_invalidInviteShowsError() {
        var api = { post: function (path, body, cb) { cb(new Error("invalid invite"), {status: 410, body: null}) } }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        findChild(page, "inviteField").text = "bad"
        findChild(page, "deviceNameField").text = "device"
        page.enroll()
        compare(page.token, "")
        verify(page.errorText.length > 0)
    }
}
