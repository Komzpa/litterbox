import QtQuick
import QtTest
import litterbox 1.0 as App

// Owner mobile UX acceptance fixture (REQUIREMENTS R55-R69) at the owner's
// 549x1280 phone screenshot size. capture_* functions only write PNGs into
// the working directory (mobileux-*.png) so the same file can render an
// older checkout for a before/after comparison; test_* functions assert.
TestCase {
    id: testCase
    name: "MobileUx"
    when: windowShown
    width: 549
    height: 1280
    property url pagesDir: Qt.resolvedUrl("../pages/")

    readonly property string polisPreheader: "\u200c\u034f\ufeff\ud83d\udee1\ufe0f Туристическая страховка для поездки в Европу"

    ListModel {
        id: store
        property bool online: true
        property string offlineReason: ""
        property int pendingOps: 0
        property bool saveFailed: false
        property var failedWrites: []
        property int mailSyncPending: 0
        property bool mailSyncComplete: true
        property int refreshCalls: 0
        signal mailSyncStateChanged()
        signal operationFailed(string opId, int status, var response)
        signal storageError(string message)
        function retainedWriteFailed(opId) { return failedWrites.indexOf(opId) >= 0 }
        property bool failLocalSave: true
        // Failed local persistence must leave the production create dialog intact.
        function createCard(title, summary, source) {
            if (failLocalSave) {
                storageError("fixture durable storage unavailable")
                return ""
            }
            return "fixture-created-card"
        }
        property var savedNote: ({})
        function saveNote(cardId, text) {
            if (failLocalSave) {
                storageError("fixture durable storage unavailable")
                return false
            }
            savedNote = {cardId: cardId, text: text}
            return true
        }
        property var cached: ({})
        property var operations: []
        function enqueueOp(cardId, operation, args) {
            operations = operations.concat([{cardId: cardId, type: operation, args: args}])
            return "fixture-operation"
        }
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cardIds() { const ids = []; for (let i = 0; i < count; ++i) ids.push(get(i).cardId); return ids }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function refresh() { ++refreshCalls }
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) {}
        function openCachedFile(cardId, link) { return "" }
    }
    QtObject { id: api; property string baseUrl: ""; property string token: "token" }
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

    function mail(id, sender, title, snippet, at, extra) {
        const card = Object.assign({
            id: id, title: title, source: "mail", section: "now", bundle_id: "", pinned_rank: null,
            important: false, timed: false, has_body: true, summary: "", snippet: snippet, note: "",
            account_name: "darafei@maumap.com", sender_name: sender, sender_address: "",
            received_at: at
        }, extra || {})
        return {cardId: id, title: title, section: "now", card: card}
    }
    function agent(id, title, summary, extra) {
        const card = Object.assign({
            id: id, title: title, source: "agent", section: "now", bundle_id: "", pinned_rank: null,
            important: false, timed: false, has_body: false, summary: summary, snippet: "", note: "",
            account_name: "", sender_name: "", received_at: "2026-10-15T09:10:00Z"
        }, extra || {})
        return {cardId: id, title: title, section: "now", card: card}
    }
    function seed() {
        store.clear()
        store.online = true
        store.offlineReason = ""
        store.pendingOps = 0
        store.operations = []
        store.saveFailed = false
        store.failedWrites = []
        store.mailSyncPending = 0
        store.mailSyncComplete = true
        store.refreshCalls = 0
        store.failLocalSave = true
        store.savedNote = ({})
        const rows = []
        rows.push(mail("polis-1", "Polis.ge", "Туристическая страховка: скидка 20% до конца октября", testCase.polisPreheader, "2026-10-15T10:20:00Z"))
        // PostGIS bot-review bundle of 4 (leader first, members hidden until expanded).
        const pg4 = {bundle_id: "gh-postgis-4", bundle_title: "postgis/postgis · bot reviews", bundle_member_count: 4}
        rows.push(mail("pg4-a", "coderabbitai[bot]", "Re: [postgis/postgis] MVT fast path (PR #812)", "coderabbitai[bot] left a comment on this pull request.", "2026-10-15T03:17:00Z", Object.assign({bundle_leader: true}, pg4)))
        rows.push(mail("pg4-b", "Copilot", "Re: [postgis/postgis] MVT fast path (PR #812)", "@Copilot commented on this pull request.", "2026-10-15T02:33:00Z", Object.assign({bundle_leader: false}, pg4)))
        rows.push(mail("pg4-c", "chatgpt-codex-connector[bot]", "Re: [postgis/postgis] MVT fast path (PR #812)", "Codex review: 2 suggestions.", "2026-10-15T02:10:00Z", Object.assign({bundle_leader: false}, pg4)))
        rows.push(mail("pg4-d", "coderabbitai[bot]", "Re: [postgis/postgis] ST_AsMVT clipping (PR #809)", "Actionable comments posted: 1", "2026-10-15T01:40:00Z", Object.assign({bundle_leader: false}, pg4)))
        rows.push(agent("agent-mvt", "PostGIS MVT fast path needs correctness review — GH-1182/Trac 6145", "What happened: a review on the MVT fast path found a correctness concern.\n\nNext step: review GH-1182 before merging."))
        const ha2 = {bundle_id: "gh-ha-2", bundle_title: "home-assistant/core · bot reviews", bundle_member_count: 2}
        rows.push(mail("ha2-a", "coderabbitai[bot]", "Re: [home-assistant/core] Gree native climate (PR #15520)", "", "2026-10-15T02:01:00Z", Object.assign({bundle_leader: true}, ha2)))
        rows.push(mail("ha2-b", "Copilot", "Re: [home-assistant/core] Gree native climate (PR #15520)", "@Copilot commented on this pull request.", "2026-10-15T01:20:00Z", Object.assign({bundle_leader: false}, ha2)))
        const singles = [
            ["Regina Obe", "[postgis-devel] PostGIS Day: proposals close October 15", "Reminder: the call for proposals closes today."],
            ["GitHub", "[OSGeo/gdal] Release 3.12.0 is out", "GDAL 3.12.0 released with new drivers and fixes."],
            ["TBC Bank", "Выписка по счёту за сентябрь", "Ваша выписка готова. Откройте приложение, чтобы посмотреть."],
            ["Mapbox", "Your October usage report", "You used 41% of your monthly tile requests."],
            ["Wolt", "Ваш заказ доставлен", "Спасибо, что выбрали Wolt. Оцените заказ."],
            ["OpenStreetMap", "[OSM-talk] Import guidelines update", "Proposal to update the import guidelines for building footprints."],
            ["Hetzner", "Invoice 0123456789", "Your invoice for October is available in the console."],
            ["Booking.com", "Подтверждение бронирования в Батуми", "Номер 2 гостя, 17–19 октября. Бесплатная отмена."],
            ["Google", "Security alert for darafei@maumap.com", "A new sign-in on Android was detected."],
            ["Kartoza", "QGIS training next week", "Seats are still available for the advanced course."],
            ["Magti", "Счёт за услуги связи", "Сумма к оплате 35 GEL до 25 октября."],
            ["Notion", "Weekly digest", "3 pages were updated in your workspace this week."]
        ]
        for (let i = 0; i < singles.length; ++i)
            rows.push(mail("single-" + i, singles[i][0], singles[i][1], singles[i][2], "2026-10-15T0" + (i % 9) + ":0" + (i % 6) + ":00Z"))
        rows.push(agent("agent-obligations", "Open obligations — audit 14 Oct 2026, 86 obligations", "What happened: the nightly audit found 86 open obligations."))
        for (const row of rows) store.append(row)
    }
    function openInbox(online) {
        seed()
        if (!online) {
            store.online = false
            store.offlineReason = "Software caused connection abort"
            store.pendingOps = 2
        }
        const inbox = createTemporaryObject(inboxComponent, testCase, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 549, height: 1280
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(4) !== null })
        wait(200)
        return {inbox: inbox, list: list}
    }
    function save(inbox, name) {
        const image = grabImage(inbox.contentItem.parent)
        image.save("mobileux-" + name + ".png")
    }
    function textOf(item) {
        let out = item.text !== undefined && typeof item.text === "string" && item.visible ? item.text + "\n" : ""
        for (const child of item.children) out += textOf(child)
        return out
    }
    function fullyVisibleSingleMails(list) {
        let n = 0
        for (let i = 0; i < list.count; ++i) {
            const item = list.itemAtIndex(i)
            if (!item || !item.visible || item.height <= 0) continue
            if (item.card.source !== "mail" || item.card.bundle_id) continue
            const y = item.mapToItem(list, 0, 0).y
            if (y >= 0 && y + item.height <= list.height) ++n
        }
        return n
    }

    function test_captureInboxOnline() {
        const shown = openInbox(true)
        save(shown.inbox, "inbox-online")
        const density = fullyVisibleSingleMails(shown.list)
        console.log("MOBILEUX density single mails per screen:", density)
        // R63 oracle: at least 10 single mails per 549x1280 phone screen.
        verify(density >= 10, "single mails per screen: " + density)
        let polisRow = null
        for (let i = 0; i < shown.list.count; ++i) {
            const item = shown.list.itemAtIndex(i)
            if (item && item.cardId === "polis-1") polisRow = item
        }
        verify(polisRow)
        // R63: previews drop leading invisible characters and emoji.
        compare(findChild(polisRow, "mailSnippet-polis-1").text, "Туристическая страховка для поездки в Европу")
        // Time shares the subject line; archive and overflow stay at the right edge.
        const subject = findChild(polisRow, "mailTitle-polis-1")
        const time = findChild(polisRow, "mailDate-polis-1")
        verify(time.visible)
        fuzzyCompare(time.mapToItem(polisRow, 0, 0).y, subject.mapToItem(polisRow, 0, 0).y, 6)
        const archive = findChild(polisRow, "doneButton-polis-1")
        verify(archive.visible)
        verify(archive.mapToItem(polisRow, 0, 0).x + archive.width > polisRow.width - 64)
        const more = findChild(polisRow, "inboxActions")
        verify(more.visible)
        // One row: the narrow overflow sits beside the archive, not below it.
        fuzzyCompare(more.mapToItem(polisRow, 0, 0).y, archive.mapToItem(polisRow, 0, 0).y, 1)
    }
    function test_overflowDismissalKeepsListAnchor_data() {
        const rows = []
        for (const size of [[412, 915], [480, 915], [549, 1280]])
            for (const scale of [1, 1.5])
                rows.push({tag: size[0] + "x" + size[1] + "-text" + scale,
                    w: size[0], h: size[1], scale: scale})
        return rows
    }
    function test_overflowDismissalKeepsListAnchor(data) {
        const shown = openInbox(true)
        const inbox = shown.inbox
        inbox.width = data.w
        inbox.height = data.h
        shown.list.positionViewAtIndex(8, ListView.Beginning)
        wait(180)
        const before = shown.list.contentY
        const selected = inbox.selectedCardId
        const stack = findChild(inbox, "pageStack")
        const opener = findChild(inbox, "inboxCommands")
        const menu = findChild(inbox, "headerMenu")
        mouseClick(opener)
        tryCompare(menu, "visible", true)
        for (let i = 0; i < menu.count; ++i) {
            const row = menu.itemAt(i)
            row.font = Qt.font({family: inbox.themeFont.family, pointSize: inbox.themeFont.pointSize * data.scale})
        }
        wait(180)
        const actions = findChild(menu, "headerMenuActions")
        mouseWheel(actions, actions.width / 2, actions.height / 2, 0, -480)
        wait(180)
        fuzzyCompare(shown.list.contentY, before, 1, "popup scrolling must not move the inbox")
        save(inbox, "overflow-" + data.tag)
        // Window-coordinate tap outside the popup; do not emit close directly.
        const outside = menu.parent.mapToItem(inbox.contentItem, 1, menu.parent.height / 2)
        mouseClick(inbox.contentItem, outside.x, outside.y)
        tryCompare(menu, "visible", false)
        fuzzyCompare(shown.list.contentY, before, 1)
        mouseClick(opener)
        tryCompare(menu, "visible", true)
        verify(inbox.handleBack())
        tryCompare(menu, "visible", false)
        fuzzyCompare(shown.list.contentY, before, 1)
        compare(inbox.selectedCardId, selected)
        compare(stack.depth, 1)
        compare(store.operations.length, 0, "menu scrolling and dismissal must not triage mail")
        inbox.close()
    }

    function test_phoneDrawerSearchClearAndTouchTargets() {
        const shown = openInbox(true)
        const inbox = shown.inbox
        compare(inbox.desktopNavigation, false)
        compare(inbox.desktopReader, false)
        const drawerButton = findChild(inbox, "openNavigation")
        verify(drawerButton && drawerButton.visible && drawerButton.width >= 48 && drawerButton.height >= 48)
        mouseClick(drawerButton)
        const drawer = findChild(inbox, "navigationDrawer")
        tryVerify(function() { return drawer.visible })
        for (const name of ["navInbox", "navAccounts", "navAgents", "navSettings"]) {
            const nav = findChild(drawer, name)
            verify(nav && nav.visible && nav.width >= 48 && nav.height >= 48, name + " must be a reachable labeled destination")
            verify(nav.Accessible.name.length > 0)
        }
        // Dispatch through the overlay: a scrim tap dismisses without navigating.
        const stack = findChild(inbox, "pageStack")
        const selectedBefore = inbox.selectedCardId
        const queryBefore = inbox.searchQuery
        const positionBefore = shown.list.contentY
        mouseClick(inbox.contentItem, drawer.width + (inbox.width - drawer.width) / 2, inbox.height / 2)
        tryCompare(drawer, "visible", false)
        compare(stack.depth, 1)
        mouseClick(drawerButton)
        tryCompare(drawer, "visible", true)
        verify(inbox.handleBack())
        tryCompare(drawer, "visible", false)
        compare(stack.depth, 1)
        compare(inbox.selectedCardId, selectedBefore)
        compare(inbox.searchQuery, queryBefore)
        fuzzyCompare(shown.list.contentY, positionBefore, 1)
        // R70: "tapping the scrim, Android Back or a closing swipe dismisses it".
        // Start inside the drawer so its ScrollView must yield the close drag.
        mouseClick(drawerButton)
        tryCompare(drawer, "opened", true)
        const startX = drawer.width * 0.7
        const endX = 10
        const swipeY = inbox.height / 2
        const touch = touchEvent(inbox.contentItem)
        touch.press(0, inbox.contentItem, startX, swipeY).commit()
        for (let i = 1; i <= 30; ++i) {
            wait(20)
            touch.move(0, inbox.contentItem, startX + (endX - startX) * i / 30, swipeY).commit()
        }
        touch.release(0, inbox.contentItem, endX, swipeY).commit()
        tryCompare(drawer, "visible", false)
        compare(stack.depth, 1)
        compare(inbox.selectedCardId, selectedBefore)
        compare(inbox.searchQuery, queryBefore)
        fuzzyCompare(shown.list.contentY, positionBefore, 1)
        // Device threshold measured at 80 px under both 64 and 96 margins, so 80
        // is a platform floor rather than something the margin tunes; margin set
        // to 80 so code and test agree with the measurement.
        verify(!drawer.visible)
        const edge = touchEvent(inbox.contentItem)
        edge.press(0, inbox.contentItem, 80, swipeY).commit()
        for (let i = 1; i <= 10; ++i) {
            wait(16)
            edge.move(0, inbox.contentItem, 80 + (drawer.width + 60 - 80) * i / 10, swipeY).commit()
        }
        edge.release(0, inbox.contentItem, drawer.width + 60, swipeY).commit()
        tryCompare(drawer, "visible", true)
        // A 300 ms closing fling still dismisses the drawer.
        const fling = touchEvent(inbox.contentItem)
        const flingStartX = drawer.width * 0.7
        fling.press(0, inbox.contentItem, flingStartX, swipeY).commit()
        for (let i = 1; i <= 15; ++i) {
            wait(20)
            fling.move(0, inbox.contentItem, flingStartX + (10 - flingStartX) * i / 15, swipeY).commit()
        }
        fling.release(0, inbox.contentItem, 10, swipeY).commit()
        tryCompare(drawer, "visible", false)
        const destinations = [
            {name: "navAccounts", page: "gmailAccountsPage"},
            {name: "navAgents", page: "agentThreadsPage"},
            {name: "navSettings", page: "settingsPage"}
        ]
        for (const destination of destinations) {
            mouseClick(drawerButton)
            tryCompare(drawer, "visible", true)
            mouseClick(findChild(drawer, destination.name))
            tryVerify(function() { return stack.depth === 2 && !drawer.visible })
            compare(stack.currentItem.objectName, destination.page)
            // Inbox is a real destination, not merely dismissal of the drawer.
            drawer.open()
            tryCompare(drawer, "visible", true)
            verify(findChild(drawer, destination.name).checked)
            mouseClick(findChild(drawer, "navInbox"))
            tryCompare(drawer, "visible", false)
            tryCompare(stack, "depth", 1)
        }
        compare(store.operations.length, 0, "navigation and dismissal must not triage cards")
        const toggle = findChild(inbox, "toggleSearch")
        verify(toggle && toggle.width >= 48 && toggle.height >= 48)
        mouseClick(toggle)
        const search = findChild(inbox, "inboxSearch")
        verify(search && search.visible && search.height >= 48)
        search.text = "Home Assistant synthetic"
        search.textEdited(search.text)
        tryVerify(function() { shown.list.forceLayout(); return shown.list.count === 0 })
        inbox.handleBack()
        tryVerify(function() { return inbox.searchQuery === "" && shown.list.count === store.count })
        compare(store.cardIds().length, store.count)
        const first = shown.list.itemAtIndex(0)
        const archive = findChild(first, "doneButton-polis-1")
        const overflow = findChild(first, "inboxActions")
        verify(archive.width >= 48 && archive.height >= 48)
        verify(overflow.height >= 48 && overflow.width >= 28)
        compare(store.operations.length, 0)
        inbox.close()
    }
    // Android AX reports a box one device px short of its painted size, so the pinned floor there
    // is ceil(48 dp) + 1 device px (133 px at 440 dpi, 97 px at 320 dpi); elsewhere ceil(48 dp).
    function minTapDevicePx() {
        return Math.ceil(48 * Screen.devicePixelRatio) + (Qt.platform.os === "android" ? 1 : 0)
    }
    function minTapItemPx() { return minTapDevicePx() / Screen.devicePixelRatio }
    function verifyTapTarget(item, what) {
        verify(item && item.visible, what + " must exist and be visible")
        const dpr = Screen.devicePixelRatio
        verify(Math.round(item.width * dpr) >= minTapDevicePx() && Math.round(item.height * dpr) >= minTapDevicePx(),
               what + " is " + item.width + "x" + item.height + " item px at dpr " + dpr + "; need >= 48 dp")
    }
    function test_tapTargetsReachFortyEightDpAtAnyDensity() {
        const shown = openInbox(true)
        const inbox = shown.inbox
        compare(inbox.tapTarget, minTapItemPx())
        verify(inbox.tapTarget * Screen.devicePixelRatio >= 48 * Screen.devicePixelRatio - 1e-6)
        verifyTapTarget(findChild(inbox, "openNavigation"), "openNavigation")
        verifyTapTarget(findChild(inbox, "toggleSearch"), "toggleSearch")
        verifyTapTarget(findChild(inbox, "addCardButton"), "addCardButton")
        mouseClick(findChild(inbox, "openNavigation"))
        const drawer = findChild(inbox, "navigationDrawer")
        tryVerify(function() { return drawer.visible })
        for (const name of ["navInbox", "navAccounts", "navAgents", "navSettings"])
            verifyTapTarget(findChild(drawer, name), name)
        inbox.close()
    }
    function test_readerToolbarTapTargetsReachFortyEightDp() {
        const shown = openInbox(true)
        store.cached = {"single-0": {html: "<p>body</p>", source_url: "https://mail.google.com/mail/u/0/#inbox/abc"}}
        shown.inbox.openPage("MailDetailPage", {store: store, cardId: "single-0", cardTitle: "Reader",
                                                accountName: "a@example.com", card: store.get(8).card, openLinks: false})
        tryCompare(findChild(shown.inbox, "pageStack"), "busy", false)
        const bar = findChild(shown.inbox, "mailAppBar")
        verify(bar)
        verifyTapTarget(findChild(bar, "backToInbox"), "backToInbox")
        verifyTapTarget(findChild(bar, "archiveMail"), "archiveMail")
        verifyTapTarget(findChild(bar, "mailActions"), "mailActions (overflow)")
        shown.inbox.close()
    }
    function test_searchTextFiltersRealFixtureAndClearRestoresAllCards() {
        const shown = openInbox(true)
        mouseClick(findChild(shown.inbox, "toggleSearch"))
        const search = findChild(shown.inbox, "inboxSearch")
        verify(search && search.visible)
        const before = store.cardIds()
        search.text = "QGIS training"
        search.textEdited(search.text)
        tryVerify(function() { shown.list.forceLayout(); return shown.list.count === 1 })
        compare(shown.list.itemAtIndex(0).cardId, "single-9")
        compare(store.cardIds(), before)
        mouseClick(findChild(shown.inbox, "clearInboxSearch"))
        tryCompare(shown.list, "count", before.length)
        compare(store.cardIds(), before)
        shown.inbox.close()
    }
    function test_onlineHeaderIsQuietAndOfflineRetryIsActionable() {
        const online = openInbox(true)
        const connection = findChild(online.inbox, "connectionStatusRow")
        verify(!connection.visible, "synced Online state must not consume header height")
        online.inbox.close()
        const offline = openInbox(false)
        const offlineRow = findChild(offline.inbox, "connectionStatusRow")
        verify(offlineRow && offlineRow.visible)
        const text = textOf(offlineRow)
        verify(text.indexOf("Software caused connection abort") < 0)
        verify(text.indexOf("2") >= 0)
        const retry = findChild(offlineRow, "offlineRetry")
        verify(retry && retry.visible && retry.enabled)
        offline.inbox.close()
    }
    function test_errorExpiresWithoutAcknowledgingPendingWrite() {
        const shown = openInbox(true)
        const inbox = shown.inbox
        const row = findChild(inbox, "connectionStatusRow")
        const timer = findChild(inbox, "operationErrorTimer")
        const payload = {op_id: "owner-write", type: "note", args: {text: "Owner text must survive 422"}}
        store.operations = [payload]
        store.pendingOps = 1
        store.saveFailed = true
        store.failedWrites = [payload.op_id]
        verify(row.visible, "online rejected writes must retain pending status")
        store.operationFailed("reversible-archive", 422, {})
        verify(inbox.operationError.length > 0)
        verify(timer.running)
        verify(timer.interval > 0 && timer.interval <= 8000)
        // Exercise the real timer, not a direct onTriggered call or a shortened interval.
        tryCompare(inbox, "operationError", "", 8500)
        verify(!timer.running)
        compare(store.pendingOps, 1)
        compare(JSON.stringify(store.operations), JSON.stringify([payload]))
        verify(!findChild(inbox, "bundleArchiveUndoBar").visible,
            "expired generic error must leave no persistent error overlay")
        verify(row.visible)
        for (let i = 0; i < 3; ++i) store.operationFailed(payload.op_id, 422, {})
        compare(inbox.operationError, "")
        verify(!timer.running, "background retry must not recreate the toast")
        const retry = findChild(row, "offlineRetry")
        verify(retry.visible && retry.enabled)
        mouseClick(retry)
        compare(store.refreshCalls, 1)
        compare(store.pendingOps, 1, "Retry is not acknowledgement")
        compare(JSON.stringify(store.operations), JSON.stringify([payload]))
        save(inbox, "online-pending-after-error-expiry")
        // Only the simulated server acknowledgement changes the fixture's queue.
        store.operations = []
        store.failedWrites = []
        store.saveFailed = false
        store.pendingOps = 0
        tryCompare(row, "visible", false)
        inbox.close()
    }
    function test_syncTimeRequiresDurableContentNotConnectivity() {
        const shown = openInbox(false)
        const inbox = shown.inbox
        const row = findChild(inbox, "connectionStatusRow")
        inbox.lastSyncAt = 0
        store.mailSyncComplete = false
        store.mailSyncPending = 2
        store.pendingOps = 0
        store.online = true
        compare(inbox.lastSyncAt, 0)
        verify(row.visible)
        verify(inbox.contentSyncIncomplete)
        store.mailSyncStateChanged()
        compare(inbox.lastSyncAt, 0, "metadata/missing bodies cannot report completed sync")
        store.mailSyncPending = 0
        store.mailSyncStateChanged()
        compare(inbox.lastSyncAt, 0, "body persistence must complete before claiming sync")
        verify(row.visible)
        store.mailSyncComplete = true
        store.mailSyncStateChanged()
        verify(inbox.lastSyncAt > 0)
        tryCompare(row, "visible", false)
        const completedAt = inbox.lastSyncAt
        store.online = false
        store.online = true
        compare(inbox.lastSyncAt, completedAt, "reconnect must not invent a new successful sync")
        store.online = false
        store.pendingOps = 1
        store.pendingOps = 0
        compare(inbox.lastSyncAt, completedAt, "offline queue transitions must not claim a fresh sync")
        inbox.close()
    }
    // A background SQLite failure must not acquire ownership of an editor.
    function test_backgroundStorageFailureRecoversWithoutForegroundSave() {
        const shown = openInbox(true)
        const inbox = shown.inbox
        const row = findChild(inbox, "connectionStatusRow")
        inbox.lastSyncAt = 0
        store.mailSyncComplete = false
        store.mailSyncPending = 1
        store.storageError("fixture background body write failed")
        compare(inbox.localStorageError, "")
        verify(inbox.offlineSummary().indexOf("draft") < 0)
        verify(row.visible)
        store.online = false
        store.online = true
        store.mailSyncPending = 0
        store.mailSyncStateChanged()
        compare(inbox.lastSyncAt, 0, "reconnect and optimistic cache are not durable recovery")
        verify(row.visible)
        tryCompare(inbox, "operationError", "", 8500)
        compare(inbox.lastSyncAt, 0, "notice expiry is not durable recovery")
        verify(row.visible)
        store.mailSyncComplete = true
        store.mailSyncStateChanged()
        verify(inbox.lastSyncAt > 0)
        compare(inbox.localStorageError, "")
        tryCompare(row, "visible", false)
        inbox.close()
    }

    function test_failedNoteSurvivesBackgroundRecoveryUntilOwnSave() {
        const shown = openInbox(true)
        const inbox = shown.inbox
        const dialog = findChild(inbox, "noteDialog")
        const field = findChild(dialog, "noteField")
        dialog.cardId = "agent-mvt"
        dialog.open()
        tryCompare(dialog, "visible", true)
        const exactText = "Owner note\n  preserve spacing and punctuation!  "
        field.text = exactText
        mouseClick(findChild(dialog, "noteSaveButton"))
        verify(dialog.visible)
        verify(dialog.saveError.length > 0)
        compare(field.text, exactText)
        compare(JSON.stringify(store.savedNote), "{}")
        store.mailSyncComplete = false
        store.mailSyncPending = 1
        store.online = false
        store.online = true
        store.mailSyncPending = 0
        store.mailSyncStateChanged()
        verify(dialog.saveError.length > 0, "incomplete sync cannot clear a draft failure")
        store.failLocalSave = false
        store.mailSyncComplete = true
        store.mailSyncStateChanged()
        verify(dialog.visible, "successful unrelated body retry cannot accept a note")
        verify(dialog.saveError.length > 0)
        compare(field.text, exactText)
        compare(JSON.stringify(store.savedNote), "{}")
        mouseClick(findChild(dialog, "noteSaveButton"))
        tryCompare(dialog, "visible", false)
        compare(dialog.saveError, "")
        compare(inbox.localStorageError, "")
        compare(store.savedNote.cardId, "agent-mvt")
        compare(store.savedNote.text, exactText)
        inbox.close()
    }
    function test_authAndLocalStorageRemainActionableAfterErrorExpiry() {
        const shown = openInbox(true)
        const inbox = shown.inbox
        inbox.gmailAccounts = [{address: "fixture@example.test", status: "reauth_required"}]
        const reconnect = findChild(inbox, "reconnectBanner")
        const dialog = findChild(inbox, "createDialog")
        dialog.open()
        tryCompare(dialog, "visible", true)
        const title = findChild(dialog, "createTitle")
        const summary = findChild(dialog, "createSummary")
        title.text = "Unsaved owner draft"
        summary.text = "Exact details survive a local storage failure"
        const before = store.cardIds()
        mouseClick(findChild(dialog, "createSaveButton"))
        verify(dialog.visible)
        compare(title.text, "Unsaved owner draft")
        compare(summary.text, "Exact details survive a local storage failure")
        compare(store.cardIds(), before)
        verify(reconnect.visible)
        compare(inbox.localStorageError, dialog.saveError)
        verify(dialog.saveError.indexOf("keep your draft open") >= 0)
        const row = findChild(inbox, "connectionStatusRow")
        verify(row.visible)
        store.operationFailed("archive", 422, {})
        tryCompare(inbox, "operationError", "", 8500)
        verify(reconnect.visible, "reauth is not transient")
        verify(row.visible, "local storage failure is not transient")
        compare(inbox.localStorageError, dialog.saveError)
        verify(dialog.saveError.length > 0)
        verify(dialog.visible)
        compare(title.text, "Unsaved owner draft")
        compare(summary.text, "Exact details survive a local storage failure")
        store.mailSyncComplete = false
        store.mailSyncPending = 1
        store.online = false
        store.online = true
        store.mailSyncPending = 0
        store.mailSyncStateChanged()
        verify(dialog.saveError.length > 0, "reconnect/incomplete persistence cannot clear the draft")
        store.failLocalSave = false
        store.mailSyncComplete = true
        store.mailSyncStateChanged()
        verify(dialog.visible, "background durable retry cannot accept an unsaved create")
        verify(dialog.saveError.length > 0)
        compare(title.text, "Unsaved owner draft")
        compare(summary.text, "Exact details survive a local storage failure")
        verify(findChild(row, "offlineRetry").enabled)
        inbox.offlineDetailsShown = true
        verify(findChild(inbox, "offlineDetails").visible)
        save(inbox, "auth-local-storage-actionable")
        store.failLocalSave = false
        mouseClick(findChild(dialog, "createSaveButton"))
        tryCompare(dialog, "visible", false)
        compare(inbox.localStorageError, "")
        tryCompare(row, "visible", false)
        verify(reconnect.visible, "successful local save cannot clear account reauth")
        inbox.close()
    }
    // R66: agent report boilerplate is not the preview; mail text is untouched.
    function test_agentPreviewBoilerplate() {
        const inbox = createTemporaryObject(inboxComponent, testCase, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 549, height: 1280
        })
        verify(inbox)
        compare(inbox.cleanPreview("What happened: CI fixed on main"), "CI fixed on main")
        compare(inbox.cleanPreview("**Summary:** three PRs merged"), "three PRs merged")
        compare(inbox.cleanPreview("Meeting: what happened at 5"), "Meeting: what happened at 5")
    }
    function test_captureInboxOffline() {
        const shown = openInbox(false)
        save(shown.inbox, "inbox-offline")
        console.log("MOBILEUX offline header text:", JSON.stringify(textOf(shown.inbox.header)))
    }
    function test_captureExpandedBundle() {
        const shown = openInbox(true)
        shown.inbox.toggleBundle("gh-postgis-4")
        wait(300)
        save(shown.inbox, "inbox-bundle-expanded")
    }
    function test_captureReader() {
        const shown = openInbox(true)
        let body = "<h3>Шаблон договора</h3>"
        for (let i = 0; i < 60; ++i)
            body += "<p>Paragraph " + i + ": the long mail keeps going so the reader has to scroll as one page.</p>"
        store.cached = {"single-0": {html: body, source_url: "https://mail.google.com/mail/u/0/#inbox/abc"}}
        shown.inbox.openPage("MailDetailPage", {store: store, cardId: "single-0", cardTitle: "[postgis-devel] PostGIS Day: proposals close October 15",
                                                accountName: "darafei@maumap.com", card: store.get(8).card, openLinks: false})
        const stack = findChild(shown.inbox, "pageStack")
        tryCompare(stack, "busy", false)
        wait(1500)
        save(shown.inbox, "reader-long-mail")
    }
}
