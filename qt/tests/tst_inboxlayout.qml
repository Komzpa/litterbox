import QtQuick
import QtTest
import litterbox 1.0 as App
import org.kde.kirigami as Kirigami
import "previewcards.js" as Previews
import "techshipcontext.js" as Newsletter

TestCase {
    name: "InboxLayout"
    when: windowShown
    width: 572
    property url pagesDir: Qt.resolvedUrl("../pages/")
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

    function test_desktopRailSearchAndSplitGeometry_data() {
        const splitWidth = Math.max(1220, Kirigami.Units.gridUnit * 70)
        return [{tag: "narrow-negative-control", viewWidth: 250, rail: false, reader: false},
                {tag: "phone", viewWidth: 412, rail: false, reader: false},
                {tag: "desktop-negative-control", viewWidth: 899, rail: false, reader: false},
                {tag: "desktop-rail-boundary", viewWidth: 900, rail: true, reader: false},
                {tag: "before-split", viewWidth: splitWidth - 1, rail: true, reader: false},
                {tag: "split-boundary", viewWidth: splitWidth, rail: true, reader: true},
                {tag: "wide-split", viewWidth: splitWidth + 260, rail: true, reader: true}]
    }
    function test_desktopRailSearchAndSplitGeometry(data) {
        store.clear()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.viewWidth, height: 900
        })
        verify(inbox)
        inbox.show()
        const canvas = inbox.contentItem
        const rail = findChild(inbox, "desktopNavigation")
        const split = findChild(inbox, "inboxSplit")
        const reader = findChild(inbox, "desktopReaderHost")
        const search = findChild(inbox, "inboxSearch")
        const searchRow = findChild(inbox, "inboxSearchRow")
        const density = findChild(inbox, "inboxDensity")
        const clear = findChild(inbox, "clearInboxSearch")
        tryCompare(rail, "visible", data.rail)
        tryCompare(reader, "visible", data.reader)
        compare(inbox.desktopNavigation, data.rail)
        compare(inbox.desktopReader, data.reader)
        compare(searchRow.visible, data.rail)
        compare(findChild(inbox, "openNavigation").visible, !data.rail)
        if (data.rail) {
            tryVerify(function() { return search.width > 100 && rail.height > 0 && split.width > 0 })
            const railBox = rail.mapToItem(canvas, 0, 0)
            const splitBox = split.mapToItem(canvas, 0, 0)
            fuzzyCompare(splitBox.x, railBox.x + rail.width, 1)
            fuzzyCompare(splitBox.x + split.width, canvas.width, 1)
            verify(rail.width >= 180 && split.height > 0)
            for (const name of ["navInbox", "navAccounts", "navAgents", "navSettings"]) {
                const nav = findChild(rail, name)
                verify(nav && nav.visible && nav.width >= 48 && nav.height >= 48)
                const p = nav.mapToItem(rail, 0, 0)
                verify(p.x >= 0 && p.y >= 0 && p.x + nav.width <= rail.width + 1 && p.y + nav.height <= rail.height + 1,
                       name + " must fit inside the desktop rail")
            }
            const searchBox = searchRow.mapToItem(canvas, 0, 0)
            verify(searchBox.y + searchRow.height <= splitBox.y + 1, "search must not overlap the inbox panes")
            let right = 0
            for (const item of [search, clear, density]) {
                verify(item.visible && item.height >= 48)
                const p = item.mapToItem(searchRow, 0, 0)
                verify(p.x >= right && p.y >= 0 && p.x + item.width <= searchRow.width + 1 && p.y + item.height <= searchRow.height + 1,
                       item.objectName + " must be unclipped and nonoverlapping in desktop search")
                right = p.x + item.width
            }
        }
        if (data.reader) {
            const stack = findChild(inbox, "pageStack")
            tryVerify(function() { return stack.width >= 400 && reader.width >= 520 })
            const stackBox = stack.mapToItem(split, 0, 0)
            const readerBox = reader.mapToItem(split, 0, 0)
            verify(readerBox.x >= stackBox.x + stack.width, "split reader must be beside, not over, the inbox")
            verify(readerBox.x + reader.width <= split.width + 1)
            fuzzyCompare(reader.height, stack.height, 1)
        }
        inbox.close()
    }

    function test_densityMenuIndicatorsDoNotCoverLabels() {
        store.clear()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 323, height: 429
        })
        verify(inbox)
        inbox.show()
        mouseClick(findChild(inbox, "inboxCommands"))
        const menu = findChild(inbox, "headerMenu")
        tryCompare(menu, "visible", true)
        for (const name of ["densityComfortable", "densityCompact"]) {
            const item = findChild(menu, name)
            const label = findChild(item, "menuItemLabel")
            const check = findChild(item, "densityMenuCheck")
            verify(item && label && check && check.visible)
            const labelBox = label.mapToItem(item, 0, 0)
            const checkBox = check.mapToItem(item, 0, 0)
            verify(labelBox.x + label.width <= checkBox.x, "density label must end before its check indicator")
            verify(labelBox.y >= 0 && labelBox.y + label.height <= item.height)
        }
        const comfortable = findChild(menu, "densityComfortable")
        wait(180)
        grabImage(menu.contentItem).save("/tmp/litterbox-density-menu-popup-323x429.png")
        grabImage(inbox.contentItem.parent).save("/tmp/litterbox-density-menu-323x429.png")
        mouseClick(comfortable)
        tryCompare(inbox, "compactDensity", false)
        tryCompare(menu, "visible", false)
        inbox.close()
    }

    function test_emptyInboxMenuSettingsUsesPointerHitTest() {
        store.clear()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 323, height: 429
        })
        verify(inbox)
        inbox.show()
        const emptyState = findChild(inbox, "inboxEmptyState")
        tryCompare(emptyState, "visible", true)
        compare(emptyState.z, 0, "empty content must not outrank popup content")
        mouseClick(findChild(inbox, "inboxCommands"))
        const menu = findChild(inbox, "headerMenu")
        tryCompare(menu, "visible", true)
        wait(180)
        const settings = findChild(menu, "headerMenuSettings")
        const icon = findChild(inbox, "inboxEmptyIcon")
        const target = settings.mapToItem(inbox.contentItem, settings.width / 2, settings.height / 2)
        const iconBox = icon.mapToItem(inbox.contentItem, 0, 0)
        compare(icon.layer.enabled, true, "the icon render node must composite below popup content")
        console.log("empty-menu-geometry", JSON.stringify({
            settingsCenter: {x: target.x, y: target.y},
            envelope: {x: iconBox.x, y: iconBox.y, width: icon.width, height: icon.height}
        }))
        // Dispatch at window coordinates, not directly to the menu item's signal.
        mouseClick(inbox.contentItem, target.x, target.y)
        tryCompare(menu, "visible", false)
        tryCompare(findChild(inbox, "pageStack").currentItem, "objectName", "settingsPage")
        inbox.close()
    }

    function test_narrowNavigationUsesDrawerWithoutSidebar() {
        store.clear()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 250, height: 430
        })
        verify(inbox)
        inbox.show()
        const canvas = inbox.contentItem
        const rail = findChild(inbox, "desktopNavigation")
        const split = findChild(inbox, "inboxSplit")
        compare(rail.visible, false)
        verify(Math.abs(split.width - canvas.width) <= 1, "narrow content must not lose width to a permanent sidebar")
        grabImage(canvas.parent).save("/tmp/litterbox-navigation-250x430.png")
        const opener = findChild(inbox, "openNavigation")
        verify(opener.visible && opener.width >= 48 && opener.height >= 48)
        mouseClick(opener)
        const drawer = findChild(inbox, "navigationDrawer")
        tryCompare(drawer, "visible", true)
        for (const name of ["navInbox", "navAccounts", "navAgents", "navSettings"])
            verify(findChild(drawer, name).visible, name + " remains reachable")
        wait(180)
        grabImage(drawer.contentItem).save("/tmp/litterbox-navigation-drawer-popup-250x430.png")
        grabImage(canvas.parent).save("/tmp/litterbox-navigation-drawer-250x430.png")
        mouseClick(findChild(drawer, "navSettings"))
        tryCompare(drawer, "visible", false)
        compare(findChild(inbox, "pageStack").currentItem.objectName, "settingsPage")
        inbox.handleBack()
        tryCompare(findChild(inbox, "pageStack"), "depth", 1)
        mouseClick(opener)
        tryCompare(drawer, "visible", true)
        mouseClick(findChild(drawer, "navigationSearch"))
        tryCompare(drawer, "visible", false)
        const searchField = findChild(inbox, "inboxSearch")
        tryCompare(inbox, "searchShown", true)
        tryCompare(searchField, "visible", true)
        verify(searchField.visible)
        inbox.close()
    }
    function test_navigationNeighbor480() {
        store.clear()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 480, height: 915
        })
        verify(inbox)
        inbox.show()
        const canvas = inbox.contentItem
        const rail = findChild(inbox, "desktopNavigation")
        compare(rail.visible, false)
        grabImage(canvas.parent).save("/tmp/litterbox-navigation-480x915.png")
        const opener = findChild(inbox, "openNavigation")
        mouseClick(opener)
        const drawer = findChild(inbox, "navigationDrawer")
        tryCompare(drawer, "visible", true)
        wait(180)
        grabImage(drawer.contentItem).save("/tmp/litterbox-navigation-drawer-popup-480x915.png")
        grabImage(canvas.parent).save("/tmp/litterbox-navigation-drawer-480x915.png")
        mouseClick(findChild(drawer, "navAccounts"))
        tryCompare(drawer, "visible", false)
        compare(findChild(inbox, "pageStack").currentItem.objectName, "gmailAccountsPage")
        inbox.handleBack()
        tryCompare(findChild(inbox, "pageStack"), "depth", 1)
        inbox.close()
    }

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
            const source = findChild(row, sources[i] === "mail" ? "mailSender-mixed-" + i : "sourceLabel-mixed-" + i)
            const snippet = findChild(row, "mailSnippet-mixed-" + i)
            verify(title && title.visible && source && source.visible && snippet && snippet.visible)
            for (const landmark of [title, source, snippet]) {
                const p = landmark.mapToItem(frame, 0, 0)
                verify(landmark.width > 0 && landmark.height > 0 && p.x >= 0 && p.y >= 0 &&
                       p.x + landmark.width <= frame.width + 1 && p.y + landmark.height <= frame.height + 1,
                       landmark.objectName + " must have a visible bbox inside its card")
            }
            const titleBox = title.mapToItem(frame, 0, 0)
            const sourceBox = source.mapToItem(frame, 0, 0)
            const snippetBox = snippet.mapToItem(frame, 0, 0)
            verify(sourceBox.y >= titleBox.y + title.height - 1, "source landmark must sit below the subject")
            verify(snippetBox.y >= sourceBox.y + source.height - 1, "preview must sit below source identity")
            verify(title.font.pointSize > source.font.pointSize && title.font.weight > source.font.weight,
                   "subject must retain stronger hierarchy than source metadata")
            const primary = findChild(row, "doneButton-mixed-" + i)
            const more = findChild(row, "inboxActions")
            for (const action of [primary, more]) {
                verify(action.visible && action.enabled)
                // R63: the overflow is one narrow 28x48 control beside the 48x48 archive on phones.
                verify(action.width >= (inbox.compactInbox && action === more ? 28 : 48) && action.height >= 48)
                const p = action.mapToItem(frame, 0, 0)
                verify(p.x >= 0 && p.y >= 0 && p.x + action.width <= frame.width + 1 && p.y + action.height <= frame.height + 1,
                       "action clipped for " + sources[i])
                const textBottom = title.mapToItem(frame, 0, title.height).y
                // R63: phone actions sit in a right column beside the text, not in a row below it.
                verify(p.x >= title.mapToItem(frame, title.width, 0).x,
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
            const memberDate = findChild(row, "mailDate-bundle-" + i)
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
    function test_expandedMemberContext_data() {
        return [{tag: "desktop", viewWidth: 1356, viewHeight: 475},
                {tag: "phone", viewWidth: 412, viewHeight: 915}]
    }
    function test_expandedMemberContext(data) {
        store.clear()
        compare(Newsletter.version, 1)
        const cards = Newsletter.cards()
        const retained = JSON.stringify(cards)
        const single = Object.assign({}, cards[0], {id: "single-context", bundle_id: "", title: "Standalone mail neighbor"})
        delete single.bundle_leader
        function populate(records) {
            store.clear()
            for (const card of records.concat([single]))
                store.append({cardId: card.id, title: card.title, section: card.section, card: card})
        }
        populate(cards)
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.viewWidth, height: data.viewHeight
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = data.viewHeight * 4
        function ready() { list.forceLayout(); return list.itemAtIndex(3) !== null }
        tryVerify(ready)
        function capture(name) {
            wait(150)
            if (typeof mailFixtures !== "undefined")
                grabImage(inbox.contentItem.parent).save(mailFixtures.screenshotPath(name + "-" + data.viewWidth + ".png"))
        }
        capture("collapsed")
        mouseClick(findChild(list.itemAtIndex(0), "bundleToggle-" + cards[0].id))
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(1).visible })
        capture("expanded")
        const shared = cards.map(function(card, index) {
            const row = list.itemAtIndex(index)
            return {sender: findChild(row, "mailSender-" + card.id).visible,
                    account: findChild(row, "mailAccount-" + card.id).visible,
                    context: findChild(row, "mailContext-" + card.id).visible}
        })
        const header = findChild(list.itemAtIndex(0), "bundleAccountLabel-" + cards[0].id)
        const headerHasAccount = header.text.indexOf(cards[0].account_name) >= 0
        compare(header.text.split(cards[0].sender_name).length - 1, 1, "Shared sender appears once on summary")
        compare(header.text.split(cards[0].account_name).length - 1, 1, "Shared account appears once on summary")
        for (let i = 0; i < cards.length; ++i) {
            const row = list.itemAtIndex(i)
            compare(row.cardId, cards[i].id, "Expansion preserves source order")
            compare(store.get(i).card.body, cards[i].body, "Expansion retains full body")
            compare(findChild(row, "mailTitle-" + cards[i].id).text, cards[i].title)
            const preview = findChild(row, "mailSnippet-" + cards[i].id)
            compare(preview.text, cards[i].snippet)
            verify(preview.visible && preview.width > 0 && preview.height > 0)
            const box = preview.mapToItem(inbox.contentItem.parent, 0, 0)
            verify(box.x >= 0 && box.y >= 0 && box.x + preview.width <= data.viewWidth &&
                   box.y + preview.height <= data.viewHeight, "Preview must be inside the rendered viewport")
            // Scan the composited window, not an isolated label render target.
            const painted = grabImage(inbox.contentItem.parent)
            let inkPixels = 0
            const left = Math.ceil(box.x), top = Math.ceil(box.y)
            const background = painted.pixel(left, top)
            for (let y = top; y < Math.floor(box.y + preview.height); ++y)
                for (let x = left; x < Math.floor(box.x + preview.width); ++x)
                    if (!Qt.colorEqual(painted.pixel(x, y), background)) inkPixels++
            verify(inkPixels > 8, "Each original preview must paint visible text pixels: " + cards[i].id)
            verify(findChild(row, "mailDate-" + cards[i].id).visible)
        }
        compare(JSON.stringify(cards), retained, "Consumer does not mutate retained fixture bodies/order")
        const singleRow = list.itemAtIndex(3)
        verify(findChild(singleRow, "mailSender-single-context").visible)
        compare(findChild(singleRow, "mailAccount-single-context").visible, !inbox.compactInbox)
        const mixed = cards.map(function(card) { return Object.assign({}, card) })
        mixed[2].account_name = "different@example.test"
        mixed[2].sender_name = "Different sender"
        populate(mixed)
        tryVerify(ready)
        wait(150)
        capture("mixed-expanded")
        list.positionViewAtEnd()
        capture("mixed-account-and-single-neighbor")
        const mixedRow = list.itemAtIndex(2)
        const mixedAccount = findChild(mixedRow, "mailAccount-" + mixed[2].id)
        const mixedSender = findChild(mixedRow, "mailSender-" + mixed[2].id)
        const mixedAccountVisible = mixedAccount.visible
        const mixedSenderVisible = mixedSender.visible
        compare(mixedAccount.text, "different@example.test")
        compare(mixedSender.text, "Different sender")
        console.log("CONTEXT_ORACLE", JSON.stringify({width: data.viewWidth, shared: shared,
                    headerHasAccount: headerHasAccount, mixedAccount: mixedAccountVisible, mixedSender: mixedSenderVisible}))
        const compactPane = inbox.compactInbox
        inbox.close()
        for (const member of shared) {
            verify(!member.account, "Same-account bundle member must omit repeated account")
            verify(!member.sender, "Same-sender bundle member must omit repeated sender")
            // A compact desktop split pane also keeps the context-line arrival time.
            verify(!member.context || compactPane, "Repeated-only context row must collapse outside compact panes")
        }
        verify(headerHasAccount, "Shared account identity must remain on the bundle summary")
        verify(mixedAccountVisible, "Mixed-account child must retain its own identity")
        verify(mixedSenderVisible, "Mixed-sender child must retain its own identity")
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
        function rowFor(id) {
            for (let i = 0; i < list.count; ++i) {
                const row = list.itemAtIndex(i)
                if (row && row.cardId === id) return row
            }
            fail("Missing preview row " + id)
        }
        const flying = findChild(rowFor("flying"), "mailSnippet-flying")
        const jasmine = findChild(rowFor("jasmine"), "mailSnippet-jasmine")
        const qgis = findChild(rowFor("qgis"), "mailSnippet-qgis")
        compare(flying.visible, false, "sender-only preview must not repeat the sender")
        compare(qgis.text, "Hi list")
        verify(jasmine.text.indexOf("\n") < 0 && jasmine.text.indexOf("\r") < 0,
               "preview whitespace must not create an ellipsis-only line")
        compare(jasmine.lineCount, 1)
        const mailRow = rowFor("jasmine")
        const bundleRow = rowFor("odm")
        function x(row, name) {
            const item = findChild(row, name)
            return item.mapToItem(list, 0, 0).x + (item.leftPadding || 0)
        }
        compare(x(mailRow, "mailSender-jasmine"), x(bundleRow, "bundleAccountLabel-odm"))
        compare(x(mailRow, "mailTitle-jasmine"), x(bundleRow, "bundleTitleLabel-odm"))
        function actionGroup(row, primaryName, overflowName) {
            const primary = findChild(row, primaryName)
            const overflow = findChild(row, overflowName)
            const frame = findChild(row, "inboxCard")
            verify(primary && overflow && primary.visible && overflow.visible)
            verify(primary.width >= 48 && primary.height >= 48)
            verify(overflow.width >= 48 && overflow.height >= 48)
            const a = primary.mapToItem(list, 0, 0)
            const b = overflow.mapToItem(list, 0, 0)
            verify(a.x + primary.width <= b.x || b.x + overflow.width <= a.x,
                   "Primary and overflow hit targets must not overlap")
            const left = Math.min(a.x, b.x)
            const right = Math.max(a.x + primary.width, b.x + overflow.width)
            const top = Math.min(a.y, b.y)
            const bottom = Math.max(a.y + primary.height, b.y + overflow.height)
            const frameBox = frame.mapToItem(list, 0, 0)
            verify(left >= frameBox.x && right <= frameBox.x + frame.width,
                   "Both actions must remain inside the card bbox")
            verify(top >= frameBox.y && bottom <= frameBox.y + frame.height,
                   "Both actions must remain vertically inside the card bbox")
            return {left: left, right: right, top: top, bottom: bottom}
        }
        const mailActions = actionGroup(mailRow, "doneButton-jasmine", "inboxActions")
        const bundleActions = actionGroup(bundleRow, "archiveBundle-odm", "bundleOptions-odm")
        compare(mailActions.left, bundleActions.left)
        compare(mailActions.right, bundleActions.right)
        // Phone and desktop use the same title-aligned trailing date column.
        const mailDateName = "mailDate-jasmine"
        const bundleDateName = "bundleDate-odm"
        const mailDate = findChild(mailRow, mailDateName)
        const bundleDate = findChild(bundleRow, bundleDateName)
        for (const pair of [[mailRow, mailDate], [bundleRow, bundleDate]]) {
            const frame = findChild(pair[0], "inboxCard")
            const date = pair[1]
            verify(date && date.visible && date.width > 0 && date.height > 0)
            const p = date.mapToItem(frame, 0, 0)
            verify(p.x >= 0 && p.y >= 0 && p.x + date.width <= frame.width && p.y + date.height <= frame.height,
                   "The visible arrival date must fit inside its card")
        }
        if (inbox.compactInbox) {
            const mailTitle = findChild(mailRow, "mailTitle-jasmine")
            verify(mailDate.mapToItem(list, 0, 0).x >= mailTitle.mapToItem(list, mailTitle.width, 0).x,
                   "Compact mail arrival stays beside the subject, above the action group")
            verify(bundleDate.mapToItem(list, 0, bundleDate.height).y <= bundleActions.top,
                   "Compact bundle arrival stays above its action group")
        } else {
            compare(x(mailRow, mailDateName), x(bundleRow, bundleDateName))
            compare(findChild(mailRow, "inboxCard").height, findChild(bundleRow, "inboxCard").height)
        }
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
        function rowFor(id) {
            for (let i = 0; i < list.count; ++i) {
                const row = list.itemAtIndex(i)
                if (row && row.cardId === id) return row
            }
            fail("Missing rendered preview row " + id)
        }
        const mail = findChild(rowFor("postgis6144"), "mailTitle-postgis6144")
        const bundle = findChild(rowFor("miles"), "bundleTitleLabel-miles")
        compare(mail.mapToItem(list, 0, 0).x, bundle.mapToItem(list, 0, 0).x)
        compare(findChild(rowFor("postgis6144"), "mailSnippet-postgis6144").text, "PostgreSQL and POSTGIS version:")
        compare(findChild(rowFor("postgis6143"), "mailSnippet-postgis6143").visible, false, "signature-only reply has no server snippet")
        inbox.close()
    }
    // Popup anchoring (bundle-popup-20261021): the bundle-details menu is
    // parented to its trigger, so its x/y are trigger-local like the neighbor
    // headerMenu (x: parent.width - width, y: parent.height + 8). Window-width
    // arithmetic inside trigger coordinates detaches the popup from its trigger.
    function test_bundleMenuAnchorsToItsTrigger_data() {
        return [{tag: "split-1440x900-scrolled-bundle", viewWidth: 1440, viewHeight: 900},
                {tag: "narrow-412x915-viewport-edge", viewWidth: 412, viewHeight: 915}]
    }
    function bundleMenuFor(trigger) {
        for (let i = 0; i < trigger.children.length; ++i) {
            const child = trigger.children[i]
            if (child && child.sourceComponent !== undefined && child.item) return child.item
        }
        return null
    }
    function test_bundleMenuAnchorsToItsTrigger(data) {
        store.clear()
        for (let i = 0; i < 6; ++i) {
            store.append({cardId: "popup-filler-" + i, title: "Scrolled filler " + i, section: "now", card: {
                id: "popup-filler-" + i, title: "Scrolled filler " + i, source: "manual", section: "now",
                bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: false,
                summary: "Disposable filler pushing the bundle below the fold", note: "", account_name: ""
            }})
        }
        for (let i = 0; i < 3; ++i) {
            store.append({cardId: "popup-" + i, title: "Bundle anchor message " + i, section: "now", card: {
                id: "popup-" + i, title: "Bundle anchor message " + i, source: "mail", section: "now",
                bundle_id: "octo", bundle_title: "sender:notifications@github.com", bundle_leader: i === 0,
                bundle_member_count: 3, pinned_rank: null, important: false, timed: false, has_body: false,
                summary: "", snippet: "Bundle member preview for popup anchoring", note: "",
                account_name: "owner@example.test", sender_name: "Octo bot " + i,
                sender_address: "octo" + i + "@example.test", received_at: new Date().toISOString()
            }})
        }
        for (let i = 0; i < 4; ++i) {
            store.append({cardId: "popup-tail-" + i, title: "Trailing card " + i, section: "now", card: {
                id: "popup-tail-" + i, title: "Trailing card " + i, source: "manual", section: "now",
                bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: false,
                summary: "Disposable trailing card", note: "", account_name: ""
            }})
        }
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.viewWidth, height: data.viewHeight
        })
        verify(inbox)
        inbox.show()
        // Window surface: the painted root (grabImage target), so popup and
        // trigger boxes compare in the same mapped space as real screenshots.
        const surface = inbox.contentItem.parent
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(6) !== null })
        // Scrolled bundle: the leader starts below six fillers and is scrolled
        // to the top of the visible list before the popup opens.
        list.positionViewAtIndex(6, ListView.Beginning)
        wait(250)
        list.forceLayout()
        let trigger = null
        for (let i = 0; i < list.count && !trigger; ++i) {
            const row = list.itemAtIndex(i)
            const candidate = row && findChild(row, "bundleOptions-popup-0")
            if (candidate && candidate.visible && candidate.width > 0) trigger = candidate
        }
        verify(trigger, "the bundle leader must expose its options trigger")
        const triggerBox = trigger.mapToItem(surface, 0, 0)
        verify(triggerBox.y >= 0 && triggerBox.y + trigger.height <= surface.height,
               "the scrolled trigger must sit inside the viewport")
        // Negative control: the retired window-width formula with its missing
        // y must fail the same adjacency oracle, so this check can fail.
        const retiredRight = triggerBox.x + inbox.width - 3 * inbox.edgeSpacing
        const retiredTop = triggerBox.y
        const oracleRejectsRetired = Math.abs(retiredRight - (triggerBox.x + trigger.width)) > 2
                || Math.abs(retiredTop - (triggerBox.y + trigger.height + 8)) > 2
        verify(oracleRejectsRetired, "negative control: the oracle must reject the retired global-offset formula")
        mouseClick(trigger)
        const menu = bundleMenuFor(trigger)
        verify(menu, "the trigger's Loader must build the bundle-details menu")
        tryCompare(menu, "visible", true)
        wait(200)
        // Evaluated-parent inspection: the popup's parent is the trigger item,
        // so popup x/y live in trigger-local coordinates and map from there.
        compare(menu.parent, trigger, "the bundle menu must be parented to its trigger")
        const popupBox = menu.background.mapToItem(surface, 0, 0)
        console.log("BUNDLE_POPUP_GEOMETRY " + JSON.stringify({
            tag: data.tag, viewport: [surface.width, surface.height],
            trigger: {x: triggerBox.x, y: triggerBox.y, width: trigger.width, height: trigger.height},
            popup: {x: popupBox.x, y: popupBox.y, width: menu.width, height: menu.height}
        }))
        verify(Math.abs(popupBox.x + menu.width - (triggerBox.x + trigger.width)) <= 2,
               "bundle menu must right-align with its trigger: popupRight " + (popupBox.x + menu.width) + " triggerRight " + (triggerBox.x + trigger.width))
        verify(Math.abs(popupBox.y - (triggerBox.y + trigger.height + 8)) <= 2,
               "bundle menu must sit 8px below its trigger: popupTop " + popupBox.y + " expected " + (triggerBox.y + trigger.height + 8))
        verify(popupBox.x >= -1 && popupBox.x + menu.width <= surface.width + 1,
               "bundle menu must fit the viewport horizontally: " + popupBox.x + ".." + (popupBox.x + menu.width) + " of " + surface.width)
        verify(popupBox.y >= -1 && popupBox.y + menu.height <= surface.height + 1,
               "bundle menu must fit the viewport vertically: " + popupBox.y + ".." + (popupBox.y + menu.height) + " of " + surface.height)
        grabImage(inbox.contentItem.parent).save("/tmp/litterbox-bundle-popup-" + data.tag + ".png")
        menu.close()
        tryCompare(menu, "visible", false)
        // Header menu as control: the neighbor's identical local anchoring
        // must pass the same oracle before and after the fix.
        const commands = findChild(inbox, "inboxCommands")
        verify(commands)
        mouseClick(commands)
        const header = findChild(inbox, "headerMenu")
        tryCompare(header, "visible", true)
        wait(150)
        compare(header.parent, commands, "the header menu keeps its evaluated trigger parent")
        const commandBox = commands.mapToItem(surface, 0, 0)
        const headerBox = header.background.mapToItem(surface, 0, 0)
        console.log("HEADER_MENU_GEOMETRY " + JSON.stringify({
            tag: data.tag, viewport: [surface.width, surface.height],
            trigger: {x: commandBox.x, y: commandBox.y, width: commands.width, height: commands.height},
            popup: {x: headerBox.x, y: headerBox.y, width: header.width, height: header.height}
        }))
        verify(Math.abs(headerBox.x + header.width - (commandBox.x + commands.width)) <= 2,
               "header menu control must right-align with its trigger: popupRight " + (headerBox.x + header.width) + " triggerRight " + (commandBox.x + commands.width))
        verify(Math.abs(headerBox.y - (commandBox.y + commands.height + 8)) <= 2,
               "header menu control must sit 8px below its trigger: popupTop " + headerBox.y + " expected " + (commandBox.y + commands.height + 8))
        verify(headerBox.x >= -1 && headerBox.x + header.width <= surface.width + 1
               && headerBox.y >= -1 && headerBox.y + header.height <= surface.height + 1,
               "header menu control must stay inside the viewport: " + headerBox.x + ".." + (headerBox.x + header.width) + " x " + headerBox.y + ".." + (headerBox.y + header.height) + " of " + surface.width + "x" + surface.height)
        grabImage(inbox.contentItem.parent).save("/tmp/litterbox-header-menu-control-" + data.tag + ".png")
        header.close()
        inbox.close()
    }
}
