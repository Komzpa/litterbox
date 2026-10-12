import QtQuick
import QtQuick.Controls
import QtTest
import litterbox 1.0 as App

TestCase {
    id: proof
    name: "UiPolish"
    when: windowShown
    property var store: resizeStore
    property url pagesDir: Qt.resolvedUrl("../pages/")
    QtObject {
        id: api
        property string baseUrl: "https://inbox.example.test"
        property string token: ""
        property var accounts: [{id: "account", address: "owner@example.test", status: "reauth_required"}, {id: "other", address: "other@example.test", status: "active"}]
        function get(path, callback) { callback(null, {body: accounts}) }
        function post(path, body, callback) { callback(null, {body: {authorization_url: "https://accounts.example.test/connect"}}) }
    }
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
    function id(n) { return "11111111-1111-4111-8111-" + String(n).padStart(12, "0") }
    function seed() {
        const cards = []
        for (let i = 0; i < 5; ++i) {
            cards.push({id: id(i), title: i ? "An update on geospatial research and development" : "Your weekly research digest",
                source: "mail", section: "now", account_name: "owner@example.test",
                sender_name: i ? "LinkedIn" : "Research team", sender_address: "news@example.test",
                received_at: "2026-10-09T14:33:00Z", snippet: "The latest projects and conversations for Darafei Praliaskouski, including maps, new tools and thoughtful updates from your network.",
                has_body: true, pinned_rank: null, bundle_id: i ? "digest" : "",
                bundle_title: i ? "Geospatial research" : "", important: false, timed: false,
                note: "", source_url: "", state: "open"})
        }
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
        store.applyRemoteMailBody(id(0), {html: mailFixtures.read("plain")})
        return cards
    }
    function shot(inbox, phase, screen, width) {
        wait(180)
        grabImage(inbox.contentItem.parent).save(mailFixtures.screenshotPath(phase + "/" + screen + "-" + width + ".png"))
    }
    function capture(phase) {
        for (const width of [1440, 900]) {
            const cards = seed()
            const inbox = createTemporaryObject(inboxComponent, proof, {
                store: store, api: api, updater: updater, timeRules: timeRules, width: width, height: 1000
            })
            verify(inbox)
            inbox.show()
            const list = findChild(inbox, "inboxList")
            tryVerify(function() { list.forceLayout(); return list.itemAtIndex(1) !== null })
            shot(inbox, phase, "inbox", width)
            const accounts = findChild(inbox, "headerMenuAccounts")
            accounts.menu.open()
            tryCompare(accounts.menu, "opened", true)
            accounts.menu.currentIndex = 0
            shot(inbox, phase, "overflow", width)
            accounts.menu.close()
            inbox.toggleBundle("digest")
            tryVerify(function() { list.forceLayout(); return list.itemAtIndex(2) && list.itemAtIndex(2).visible })
            shot(inbox, phase, "bundle-expanded", width)
            inbox.openPage("MailDetailPage", {store: store, cardId: id(0), cardTitle: cards[0].title,
                accountName: cards[0].account_name, card: cards[0], openLinks: false})
            const stack = findChild(inbox, "pageStack")
            tryCompare(stack, "busy", false)
            tryVerify(function() { const body = findChild(stack.currentItem, "body"); return body && !body.loading && body.loadProgress === 100 }, 15000)
            shot(inbox, phase, "mail", width)
            stack.pop()
            tryCompare(stack, "busy", false)
            inbox.showOperationFailure("", 409, {code: "gmail_reconnect_required", account_address: "owner@example.test"})
            inbox.openPage("GmailAccountsPage", {api: api, openLinks: false})
            tryCompare(stack, "busy", false)
            shot(inbox, phase, "accounts", width)
            stack.pop()
            tryCompare(stack, "busy", false)
            inbox.openPage("EnrollmentPage", {api: api})
            tryCompare(stack, "busy", false)
            shot(inbox, phase, "settings", width)
            stack.pop()
            tryCompare(stack, "busy", false)
            shot(inbox, phase, "offline", width)
            store.applyRemoteCards({now: [], later: [], missed: []})
            tryCompare(list, "count", 0)
            shot(inbox, phase, "empty", width)
            inbox.close()
            inbox.destroy()
            wait(50)
        }
    }
    function test_before() { capture("before") }
    function test_after() { capture("after") }
    function test_reconnectLifecycle_data() { return [{tag: "1440", width: 1440}, {tag: "900", width: 900}] }
    function test_reconnectLifecycle(data) {
        seed()
        api.accounts = [{id: "account", address: "owner@example.test", status: "reauth_required"}]
        const inbox = createTemporaryObject(inboxComponent, proof, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: data.width, height: 1000
        })
        verify(inbox)
        inbox.show()
        const bar = findChild(inbox, "bundleArchiveUndoBar")
        tryCompare(bar, "visible", true)
        shot(inbox, "banner", "inbox-expired", data.width)
        inbox.openPage("GmailAccountsPage", {api: api, openLinks: false})
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "busy", false)
        const page = stack.currentItem
        const list = findChild(page, "accountsList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const reconnect = findChild(list.itemAtIndex(0), "reconnectButton")
        verify(reconnect.visible)
        shot(inbox, "banner", "accounts-expired", data.width)
        page.connectAccount()
        api.accounts = [{id: "account", address: "owner@example.test", status: "active"}]
        // The connect-flow timer obtains the callback's new account state.
        tryCompare(bar, "visible", false, 5000)
        tryVerify(function() { list.forceLayout(); return !findChild(list.itemAtIndex(0), "reconnectButton").visible })
        shot(inbox, "banner", "accounts-active", data.width)
        stack.pop()
        tryCompare(stack, "busy", false)
        inbox.showOperationFailure("stale", 422, {code: "gmail_reconnect_required", account_address: "owner@example.test"})
        verify(!bar.visible, "Stale reconnect errors must not override active status")
        shot(inbox, "banner", "inbox-active", data.width)
        api.accounts = [{id: "account", address: "owner@example.test"}]
        inbox.showOperationFailure("legacy", 422, {code: "gmail_reconnect_required", account_address: "owner@example.test"})
        verify(!bar.visible, "Old servers without status must not show stale banners")
        inbox.close()
    }
    function test_geometry_data() { return [{tag: "1440", width: 1440}, {tag: "900", width: 900}, {tag: "572", width: 572}] }
    function test_geometry(data) {
        seed()
        const inbox = createTemporaryObject(inboxComponent, proof, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: data.width, height: 1000
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(1) !== null })
        const accounts = findChild(inbox, "headerMenuAccounts")
        const menu = accounts.menu
        menu.open()
        tryCompare(menu, "opened", true)
        wait(100)
        const refresh = findChild(inbox, "headerMenuRefresh")
        compare(menu.height, menu.topPadding + menu.bottomPadding + accounts.height + findChild(inbox, "headerMenuEnroll").height + refresh.height,
                "Unsupported updates must leave no empty menu tail")
        compare(accounts.icon.width, 20)
        compare(accounts.icon.height, 20)
        verify(accounts.background.x >= 0 && menu.padding >= 4, "Highlight stays inside padded menu")
        menu.close()
        const row = list.itemAtIndex(0)
        const bundle = list.itemAtIndex(1)
        const primary = findChild(row, "doneButton-" + id(0))
        const archive = findChild(bundle, "archiveBundle-" + id(1))
        compare(primary.icon.width, archive.icon.width)
        compare(primary.icon.height, archive.icon.height)
        verify(primary.width >= 48 && primary.height >= 48)
        const sender = findChild(row, "mailSender-" + id(0))
        const preview = findChild(row, "mailSnippet-" + id(0))
        verify(sender.width > 0 && preview.width > 100)
        compare(preview.maximumLineCount, 1)
        compare(preview.elide, Text.ElideRight)
        verify(preview.mapToItem(list, 0, 0).x >= inbox.edgeSpacing)
        const frame = findChild(row, "inboxCard")
        verify(frame.x >= 18 && frame.width <= 1200)
        inbox.openPage("GmailAccountsPage", {api: api, openLinks: false})
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "busy", false)
        const column = findChild(stack.currentItem, "accountsColumn")
        verify(column.width <= 720, "Accounts must not stretch across the window")
        inbox.close()
    }
}
