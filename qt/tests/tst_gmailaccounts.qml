import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "GmailAccountsPage"
    width: 500
    height: 500
    Component { id: pageComponent; Pages.GmailAccountsPage {} }

    function test_listConnectDisconnect() {
        var calls = []
        var accounts = [{id: "acc-1", address: "person@example.com"}]
        var api = {
            get: function (path, cb) { calls.push(path); cb(null, {status: 200, body: accounts}) },
            post: function (path, body, cb) { calls.push(path); cb(null, {status: 200, body: {url: "https://accounts.google.com/authorize"}}) },
            del: function (path, cb) { calls.push(path); accounts = []; cb(null, {status: 204, body: null}) }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        compare(calls[0], "/v1/gmail/accounts")
        compare(page.accounts[0].address, "person@example.com")
        page.connectAccount()
        compare(calls[1], "/v1/gmail/connect")
        compare(page.lastConnectUrl, "https://accounts.google.com/authorize")
        page.disconnect("acc-1")
        compare(calls[2], "/v1/gmail/accounts/acc-1")
        compare(calls[3], "/v1/gmail/accounts")
        compare(page.accounts.length, 0)
    }

    function test_disconnectErrorPreservesAccount() {
        var api = {
            get: function (path, cb) { cb(null, {status: 200, body: [{id: "acc-2", address: "other@example.com"}]}) },
            del: function (path, cb) { cb(new Error("disconnect failed"), {status: 500, body: null}) }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        page.disconnect("acc-2")
        compare(page.accounts.length, 1)
        verify(page.errorText.length > 0)
    }
}
