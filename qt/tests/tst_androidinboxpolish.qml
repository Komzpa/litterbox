import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "AndroidInboxPolish"
    when: windowShown
    width: 1440
    height: 915
    property url pagesDir: Qt.resolvedUrl("../pages/")
    ListModel {
        id: store
        property bool online: false
        function cardIds() { const ids = []; for (let i = 0; i < count; ++i) ids.push(get(i).cardId); return ids }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function refresh() {}
    }
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
    function fixture() {
        store.clear()
        const now = new Date()
        function stamp(days, hour, minute) { const d = new Date(now); d.setDate(d.getDate() - days); d.setHours(hour, minute, 0, 0); return d.toISOString() }
        function add(id, subject, days, bundle, leader, count, hour, minute, sender) {
            const card = {id: id, title: subject, source: "mail", section: "now", pinned_rank: null,
                important: false, timed: false, has_body: false, account_name: "owner@example.test",
                sender_name: sender || "osgeo.org", received_at: stamp(days, hour || 10, minute || 0),
                snippet: "The latest review is ready. Please review the original message.", summary: "", note: "", bundle_id: bundle || ""}
            if (bundle) { card.bundle_leader = leader; card.bundle_member_count = count; card.bundle_title = subject }
            store.append({cardId: id, title: subject, section: "now", card: card})
        }
        // Deliberately unsorted input mirrors the owner's screenshot without private mail.
        add("older", "OSGeo community news", 3)
        add("yesterday", "Your billing just got simpler", 1, "", undefined, 0, 21, 11)
        add("today-head", "Review latest patch", 0, "today", true, 1, 13, 32, "can1357/oh-my-pi [bot]")
        add("yesterday-head", "Review project changes", 1, "yesterday", true, 2, 18, 20, "can1357/oh-my-pi [bot]")
        add("yesterday-child", "Fix crash in spatial transformation", 1, "yesterday", false, 2, 17, 10, "can1357/oh-my-pi [bot]")
        add("today-single", "Review latest patch", 0, "", undefined, 0, 12, 0, "can1357/oh-my-pi [bot]")
        add("older-head", "OSGeo review comments", 1, "older", true, 2, 16, 0)
        add("older-child", "Original project discussion", 1, "older", false, 2, 15, 0)
    }
    function test_capture_data() {
        return [{tag: "phone", w: 549, h: 1280}, {tag: "neighbor", w: 480, h: 1280},
            {tag: "narrow", w: 412, h: 915}, {tag: "desktop", w: 1440, h: 900}]
    }
    function test_headerOrder() {
        fixture()
        // A non-mail priority slot must not split the mail sort into independent runs.
        store.insert(1, {cardId: "manual", title: "Keep this task slot", section: "now",
            card: {id: "manual", title: "Keep this task slot", source: "manual", section: "now", timed: false}})
        const inbox = createTemporaryObject(inboxComponent, this, {store: store, api: api, updater: updater,
            timeRules: timeRules, width: 549, height: 1280})
        verify(inbox)
        inbox.show()
        tryCompare(findChild(inbox, "inboxList"), "count", 9)
        compare(Object.values(inbox.dateStarts), ["Today", "Yesterday", "Earlier"], "unique newest-first header order across non-mail slots")
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(1) !== null })
        compare(list.itemAtIndex(1).cardId, "manual", "non-mail priority slot is unchanged")
        console.info("HEADER ORDER: " + Object.values(inbox.dateStarts).join(" > "))
        inbox.close()
    }
    function test_capture(data) {
        fixture()
        const inbox = createTemporaryObject(inboxComponent, this, {store: store, api: api, updater: updater,
            timeRules: timeRules, width: data.w, height: data.h})
        verify(inbox)
        inbox.show()
        inbox.expandedBundles = {yesterday: true}
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        wait(300)
        compare(Object.values(inbox.dateStarts), ["Today", "Yesterday", "Earlier"], "date landmarks occur once, newest first")
        compare(findChild(inbox, "inboxCommands").icon.name, "overflow-menu")
        compare(findChild(inbox, "openNavigation").visible, data.w < 900)
        function rowFor(id) {
            for (let i = 0; i < list.count; ++i) {
                const row = list.itemAtIndex(i)
                if (row && row.cardId === id) return row
            }
            fail("Missing rendered fixture row " + id)
        }
        const single = rowFor("today-single")
        const singleFrame = findChild(single, "inboxCard")
        const singleTitle = findChild(single, "mailTitle-today-single")
        const singleArchive = findChild(single, "doneButton-today-single")
        const singleDate = findChild(single, "mailDate-today-single")
        for (const id of ["today-head", "yesterday-head", "yesterday-child", "yesterday", "older", "older-head"]) {
            const row = rowFor(id)
            const frame = findChild(row, "inboxCard")
            const bundle = id.endsWith("head")
            const title = findChild(row, (bundle ? "bundleTitleLabel-" : "mailTitle-") + id)
            const date = findChild(row, (bundle ? "bundleDate-" : "mailDate-") + id)
            const archive = findChild(row, (bundle ? "archiveBundle-" : "doneButton-") + id)
            const more = findChild(row, bundle ? "bundleOptions-" + id : "inboxActions")
            compare(title.mapToItem(frame, 0, 0).x, singleTitle.mapToItem(singleFrame, 0, 0).x)
            compare(date.parent.width - date.x - date.width, (48 - inbox.actionIconSize) / 2, "date has the same trailing inset as action icon ink")
            compare(archive.mapToItem(frame, 0, 0).x, singleArchive.mapToItem(singleFrame, 0, 0).x)
            compare(archive.icon.width, singleArchive.icon.width, "bundle and single archive icons have equal width")
            compare(archive.icon.height, singleArchive.icon.height, "bundle and single archive icons have equal height")
            if (bundle && id !== "yesterday-head") {
                const bundleAvatar = findChild(row, "senderAvatar")
                const singleAvatar = findChild(single, "senderAvatar")
                compare(bundleAvatar.mapToItem(frame, 0, 0).x, singleAvatar.mapToItem(singleFrame, 0, 0).x, "bundle and single avatars share the leading x")
            }
            verify(Math.abs(title.mapToItem(frame, 0, 0).y - date.mapToItem(frame, 0, 0).y) <= 6, "time shares the title line")
            fuzzyCompare(archive.mapToItem(frame, 0, 0).y, more.mapToItem(frame, 0, 0).y, 1)
            for (const item of [title, date, archive, more]) {
                const p = item.mapToItem(frame, 0, 0)
                verify(p.x >= 0 && p.y >= 0 && p.x + item.width <= frame.width + 1 && p.y + item.height <= frame.height + 1, item.objectName + " stays inside card")
            }
            verify(archive.width >= 48 && archive.height >= 48 && more.width >= 48 && more.height >= 48)
            if (bundle && id !== "yesterday-head") {
                const chip = findChild(row, "bundleMemberCount-" + id)
                compare(chip.visible, id !== "today-head", "count chip appears only for two or more messages")
                if (id === "today-head") {
                    verify(Math.abs(frame.height - singleFrame.height) <= 2, "one-message thread and matched single bounding boxes have equal height within 2px")
                    console.info("BBOX " + data.w + ": single=" + singleFrame.width + "x" + singleFrame.height + "; one-message thread=" + frame.width + "x" + frame.height)
                } else {
                    verify(frame.height <= singleFrame.height + chip.parent.height + 6, "multi-message thread adds only its count row and spacing")
                }
            }
        }
        const child = rowFor("yesterday-child")
        const childTitle = findChild(child, "mailTitle-yesterday-child")
        verify(!childTitle.truncated, "expanded child subject wraps instead of clipping")
        const childFrame = findChild(child, "inboxCard")
        verify(childFrame.height <= singleFrame.height, "expanded child is a compact sub-row")
        compare(childFrame.background.border.width, 0, "expanded child has no inner card border")
        compare(childFrame.background.radius, 0, "expanded child has no inner rounded card")
        compare(childFrame.topPadding, 4)
        compare(childFrame.bottomPadding, 4)
        compare(childTitle.mapToItem(childFrame, 0, 0).x, singleTitle.mapToItem(singleFrame, 0, 0).x, "child uses the parent text column, not another nested indent")
        compare(inbox.dateStarts["yesterday-child"], undefined)
        const headRail = findChild(rowFor("yesterday-head"), "bundleRail")
        const childRail = findChild(child, "bundleRail")
        fuzzyCompare(headRail.mapToItem(list, 0, 0).x, childRail.mapToItem(list, 0, 0).x, 1)
        const image = grabImage(inbox.contentItem.parent)
        compare(image.width, data.w)
        compare(image.height, data.h)
        image.save(mailFixtures.screenshotPath("current-" + data.w + "x" + data.h + ".png"))
        inbox.close()
    }
}
