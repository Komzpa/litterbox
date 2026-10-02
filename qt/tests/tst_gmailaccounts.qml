import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "GmailAccountsPage"
    width: 500
    height: 500
    Component { id: pageComponent; Pages.GmailAccountsPage {} }

    function test_connectUsesAuthorizationUrl() {
        var postedPath = ""
        var api = {
            get: function (path, cb) { cb(null, {status: 200, body: []}) },
            post: function (path, body, cb) {
                postedPath = path
                cb(null, {status: 200, body: {authorization_url: "https://accounts.google.com/o/oauth2/auth?state=test"}})
            },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)

        findChild(page, "connectButton").clicked()

        compare(postedPath, "/v1/gmail/connect")
        compare(page.lastConnectUrl, "https://accounts.google.com/o/oauth2/auth?state=test")
        compare(page.errorText, "")
    }
}
