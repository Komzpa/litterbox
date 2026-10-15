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
        property var cached: ({})
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cardIds() { const ids = []; for (let i = 0; i < count; ++i) ids.push(get(i).cardId); return ids }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function refresh() {}
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
        // Time sits on the sender header line; archive and overflow stay at the right edge.
        const sender = findChild(polisRow, "mailSender-polis-1")
        const time = findChild(polisRow, "phoneMailDate-polis-1")
        verify(time.visible)
        fuzzyCompare(time.mapToItem(polisRow, 0, 0).y, sender.mapToItem(polisRow, 0, 0).y, 4)
        const archive = findChild(polisRow, "doneButton-polis-1")
        verify(archive.visible)
        verify(archive.mapToItem(polisRow, 0, 0).x + archive.width > polisRow.width - 64)
        const more = findChild(polisRow, "inboxActions")
        verify(more.visible)
        // One row: the narrow overflow sits beside the archive, not below it.
        fuzzyCompare(more.mapToItem(polisRow, 0, 0).y, archive.mapToItem(polisRow, 0, 0).y, 1)
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
