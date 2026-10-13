import QtQuick
import QtTest
import litterbox 1.0 as App
import "previewcards.js" as Previews

TestCase {
    name: "InboxLayout"
    when: windowShown
    width: 572
    height: 881

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

    function test_mixedPhoneDensity_data() {
        return [{tag: "phone", viewWidth: 412, viewHeight: 915},
                {tag: "neighbor", viewWidth: 480, viewHeight: 915},
                {tag: "desktop", viewWidth: 1280, viewHeight: 800}]
    }
    function test_mixedPhoneDensity(data) {
        store.clear()
        const sources = ["agent", "mail", "manual", "journal", "reminder", "alert", "mail"]
        for (let i = 0; i < sources.length; ++i) {
            const source = sources[i]
            store.append({cardId: "mixed-" + i, title: "Review the latest inbox update " + i, section: "now", card: {
                id: "mixed-" + i, title: "Review the latest inbox update " + i, source: source, section: "now",
                bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: source === "mail",
                summary: "Disposable mixed card: existing phone inbox must stay compact with all actions reachable.",
                snippet: "Disposable mail preview: existing phone inbox must stay compact with all actions reachable.",
                note: "", account_name: source === "mail" ? "owner@example.test" : "",
                sender_name: "Research team", received_at: "2026-10-13T12:00:00Z"
            }})
        }
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.viewWidth, height: data.viewHeight
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(4) !== null })
        wait(200)
        const canvas = inbox.contentItem.parent
        const image = grabImage(canvas)
        compare(image.width, data.viewWidth)
        compare(image.height, data.viewHeight)
        // Scan a blank interior column: white card runs separated by the inbox
        // background count rendered cards, not model rows or recycled delegates.
        const firstFrame = findChild(list.itemAtIndex(0), "inboxCard")
        const scanX = Math.ceil(firstFrame.mapToItem(canvas, 4, 0).x)
        let pixelCards = 0
        let run = 0
        for (let y = Math.ceil(list.mapToItem(canvas, 0, 0).y); y < image.height; ++y) {
            if (Qt.colorEqual(image.pixel(scanX, y), inbox.surface)) run++
            else {
                if (run >= 48) pixelCards++
                run = 0
            }
        }
        // A white run reaching the bottom is a clipped card, not a full card.
        let completeCards = 0
        for (let i = 0; i < list.count; ++i) {
            const row = list.itemAtIndex(i)
            if (!row) continue
            const frame = findChild(row, "inboxCard")
            const top = frame.mapToItem(canvas, 0, 0)
            if (top.y < 0 || top.y + frame.height > image.height) continue
            completeCards++
            const title = findChild(row, "mailTitle-mixed-" + i)
            const primary = findChild(row, "doneButton-mixed-" + i)
            const more = findChild(row, "inboxActions")
            for (const action of [primary, more]) {
                verify(action.visible && action.enabled)
                verify(action.width >= 48 && action.height >= 48)
                const p = action.mapToItem(frame, 0, 0)
                verify(p.x >= 0 && p.y >= 0 && p.x + action.width <= frame.width + 1 && p.y + action.height <= frame.height + 1,
                       "action clipped for " + sources[i])
                const textBottom = title.mapToItem(frame, 0, title.height).y
                verify(inbox.compactInbox ? p.y >= textBottom : p.x >= title.mapToItem(frame, title.width, 0).x,
                       "action overlaps text for " + sources[i])
            }
        }
        compare(pixelCards, completeCards, "pixel oracle must count complete mixed cards")
        verify(pixelCards >= 4, "R33-V: only " + pixelCards + " complete mixed cards fit")
        image.save("/tmp/litterbox-mixed-inbox-" + data.viewWidth + "x" + data.viewHeight + ".png")
        console.log("Mixed inbox " + data.viewWidth + "x" + data.viewHeight + ": " + pixelCards + " complete cards")
        // Exercise the actual overflow target on a non-mail card.
        const more = findChild(list.itemAtIndex(0), "inboxActions")
        mouseClick(more)
        const sheet = findChild(more, "cardActionSheet-mixed-0")
        tryVerify(function() { return sheet && sheet.visible })
        mouseClick(findChild(sheet, "actionSheetClose-mixed-0"))
        inbox.close()
    }

    function test_collapsedMembersLeaveNoHolesAndExpandInPlace() {
        store.clear()
        for (let i = 0; i < 4; ++i) {
            store.append({cardId: "bundle-" + i, title: "A bundled message " + i, section: "now", card: {
                id: "bundle-" + i, title: "A bundled message " + i, source: "mail", section: "now",
                bundle_id: "github", bundle_title: "sender:notifications@github.com", bundle_leader: i === 0,
                bundle_member_count: 4, pinned_rank: null, important: false, timed: false,
                has_body: false, summary: "", snippet: "Bundle member preview for sender context", note: "", account_name: "owner@example.test",
                sender_name: "Bundle sender " + i, sender_address: "sender" + i + "@example.test", received_at: new Date().toISOString()
            }})
        }
        store.append({cardId: "standalone", title: "Next visible card", section: "now", card: {
            id: "standalone", title: "Next visible card", source: "manual", section: "now",
            bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: false,
            summary: "", note: "", account_name: ""
        }})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 572, height: 881
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(4) !== null })
        const leader = list.itemAtIndex(0)
        const next = list.itemAtIndex(4)
        const frame = findChild(leader, "inboxCard")
        const gap = next.y - leader.y - frame.y - frame.height
        verify(gap >= 0 && gap <= inbox.edgeSpacing + 1,
               "collapsed bundle left a hole of " + gap + "px between visible cards")

        const toggle = findChild(leader, "bundleToggle-bundle-0")
        verify(toggle.width >= 48 && toggle.height >= 48)
        mouseClick(toggle)
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(1).visible })
        for (let i = 0; i < 4; ++i) {
            const row = list.itemAtIndex(i)
            const following = list.itemAtIndex(i + 1)
            const rowFrame = findChild(row, "inboxCard")
            const expandedGap = following.y - row.y - rowFrame.y - rowFrame.height
            verify(expandedGap >= 0 && expandedGap <= inbox.edgeSpacing + 1,
                   "expanded bundle spacing differs at row " + i + ": " + expandedGap)
        }
        const expandedToggle = findChild(list.itemAtIndex(0), "bundleToggle-bundle-0")
        for (let i = 0; i < 4; ++i) {
            const row = list.itemAtIndex(i)
            const memberContext = findChild(row, "mailContext-bundle-" + i)
            const memberSender = findChild(row, "mailSender-bundle-" + i)
            const memberDate = findChild(row, (inbox.compactInbox ? "phoneMailDate-" : "mailDate-") + "bundle-" + i)
            const memberSnippet = findChild(row, "mailSnippet-bundle-" + i)
            verify(memberContext && memberContext.visible, "expanded bundle mail context must be visible")
            verify(memberSender && memberSender.visible, "expanded bundle sender must be visible")
            verify(memberDate && memberDate.visible, "expanded bundle arrival date/time must be visible")
            verify(memberSnippet && memberSnippet.visible && memberSnippet.text.indexOf("Bundle member preview") >= 0,
                   "expanded bundle snippet must be visible")
        }
        verify(waitForRendering(expandedToggle), "Expanded opener geometry must be rendered before the second center click")
        mouseClick(expandedToggle)
        tryVerify(function() { list.forceLayout(); return !list.itemAtIndex(1).visible })
        const collapsedLeader = list.itemAtIndex(0)
        const collapsedNext = list.itemAtIndex(4)
        const collapsedFrame = findChild(collapsedLeader, "inboxCard")
        compare(collapsedNext.y - collapsedLeader.y - collapsedFrame.y - collapsedFrame.height, gap)
    }
    function test_singleMailShowsSenderDateAndSnippet() {
        store.clear()
        store.append({cardId: "mail-context", title: "Tree genealogy", section: "now", card: {
            id: "mail-context", title: "Tree genealogy", source: "mail", section: "now",
            bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: true,
            summary: "", snippet: "The family archive includes the latest branch and records.", note: "",
            sender_name: "German Loiko", sender_address: "german@example.test",
            received_at: new Date().toISOString(), account_name: "owner@example.test"
        }})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 700, height: 600
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const row = list.itemAtIndex(0)
        const sender = findChild(row, "mailSender-mail-context")
        const date = findChild(row, "mailDate-mail-context")
        const snippet = findChild(row, "mailSnippet-mail-context")
        verify(sender && sender.visible && sender.text === "German Loiko", "sender must be visible")
        verify(date && date.visible && date.text !== "", "mail arrival date/time must be visible")
        verify(snippet && snippet.visible && snippet.text.indexOf("latest branch") >= 0, "body preview must be visible")
        verify(findChild(row, "mailAccount-mail-context").text === "owner@example.test", "account must remain visible")
        inbox.close()
    }
    function test_cleanPreviewGrid_data() {
        return [{tag: "1440", viewWidth: 1440}, {tag: "1200", viewWidth: 1200}]
    }
    function test_cleanPreviewGrid(data) {
        store.clear()
        const cards = Previews.cards()
        for (const card of cards)
            store.append({cardId: card.id, title: card.title, section: card.section, card: card})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.viewWidth, height: 1100
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(6) !== null })
        wait(200)
        const phase = "after"
        grabImage(inbox.contentItem).save("/tmp/litterbox-clean-previews/" + phase + "-" + data.viewWidth + ".png")
        const flying = findChild(list.itemAtIndex(0), "mailSnippet-flying")
        const jasmine = findChild(list.itemAtIndex(3), "mailSnippet-jasmine")
        const qgis = findChild(list.itemAtIndex(4), "mailSnippet-qgis")
        compare(flying.visible, false, "sender-only preview must not repeat the sender")
        compare(qgis.text, "Hi list")
        verify(jasmine.text.indexOf("\n") < 0 && jasmine.text.indexOf("\r") < 0,
               "preview whitespace must not create an ellipsis-only line")
        compare(jasmine.lineCount, 1)
        const mailRow = list.itemAtIndex(3)
        const bundleRow = list.itemAtIndex(1)
        function x(row, name) {
            const item = findChild(row, name)
            return item.mapToItem(list, 0, 0).x + (item.leftPadding || 0)
        }
        compare(x(mailRow, "mailSender-jasmine"), x(bundleRow, "bundleAccountLabel-odm"))
        compare(x(mailRow, "mailTitle-jasmine"), x(bundleRow, "bundleTitleLabel-odm"))
        compare(findChild(mailRow, "doneButton-jasmine").mapToItem(list, 0, 0).x,
                findChild(bundleRow, "archiveBundle-odm").mapToItem(list, 0, 0).x)
        compare(x(mailRow, "mailDate-jasmine"), x(bundleRow, "bundleDate-odm"))
        compare(findChild(mailRow, "inboxCard").height, findChild(bundleRow, "inboxCard").height)
        inbox.close()
    }
    function test_additionalScreenshot_data() { return test_cleanPreviewGrid_data() }
    function test_additionalScreenshot(data) {
        store.clear()
        const cards = Previews.additionalCards()
        for (const card of cards)
            store.append({cardId: card.id, title: card.title, section: card.section, card: card})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.viewWidth, height: 1250
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(7) !== null })
        wait(200)
        const phase = "after"
        grabImage(inbox.contentItem.parent).save("/tmp/litterbox-clean-previews/additional-" + phase + "-" + data.viewWidth + ".png")
        for (const id of ["postgis6144", "sleep", "confirmation", "maproulette", "hosting"]) {
            let label = null
            for (let i = 0; i < list.count && !label; ++i)
                label = findChild(list.itemAtIndex(i), "mailSnippet-" + id)
            verify(label)
            verify(label.text.indexOf("\n") < 0 && label.text.indexOf("\r") < 0, id + " must not render an ellipsis-only line")
            compare(label.lineCount, 1)
        }
        const mail = findChild(list.itemAtIndex(0), "mailTitle-postgis6144")
        const bundle = findChild(list.itemAtIndex(2), "bundleTitleLabel-miles")
        compare(mail.mapToItem(list, 0, 0).x, bundle.mapToItem(list, 0, 0).x)
        compare(findChild(list.itemAtIndex(0), "mailSnippet-postgis6144").text, "PostgreSQL and POSTGIS version:")
        compare(findChild(list.itemAtIndex(1), "mailSnippet-postgis6143").visible, false, "signature-only reply has no server snippet")
        inbox.close()
    }
}
