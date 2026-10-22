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

    function test_reloadEmptyListShowsNoError() {
        var api = {
            get: function (path, cb) { cb(null, {status: 200, body: []}) },
            post: function (path, body, cb) { },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        page.reload()
        compare(page.errorText, "")
        compare(page.accounts.length, 0)
    }

    function test_failedReloadDoesNotClaimNoAccounts() {
        var api = {
            get: function (path, cb) { cb(new Error("offline"), null) },
            post: function (path, body, cb) { },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        var emptyState = findChild(page, "emptyStateLabel")
        verify(emptyState)
        verify(!emptyState.visible)
        page.reload()
        compare(page.errorText, "Could not load the account list.")
        verify(!emptyState.visible)
    }

    function test_reloadListShowsAccounts() {
        var api = {
            get: function (path, cb) { cb(null, {status: 200, body: [{id: "x", address: "darafei@maumap.com"}]}) },
            post: function (path, body, cb) { },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        page.reload()
        compare(page.errorText, "")
        compare(page.accounts.length, 1)
        compare(page.accounts[0].address, "darafei@maumap.com")
    }

    function test_photoConsentBindsAccountAndClearsStaleUrl() {
        var posted = []
        var statusLoads = 0
        var api = {
            get: function (path, cb) {
                if (path.indexOf("sender-photos") >= 0) {
                    statusLoads++
                    cb(null, {status: 200, body: {accounts: [
                        {account_id: "acc-a", address: "a@example.org", authorization_required: true, required_scopes: ["contacts.readonly", "contacts.other.readonly"]},
                        {account_id: "acc-b", address: "b@example.org", photo_ready: true}
                    ]}})
                    return
                }
                cb(null, {status: 200, body: [{id: "acc-a", address: "a@example.org", status: "active"}]})
            },
            post: function (path, body, cb) {
                posted.push({path: path, body: body})
                cb(null, {status: 200, body: {authorization_url: "https://accounts.google.com/o/oauth2/auth?state=bound"}})
            },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        // The harness never shows the window; make the chain visible so the
        // visibility bindings below reflect their state, not hidden ancestors.
        this.visible = true
        // Component.onCompleted already loaded once; this is the return-from-
        // browser reload that must keep the status fresh.
        compare(statusLoads, 1)

        var button = findChild(page, "photoConsentButton")
        verify(button)
        button.clicked()
        compare(posted.length, 1)
        compare(posted[0].path, "/v1/gmail/photo-consent")
        compare(posted[0].body.account_id, "acc-a")
        compare(page.photoConsentUrl, "https://accounts.google.com/o/oauth2/auth?state=bound")
        compare(page.photoConsentAccountId, "acc-a")
        compare(page.photoConsentAddress, "a@example.org")
        var prompt = findChild(page, "photoConsentPrompt")
        verify(prompt.visible)
        verify(prompt.text.indexOf("a@example.org") >= 0)

        // Returning to the page drops the already-consumed URL and refreshes
        // the server-reported status instead of inviting a second click.
        page.reload()
        compare(page.photoConsentUrl, "")
        compare(page.photoConsentAccountId, "")
        compare(statusLoads, 2)
    }

    function test_photoConsentWrongAccountShowsErrorNotStaleLink() {
        var posted = []
        var api = {
            get: function (path, cb) {
                if (path.indexOf("sender-photos") >= 0) {
                    cb(null, {status: 200, body: {accounts: [
                        {account_id: "acc-a", address: "a@example.org", authorization_required: true, required_scopes: ["contacts.readonly", "contacts.other.readonly"]}
                    ]}})
                    return
                }
                cb(null, {status: 200, body: [{id: "acc-a", address: "a@example.org", status: "active"}]})
            },
            post: function (path, body, cb) {
                posted.push({path: path, body: body})
                cb(new Error("mismatch"), null)
            },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        this.visible = true
        findChild(page, "photoConsentButton").clicked()
        compare(posted.length, 1)
        compare(posted[0].body.account_id, "acc-a")
        compare(page.photoConsentUrl, "")
        verify(page.photoConsentError !== "")
        compare(findChild(page, "photoConsentError").visible, true)
    }

    // Settings refresh updates in place: while a refresh is in flight the
    // busy gear stays hidden and loaded rows/errors stay put; an unchanged
    // reply keeps the same delegates instead of recreating them.
    function test_refreshUpdatesInPlaceWithoutFlicker() {
        var pending = []
        var accountsFail = false
        var api = {
            get: function (path, cb) { pending.push({path: path, cb: cb}) },
            post: function (path, body, cb) { },
            del: function (path, cb) { }
        }
        function flush() {
            var calls = pending
            pending = []
            for (var i = 0; i < calls.length; ++i) {
                if (calls[i].path.indexOf("sender-photos") >= 0)
                    calls[i].cb(new Error("status down"), null)
                else if (accountsFail)
                    calls[i].cb(new Error("accounts down"), null)
                else
                    calls[i].cb(null, {status: 200, body: [{id: "acc-a", address: "a@example.org", status: "active"}]})
            }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        this.visible = true
        var busy = findChild(page, "accountsBusy")
        var list = findChild(page, "accountsList")
        verify(busy.visible, "first load shows the spinner")
        flush()
        verify(!busy.visible)
        tryCompare(list, "count", 1)
        var row = list.itemAtIndex(0)
        verify(row)
        var accountsBefore = page.accounts
        var photoError = findChild(page, "photoStatusError")
        verify(photoError.visible)

        page.reload()
        verify(!busy.visible, "refresh never shows the busy gear")
        verify(photoError.visible, "photo error stays while refresh is in flight")
        compare(list.count, 1)
        flush()
        verify(page.accounts === accountsBefore, "unchanged reply keeps the model array")
        verify(list.itemAtIndex(0) === row, "unchanged reply keeps the delegate")
        verify(photoError.visible)

        accountsFail = true
        page.reload()
        flush()
        compare(page.errorText, "Could not load the account list.")
        page.reload()
        compare(page.errorText, "Could not load the account list.", "error stays while refresh is in flight")
        verify(!busy.visible)
        compare(list.count, 1)
    }

    // A failed accounts reply (e.g. HTTP 429) ends the 2 s connect poll
    // instead of re-requesting forever against a rate-limited token.
    function test_failedReloadStopsConnectPoll() {
        var gets = 0
        var api = {
            get: function (path, cb) { gets++; cb(new Error("429 rate limit exceeded"), null) },
            post: function (path, body, cb) { cb(null, {status: 200, body: {authorization_url: "https://accounts.google.com/o/oauth2/auth?state=t"}}) },
            del: function (path, cb) { }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api, openLinks: false})
        verify(page)
        findChild(page, "connectButton").clicked()
        compare(page.connecting, false)
        var before = gets
        wait(2500)
        compare(gets, before, "no poll requests after a failed reply")
    }
}
