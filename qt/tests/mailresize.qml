import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    id: testCase
    name: "MailResize"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    property var store: resizeStore
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

    function cardId(index) { return "11111111-1111-4111-8111-" + String(index).padStart(12, "0") }
    function initTestCase() {
        const cards = []
        for (let i = 0; i < 100; ++i) {
            cards.push({id: cardId(i), title: "Message " + i, source: "mail", section: "now",
                account_name: "fixture@example.test", summary: "Read the cached message without archiving",
                has_body: true, pinned_rank: null, bundle_id: "", important: false, timed: false,
                note: "", source_url: "", state: "open"})
        }
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
    }
    function test_continuousResize_data() {
        return ["google", "nyt", "linkedin"].map(fixture => ({tag: fixture, fixture: fixture}))
    }
    function test_continuousResize(row) {
        store.applyRemoteMailBody(cardId(0), {html: mailFixtures.read(row.fixture)})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 1000, height: 879, title: "Mail resize " + row.fixture
        })
        verify(inbox)
        inbox.openPage("MailDetailPage", {store: store, cardId: cardId(0), cardTitle: row.fixture, openLinks: false})
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "busy", false)
        let body = null
        tryVerify(function() { body = findChild(stack.currentItem, "body"); return body !== null }, 5000)
        tryVerify(function() { return !body.loading && body.loadProgress === 100 }, 15000)
        wait(500)
        mailFixtures.startHeartbeat()
        console.log("RESIZE_READY", row.fixture)
        wait(10000)
        const blocked = mailFixtures.stopHeartbeat()
        console.log("RESIZE_RESULT", row.fixture, blocked.toFixed(3))
        console.log("RESIZE_BLOCKS", row.fixture, JSON.stringify(mailFixtures.heartbeatBlocks()))
        console.log("RESIZE_SCREENSHOT", row.fixture)
        wait(500)
        verify(blocked < 50, row.fixture + " GUI-thread gap " + blocked.toFixed(3) + " ms >= 50 ms")
        inbox.close()
    }
    function reportPhase(label) {
        const blocked = mailFixtures.stopHeartbeat()
        console.log("ACTION_RESULT", label, blocked.toFixed(3), JSON.stringify(mailFixtures.heartbeatBlocks()))
        return blocked
    }
    function test_mailOpenClose() {
        const rows = [
            {tag: "google", fixture: "google", index: 0},
            {tag: "nyt", fixture: "nyt", index: 1},
            {tag: "linkedin", fixture: "linkedin", index: 2},
            {tag: "mid-list", fixture: "google", index: 50}
        ]
        for (const row of rows)
            store.applyRemoteMailBody(cardId(row.index), {html: mailFixtures.read(row.fixture)})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 1000, height: 879, title: "Mail open/close"
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        const stack = findChild(inbox, "pageStack")
        const originalIds = store.cardIds()
        const failures = []
        wait(300)
        function cycle(row, label, cold, screenshot) {
            list.positionViewAtIndex(row.index, ListView.Beginning)
            wait(200)
            const before = list.contentY
            const preview = findChild(list.itemAtIndex(row.index), "openCard-" + cardId(row.index))
            verify(preview)
            mailFixtures.startHeartbeat()
            mouseClick(preview, preview.width / 2, preview.height / 2)
            tryCompare(stack, "depth", 2)
            tryCompare(stack, "busy", false)
            compare(stack.currentItem.cardId, cardId(row.index))
            compare(stack.currentItem.cardTitle, "Message " + row.index)
            compare(stack.currentItem.html, mailFixtures.read(row.fixture), "The selected message replaces the previous body")
            let body = null
            tryVerify(function() { body = findChild(stack.currentItem, "body"); return body !== null }, 5000)
            tryVerify(function() { return !body.loading && body.loadProgress === 100 }, 15000)
            wait(200)
            const opened = reportPhase("open-" + label)
            if (!cold && opened >= 100) failures.push("open-" + label + " " + opened.toFixed(3) + " ms")
            if (screenshot) {
                inbox.title = "Mail resize " + row.tag
                wait(100)
                console.log("RESIZE_SCREENSHOT", row.tag)
                wait(100)
            }
            mailFixtures.startHeartbeat()
            mouseClick(findChild(stack.currentItem, "backToInbox"))
            tryCompare(stack, "depth", 1)
            tryCompare(stack, "busy", false)
            wait(200)
            const closed = reportPhase("close-" + label)
            if (closed >= 50) failures.push("close-" + label + " " + closed.toFixed(3) + " ms")
            compare(list.contentY, before, "Closing restores the same scroll position")
            compare(list.itemAtIndex(row.index).cardId, cardId(row.index))
            compare(store.cardIds(), originalIds, "Opening/closing must not archive or reorder mail")
        }
        cycle(rows[0], "cold-google", true, false)
        for (let run = 1; run <= 3; ++run)
            for (const row of rows) cycle(row, row.tag + "-" + run, false, run === 1)
        inbox.close()
        verify(failures.length === 0, failures.join("; "))
    }

    function test_zBroaderInteractions() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 1000, height: 879, title: "Mail interactions"
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        const stack = findChild(inbox, "pageStack")
        wait(300)
        const originalIds = store.cardIds()
        mailFixtures.startHeartbeat()
        for (let i = 0; i < 40; ++i) {
            mouseWheel(list, list.width / 2, list.height / 2, 0, -120)
            wait(20)
        }
        verify(list.contentY > 0)
        reportPhase("scroll-inbox")
        list.positionViewAtBeginning()
        wait(200)
        for (const fixture of ["google", "nyt", "linkedin"]) {
            store.applyRemoteMailBody(cardId(0), {html: mailFixtures.read(fixture)})
            wait(100)
            mailFixtures.startHeartbeat()
            const preview = findChild(list.itemAtIndex(0), "openCard-" + cardId(0))
            verify(preview)
            mouseClick(preview, preview.width / 2, preview.height / 2)
            tryCompare(stack, "depth", 2)
            tryCompare(stack, "busy", false)
            let body = null
            tryVerify(function() { body = findChild(stack.currentItem, "body"); return body !== null }, 5000)
            tryVerify(function() { return !body.loading && body.loadProgress === 100 }, 15000)
            wait(200)
            reportPhase("open-" + fixture)
            mailFixtures.startHeartbeat()
            mouseClick(findChild(stack.currentItem, "backToInbox"))
            tryCompare(stack, "depth", 1)
            tryCompare(stack, "busy", false)
            wait(200)
            reportPhase("close-" + fixture)
            compare(store.cardIds(), originalIds, "Opening/closing must not archive or reorder mail")
        }
        mailFixtures.startHeartbeat()
        const archive = findChild(list.itemAtIndex(0), "doneButton-" + cardId(0))
        verify(archive)
        mouseClick(archive)
        tryVerify(function() { return store.cardIds().indexOf(cardId(0)) < 0 })
        wait(200)
        reportPhase("archive-mail-offline")
        mailFixtures.startHeartbeat()
        mouseClick(findChild(inbox, "addCardButton"))
        const dialog = findChild(inbox, "createDialog")
        tryCompare(dialog, "opened", true)
        wait(100)
        reportPhase("add-card-dialog")
        findChild(inbox, "createTitle").text = "Resize interaction proof"
        mailFixtures.startHeartbeat()
        mouseClick(findChild(inbox, "createSaveButton"))
        tryCompare(dialog, "opened", false)
        tryCompare(list, "count", originalIds.length)
        verify(store.cardIds()[0] !== cardId(0))
        tryVerify(function() { return list.itemAtIndex(0) && list.itemAtIndex(0).title === "Resize interaction proof" })
        compare(store.pendingOps, 2, "Both offline actions remain queued")
        wait(200)
        reportPhase("create-card-offline")
        inbox.close()
    }
}
