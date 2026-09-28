import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "MailDetailPage"
    width: 500
    height: 500
    Component { id: pageComponent; Pages.MailDetailPage {} }

    function test_loadAndGmailLink() {
        var called = ""
        var api = { get: function (path, cb) {
            called = path
            cb(null, {status: 200, body: {html: "<p>Hello <b>world</b></p>", threadId: "thread-123", accountId: "account-1"}})
        }}
        var page = createTemporaryObject(pageComponent, this, {api: api, cardId: "card-7", openLinks: false})
        verify(page)
        compare(called, "/v1/cards/card-7/body")
        compare(page.html, "<p>Hello <b>world</b></p>")
        compare(page.gmailUrl, "https://mail.google.com/mail/u/0/#all/thread-123")
        compare(page.errorText, "")
        compare(findChild(page, "openInGmail").text, qsTr("Open in Gmail"))
    }

    function test_errorDoesNotExposeStaleBody() {
        var api = { get: function (path, cb) { cb(new Error("not found"), {status: 404, body: null}) } }
        var page = createTemporaryObject(pageComponent, this, {api: api, cardId: "card-8"})
        verify(page)
        compare(page.html, "")
        compare(page.gmailUrl, "")
        verify(page.errorText.length > 0)
    }
}
