import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    id: testCase
    name: "BundleExpand"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    property var store: bundleStore
    QtObject {
        id: api
        property string baseUrl: ""
        property string token: ""
        property var accounts: []
        function get(path, callback) { callback(null, {body: accounts}) }
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

    function card(id, bundle, section, extra) {
        return Object.assign({id: id, title: id, source: "mail", section: section,
            bundle_id: bundle, bundle_title: bundle, pinned_rank: null, important: false,
            timed: false, has_body: false, summary: "", note: "", account_name: "test@example.test"}, extra || {})
    }
    function snapshot(list, ids) {
        const rows = []
        for (let i = 0; i < list.count; ++i) {
            const row = list.itemAtIndex(i)
            if (row && ids.indexOf(row.cardId) >= 0)
                rows.push({id: row.cardId, index: i, visible: row.visible, y: row.y, height: row.height,
                    leader: row.card.bundle_leader, section: row.card.section})
        }
        return rows
    }
    function capture(inbox, name) {
        if (!bundleProofDirectory) return
        const image = grabImage(inbox.contentItem.parent)
        image.save(bundleProofDirectory + "/" + name + ".png")
    }
    function test_harnessSmoke() {
        if (bundleCacheLoaded) return
        verify(store.applyRemoteCards({now: [card("smoke", "", "now")], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 1440, height: 1000
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { return list.itemAtIndex(0) !== null })
        compare(list.itemAtIndex(0).cardId, "smoke")
        verify(list.itemAtIndex(0).visible)
        inbox.close()
    }
    function test_senderSummary_data() {
        const cases = [
            {tag: "one-sender", names: ["LinkedIn", "LinkedIn"]},
            {tag: "two-senders", names: ["LinkedIn", "LinkedIn Job Alerts", "LinkedIn"]},
            {tag: "more-senders", names: ["LinkedIn", "LinkedIn Job Alerts", "LinkedIn", "Recruiting", "Careers"]},
            {tag: "missing-sender", names: [undefined, ""]},
            {tag: "custom-topic-negative-control", names: ["LinkedIn", "LinkedIn Job Alerts"], bundle: "Career opportunities"}
        ]
        const rows = []
        for (const width of [412, 520, 1356])
            for (const data of cases)
                rows.push(Object.assign({}, data, {tag: data.tag + "-" + width, width: width}))
        return rows
    }
    function test_senderSummary(data) {
        const bundle = data.bundle || "sender:messages-noreply@linkedin.com"
        const cards = data.names.map((name, index) => card("sender-" + index, bundle, "now", {
            sender_name: name, title: index === 0 ? "Your latest career update" : "An earlier update",
            snippet: "New projects and conversations from your network.",
            received_at: index === 0 ? "2026-10-17T12:00:00Z" : "2026-10-16T12:00:00Z"
        }))
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: data.width, height: 915
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const row = list.itemAtIndex(0)
        const toggle = findChild(row, "bundleToggle-sender-0")
        const expectedTitle = data.bundle || cards[0].title
        tryCompare(toggle, "text", expectedTitle)
        const sender = findChild(row, "bundleAccountLabel-sender-0")
        verify(sender && sender.visible && sender.text.indexOf(cards[0].account_name) >= 0)
        for (const name of Array.from(new Set(data.names.filter(name => name))).slice(0, 2))
            verify(sender.text.indexOf(name) >= 0, "bundle metadata must retain sender identity")
        const latest = findChild(row, "bundleLatestLabel-sender-0")
        verify(latest && latest.visible && latest.height > 0, "Collapsed preview must show the latest email snippet")
        compare(latest.text, cards[0].snippet)
        const title = findChild(row, "bundleTitleLabel-sender-0")
        compare(title.text, expectedTitle, "Sender groups use the latest original subject; custom topics stay custom")
        verify(title.visible && title.width > 0 && title.height > 0 && !title.truncated)
        capture(inbox, "sender-summary-" + data.tag + "-collapsed")
        mouseClick(toggle, toggle.width / 2, toggle.height / 2)
        tryCompare(inbox.expandedBundles, bundle, true)
        const expandedLatest = findChild(list.itemAtIndex(0), "bundleLatestLabel-sender-0")
        verify(!expandedLatest.visible, "Expanded cards show their subjects instead of the collapsed preview")
        const expandedTitle = findChild(list.itemAtIndex(0), "bundleTitleLabel-sender-0")
        compare(expandedTitle.text, expectedTitle, "Expansion must preserve the original subject or custom topic")
        verify(expandedTitle.visible && expandedTitle.width > 0 && expandedTitle.height > 0 && !expandedTitle.truncated)
        capture(inbox, "sender-summary-" + data.tag + "-expanded")
        inbox.close()
    }
    function test_senderBundlesUseOriginalSubjectsAndSkipPinnedMembers() {
        const subjects = ["Synthetic river survey", "Synthetic bridge review", "Synthetic flood note"]
        const ids = ["42111111-1111-4111-8111-111111111111", "42111111-1111-4111-8111-111111111112", "42111111-1111-4111-8111-111111111113"]
        const bundle = "generic-group-782"
        const cards = ids.map((id, index) => card(id, bundle, "now", {
            title: subjects[index], bundle_title: "sender:robot@synthetic.test", state: "open",
            snippet: "Synthetic unrelated project note", sender_name: "Synthetic robot", sender_address: "robot@synthetic.test",
            received_at: "2026-10-1" + (index + 1) + "T12:00:00Z"
        }))
        const pinned = card("42111111-1111-4111-8111-111111111114", bundle, "pinned", {
            title: "Pinned synthetic outlier", bundle_title: "sender:robot@synthetic.test", state: "open", pinned_rank: 1,
            sender_name: "Different pinned writer", sender_address: "pinned@synthetic.test",
            account_name: "other@synthetic.test", received_at: "2026-10-16T12:00:00Z"
        })
        verify(store.applyRemoteCards({now: cards.concat([pinned]), later: [], missed: []}))
        const originalOrder = [pinned.id].concat(ids)
        compare(store.cardIds(), originalOrder)
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 1440, height: 900
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 30000
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(3) !== null })
        const pinnedRow = list.itemAtIndex(0)
        const summary = list.itemAtIndex(1)
        compare(pinnedRow.cardId, pinned.id)
        verify(pinnedRow.visible && !pinnedRow.bundleLeader && !pinnedRow.bundled, "Pinned mail stays standalone")
        const title = findChild(summary, "bundleTitleLabel-" + ids[0])
        compare(title.text, subjects[2], "Newest unpinned original subject wins, not the newer pinned outlier")
        verify(title.text.indexOf("sender:") < 0)
        const metadata = findChild(summary, "bundleAccountLabel-" + ids[0])
        verify(metadata.text.indexOf("Different pinned writer") < 0 && metadata.text.indexOf("other@synthetic.test") < 0)
        verify(!list.itemAtIndex(2).visible && !list.itemAtIndex(3).visible)
        mouseClick(findChild(summary, "bundleToggle-" + ids[0]))
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(2).visible && list.itemAtIndex(3).visible })
        for (let i = 0; i < 3; ++i) compare(list.itemAtIndex(i + 1).card.title, subjects[i])
        const landmark = findChild(summary, "dateLandmark-" + ids[0])
        verify(landmark && landmark.visible && landmark.text.length > 0)
        const frame = findChild(summary, "inboxCard")
        const landmarkBox = landmark.mapToItem(summary, 0, 0)
        verify(landmark.width > 0 && landmark.height > 0 && landmarkBox.y + landmark.height <= frame.y + 1,
               "Date landmark must occupy its own bbox above the card")
        inbox.compactDensity = false
        wait(100)
        const comfortableHeight = frame.height
        inbox.compactDensity = true
        wait(100)
        verify(frame.height < comfortableHeight, "Density changes geometry without changing content order")
        compare(store.cardIds(), originalOrder)
        mouseClick(findChild(summary, "archiveBundle-" + ids[0]))
        tryVerify(function() { return store.cardIds().length === 1 })
        compare(store.cardIds(), [pinned.id], "Bulk archive excludes the pinned outlier")
        mouseClick(findChild(inbox, "bundleArchiveUndoButton"))
        tryVerify(function() { return store.cardIds().length === originalOrder.length })
        compare(store.cardIds(), originalOrder)
        inbox.close()
    }
    function test_receiptTitleUsesNarrowCardWidth() {
        const bundle = "topic:receipts"
        const titleText = "Your receipt from NOVA SCENT PRIVATE LIMITED #2547-5714"
        verify(store.applyRemoteCards({now: [
            card("receipt-leader", bundle, "now", {bundle_title: titleText}),
            card("receipt-member", bundle, "now", {bundle_title: titleText})
        ], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 412, height: 915
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        function checkGeometry() {
            const row = list.itemAtIndex(0)
            const frame = findChild(row, "inboxCard")
            const title = findChild(row, "bundleTitleLabel-receipt-leader")
            const badge = findChild(row, "bundleMemberCount-receipt-leader").parent
            // R67: the avatar column (48 px) precedes the title on phones.
            verify(title.width >= (frame.width - 48) * 0.6, "At 412 px the title must use at least 60% of the card width beside the avatar: " + title.width + " / " + frame.width)
            compare(title.wrapMode, Text.WordWrap, "Narrow titles must never wrap mid-word")
            const titleTop = title.mapToItem(frame, 0, 0)
            const badgeTop = badge.mapToItem(frame, 0, 0)
            verify(badgeTop.y >= titleTop.y + title.height || badgeTop.y + badge.height <= titleTop.y
                || badgeTop.x >= titleTop.x + title.width || badgeTop.x + badge.width <= titleTop.x,
                "The count badge must not overlap the title")
        }
        wait(100)
        checkGeometry()
        inbox.toggleBundle(bundle)
        tryCompare(inbox.expandedBundles, bundle, true)
        wait(100)
        list.forceLayout()
        checkGeometry()
        inbox.close()
    }
    function test_nonAdjacentMembersExpandUnderLeader_data() {
        return [{tag: "1440", width: 1440, height: 1000}, {tag: "598", width: 598, height: 1200}, {tag: "520", width: 520, height: 900}]
    }
    function test_nonAdjacentMembersExpandUnderLeader(data) {
        let ids
        let leaderId
        let bundle
        if (bundleCacheLoaded) {
            const leader = bundleCachedCards.find(card => card.bundle_title === "sender:messages-noreply@linkedin.com" && card.bundle_leader === true)
            verify(leader, "Copied cache must contain the LinkedIn bundle")
            bundle = leader.bundle_id
            ids = bundleCachedCards.filter(card => card.bundle_id === bundle && card.bundle_leader !== undefined).map(card => card.id)
        } else {
            bundle = "sender:messages-noreply@linkedin.com"
            ids = ["leader", "member-now", "member-later"]
            verify(store.applyRemoteCards({now: [card("leader", bundle, "now"), card("unrelated", "", "now"),
                card("member-now", bundle, "now"), card("important", bundle, "now", {important: true}),
                card("pinned", bundle, "now", {pinned_rank: 1})],
                later: [card("later-unrelated", "", "later"), card("member-later", bundle, "later")], missed: []}))
        }
        leaderId = ids[0]
        const sourceIds = store.cardIds()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.width, height: data.height, title: "Bundle proof " + data.width
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 30000
        tryVerify(function() { list.forceLayout(); return snapshot(list, ids).length === ids.length }, 15000)
        let rows = snapshot(list, ids)
        console.log("BUNDLE_COLLAPSED", JSON.stringify(rows), JSON.stringify(inbox.expandedBundles))
        compare(rows.filter(row => row.visible).length, 1)
        compare(rows.find(row => row.id === leaderId).leader, true)
        const leader = rows.find(row => row.id === leaderId)
        list.positionViewAtIndex(leader.index, ListView.Beginning)
        wait(200)
        capture(inbox, "collapsed-" + data.width)
        const toggle = findChild(list.itemAtIndex(leader.index), "bundleToggle-" + leaderId)
        verify(toggle)
        verify(toggle.width >= 48 && toggle.height >= 48)
        const archive = findChild(list.itemAtIndex(leader.index), "archiveBundle-" + leaderId)
        verify(archive && archive.visible)
        const collapsedRows = snapshot(list, sourceIds)
        if (data.width <= 598) {
            const titleLabel = findChild(list.itemAtIndex(leader.index), "bundleTitleLabel-" + leaderId)
            const accountLabel = findChild(list.itemAtIndex(leader.index), "bundleAccountLabel-" + leaderId)
            verify(titleLabel && !titleLabel.truncated, "Bundle title must not be elided at 520 or 598 px")
            verify(accountLabel && !accountLabel.truncated, "Bundle account must not be elided at 520 or 598 px")
            compare(archive.width, 48, "Narrow archive action must retain a 48 px hit target")
            verify(archive.height >= 48)
            verify(archive.Accessible.name.length > 0, "bundle archive must have an accessible action name")
        }
        const settledLeader = collapsedRows.find(row => row.id === leaderId)
        const nextVisible = collapsedRows.find(row => row.visible && row.index > settledLeader.index)
        verify(nextVisible)
        const collapsedGap = nextVisible.y - settledLeader.y - settledLeader.height
        verify(collapsedGap >= 0 && collapsedGap <= inbox.edgeSpacing + 1, "Collapsed visible cards must have no spacing hole")
        mouseClick(toggle, 12, toggle.height / 2)
        tryCompare(inbox.expandedBundles, bundle, true)
        wait(200)
        list.forceLayout()
        rows = snapshot(list, ids)
        console.log("BUNDLE_EXPANDED", JSON.stringify(rows), JSON.stringify(inbox.expandedBundles))
        capture(inbox, "expanded-" + data.width)
        compare(rows.filter(row => row.visible).length, ids.length)
        const ordered = rows.sort((a, b) => a.index - b.index)
        for (let i = 1; i < ordered.length; ++i) {
            compare(ordered[i].id, ids[i])
            compare(ordered[i].index, ordered[0].index + i, "Expanded members must be immediately after the leader")
            verify(Math.abs(ordered[i].y - ordered[i - 1].y - ordered[i - 1].height) <= 1,
                "Expanded members must have no intervening cards or section headings")
        }
        compare(store.cardIds(), sourceIds, "Expansion is presentation-only, not a persistent reorder")
        const expandedToggle = findChild(list.itemAtIndex(ordered[0].index), "bundleToggle-" + leaderId)
        mouseClick(expandedToggle, 12, expandedToggle.height / 2)
        tryCompare(inbox.expandedBundles, bundle, false)
        tryVerify(function() { list.forceLayout(); return snapshot(list, ids).filter(row => row.visible).length === 1 })
        for (const row of snapshot(list, ids).filter(row => row.id !== leaderId)) {
            verify(!row.visible)
            compare(row.height, 0, "Hidden members must consume no height")
        }
        tryVerify(function() {
            list.forceLayout()
            const visible = snapshot(list, [leaderId, nextVisible.id])
            return visible.length === 2 && Math.abs(visible[1].y - visible[0].y - visible[0].height - collapsedGap) <= 1
        }, 5000, "Collapse must restore the following visible card's gap")
        if (!bundleCacheLoaded) {
            const exempt = snapshot(list, ["important", "pinned"])
            compare(exempt.filter(row => row.visible).length, 2, "Exempt cards remain standalone")
        }
        inbox.close()
    }
    function test_singleImportantMemberDisclosureAndCount() {
        const bundle = "topic:security"
        const leader = card("single-leader", bundle, "now", {
            bundle_id: bundle, bundle_title: "Security alerts", bundle_leader: true,
            bundle_member_count: 1, important: false
        })
        const important = card("single-important", bundle, "now", {
            bundle_id: bundle, bundle_title: "Security alerts", bundle_leader: false,
            bundle_member_count: 1, important: true
        })
        verify(store.applyRemoteCards({now: [leader, important], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 700, height: 900
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const row = list.itemAtIndex(0)
        const count = findChild(row, "bundleMemberCount-single-leader")
        const disclosure = findChild(row, "importantDisclosure-single-leader")
        verify(count && count.visible && /\b1\b/.test(count.text))
        verify(disclosure && disclosure.visible && /\b1\b/.test(disclosure.text) && /\b2\b/.test(disclosure.text),
               "disclosure must distinguish the separate important member from both unpinned archive targets")
        inbox.close()
    }
    function test_noImportantDisclosureWhenCountIsZero() {
        const bundle = "topic:quiet"
        const cards = [card("quiet-leader", bundle, "now"), card("quiet-member", bundle, "now")]
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 700, height: 900
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const disclosure = findChild(list.itemAtIndex(0), "importantDisclosure-quiet-leader")
        verify(disclosure)
        verify(!disclosure.visible)
        inbox.close()
    }
    function test_expandedFrameEnclosesMembers_data() {
        return [
            {tag: "two-520", count: 2, width: 520, height: 900},
            {tag: "three-520", count: 3, width: 520, height: 900},
            {tag: "two-598", count: 2, width: 598, height: 1200},
            {tag: "three-598", count: 3, width: 598, height: 1200},
            {tag: "important-last-520", count: 3, width: 520, height: 900, importantLast: true},
            {tag: "important-last-598", count: 3, width: 598, height: 1200, importantLast: true}
        ]
    }
    function test_expandedFrameEnclosesMembers(data) {
        const bundle = "topic:frame-" + data.tag
        const cards = []
        for (let i = 0; i < data.count; ++i)
            cards.push(card("frame-" + i, bundle, "now", {important: !!data.importantLast && i === data.count - 1}))
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.width, height: data.height
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 30000
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(data.count - 1) !== null })
        const toggle = findChild(list.itemAtIndex(0), "bundleToggle-frame-0")
        verify(toggle && toggle.visible)
        mouseClick(toggle, 12, toggle.height / 2)
        tryCompare(inbox.expandedBundles, bundle, true)
        wait(200)
        list.forceLayout()
        capture(inbox, "frame-" + data.tag)
        const failures = []
        const geometry = []
        const ordinaryCount = data.count - (data.importantLast ? 1 : 0)
        let previousSides = null
        let frameTop = -1
        let lastMemberBottom = -1
        let frameBottom = -1
        function check(condition, message) {
            if (!condition) failures.push(message)
        }
        for (let i = 0; i < list.count; ++i) {
            const row = list.itemAtIndex(i)
            verify(row && row.visible)
            const frame = findChild(row, "inboxCard")
            verify(frame && frame.background)
            const background = frame.background
            const top = background.mapToItem(inbox.contentItem, 0, 0).y
            const bottom = background.mapToItem(inbox.contentItem, 0, background.height).y
            const sides = {left: null, right: null}
            let topEdges = 0
            let bottomEdges = 0
            for (const edge of background.children) {
                if (!edge.visible || edge.opacity === 0) continue
                if (edge.width === 1 && edge.height === background.height) {
                    const side = Math.abs(edge.x) <= 1 ? "left"
                        : Math.abs(edge.x + edge.width - background.width) <= 1 ? "right" : ""
                    if (side) sides[side] = {
                        x: edge.mapToItem(inbox.contentItem, 0, 0).x,
                        top: edge.mapToItem(inbox.contentItem, 0, 0).y,
                        bottom: edge.mapToItem(inbox.contentItem, 0, edge.height).y
                    }
                }
                if (edge.height === 1 && edge.width === background.width) {
                    if (Math.abs(edge.y) <= 1) ++topEdges
                    if (Math.abs(edge.y + edge.height - background.height) <= 1) {
                        ++bottomEdges
                        frameBottom = edge.mapToItem(inbox.contentItem, 0, edge.height).y
                    }
                }
            }
            geometry.push({id: row.cardId, top: top, bottom: bottom, sides: sides,
                topEdges: topEdges, bottomEdges: bottomEdges, borderWidth: background.border.width})
            if (row.card.important) {
                check(background.visible && background.border.width > 0 && background.radius > 0,
                    row.cardId + ": important member must paint its own standalone rounded border")
                check(!sides.left && !sides.right && topEdges === 0 && bottomEdges === 0,
                    row.cardId + ": important member must not paint bundle-frame edges")
                check(top > lastMemberBottom,
                    row.cardId + ": important standalone border must be separated from the ordinary frame")
                continue
            }
            if (i === 0) frameTop = top
            lastMemberBottom = bottom
            for (const side of ["left", "right"]) {
                const edge = sides[side]
                check(edge && Math.abs(edge.top - top) <= 1 && Math.abs(edge.bottom - bottom) <= 1,
                    row.cardId + ": painted " + side + " border must span the full member height")
                if (edge && previousSides && previousSides[side]) {
                    check(Math.abs(edge.x - previousSides[side].x) <= 1
                            && Math.abs(edge.top - previousSides[side].bottom) <= 1,
                        row.cardId + ": painted " + side + " border must continue without a gap from the preceding member")
                }
            }
            check(topEdges === (i === 0 ? 1 : 0),
                row.cardId + ": only the first ordinary member may paint the frame's top edge")
            check(bottomEdges === (i === ordinaryCount - 1 ? 1 : 0),
                row.cardId + ": only the last ordinary member may paint a closing edge")
            check(background.border.width === 0,
                row.cardId + ": expanded ordinary members must not paint a standalone border, including focus outlines")
            previousSides = sides
        }
        check(Math.abs(frameBottom - lastMemberBottom) <= 1,
            "The painted bundle frame must close at the last ordinary member's bottom")
        console.log("BUNDLE_FRAME_GEOMETRY", JSON.stringify({count: data.count, width: data.width,
            frameTop: frameTop, frameBottom: frameBottom, lastMemberBottom: lastMemberBottom,
            rows: geometry, failures: failures}))
        verify(failures.length === 0, failures.join("\n"))
        inbox.close()
    }
    function test_archiveToastLeavesNoHeaderGap_data() {
        return [{tag: "520", width: 520, height: 900}, {tag: "598", width: 598, height: 1200}]
    }
    function test_archiveToastLeavesNoHeaderGap(data) {
        store.online = false
        store.setBundleArchiveUndoDurationForTest(8000)
        const bundle = "topic:archive-gap-" + data.tag
        const leaderId = "gap-leader-" + data.tag
        const memberId = "gap-member-" + data.tag
        const survivorId = "gap-survivor-" + data.tag
        verify(store.applyRemoteCards({now: [
            card(leaderId, bundle, "now", {state: "open"}),
            card(memberId, bundle, "now", {state: "open"}),
            card(survivorId, "", "now", {state: "open"})
        ], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: data.width, height: data.height
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const archive = findChild(list.itemAtIndex(0), "archiveBundle-" + leaderId)
        verify(archive && archive.visible)
        mouseClick(archive, archive.width / 2, archive.height / 2)
        tryCompare(list, "count", 1)
        const toast = findChild(inbox, "bundleArchiveUndoBar")
        verify(toast && toast.visible, "Archive toast must be showing during the geometry check")
        wait(200)
        list.forceLayout()
        const firstCard = list.itemAtIndex(0)
        compare(firstCard.cardId, survivorId)
        const firstCardTop = firstCard.mapToItem(inbox.contentItem, 0, 0).y
        const headerBottom = inbox.header.mapToItem(inbox.contentItem, 0, inbox.header.height).y
        console.log("ARCHIVE_HEADER_GEOMETRY", JSON.stringify({width: data.width,
            firstCardTop: firstCardTop, headerBottom: headerBottom,
            topMargin: list.topMargin, toastVisible: toast.visible}))
        compare(list.topMargin, inbox.edgeSpacing, "Keep the normal ListView breathing room")
        compare(firstCardTop, headerBottom + list.topMargin, "Archive toast must not add an extra gap below the header")
        inbox.close()
    }
    function test_archiveBundleUndoExpiresFromTokenDeadline() {
        const bundle = "sender:expiry@example.test"
        const ids = ["expiry-leader", "expiry-member"]
        verify(store.applyRemoteCards({now: [card(ids[0], bundle, "now", {state: "open"}), card(ids[1], bundle, "now", {state: "open"})], later: [], missed: []}))
        store.setBundleArchiveUndoDurationForTest(1200)
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 520, height: 900
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 30000
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const archive = findChild(list.itemAtIndex(0), "archiveBundle-" + ids[0])
        mouseClick(archive, archive.width / 2, archive.height / 2)
        tryCompare(store, "bundleArchiveUndoActive", true)
        const bar = findChild(inbox, "bundleArchiveUndoBar")
        verify(bar && bar.visible)
        tryCompare(store, "bundleArchiveUndoActive", false, 3000)
        tryVerify(function() { return inbox.bundleArchiveStatus.length > 0 })
        compare(inbox.undoBundleArchive(), false, "Undo must reject the expired CardStore token")
        compare(findChild(bar, "bundleArchiveUndoMessage").text, inbox.bundleArchiveStatus)
        store.setBundleArchiveUndoDurationForTest(8000)
        inbox.close()
    }
    function test_archiveBundleUndo() {
        const bundle = "sender:undo@example.test"
        const ids = ["undo-leader", "undo-member"]
        verify(store.applyRemoteCards({now: [
            card(ids[0], bundle, "now", {state: "open"}),
            card("undo-unrelated", "", "now", {state: "open"}),
            card(ids[1], bundle, "now", {state: "open"})
        ], later: [], missed: []}))
        const before = store.cardIds()
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 520, height: 900
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 30000
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const archive = findChild(list.itemAtIndex(0), "archiveBundle-" + ids[0])
        verify(archive && archive.visible)
        mouseClick(archive, archive.width / 2, archive.height / 2)
        tryVerify(function() { return store.cardIds().indexOf(ids[0]) < 0 && store.cardIds().indexOf(ids[1]) < 0 })
        compare(store.cardIds(), ["undo-unrelated"])
        const bar = findChild(inbox, "bundleArchiveUndoBar")
        verify(bar && bar.visible)
        verify(/\b2\b/.test(findChild(bar, "bundleArchiveUndoMessage").text), "archive feedback must identify both affected messages")
        const undo = findChild(bar, "bundleArchiveUndoButton")
        verify(undo && undo.visible && undo.width >= 48 && undo.height >= 48)
        mouseClick(undo, undo.width / 2, undo.height / 2)
        tryCompare(inbox, "bundleArchiveStatus", "")
        compare(store.cardIds(), before, "Undo restores every bundle member to its original order")
        verify(!bar.visible, "Undo hides the inline bar")
        inbox.close()
    }
    function test_nonMailArchiveHasNoFalseUndoAndNeighborRemains() {
        const id = "41111111-1111-4111-8111-111111111111"
        const neighbor = "41111111-1111-4111-8111-111111111112"
        verify(store.applyRemoteCards({now: [
            {id: id, title: "Synthetic task", source: "manual", section: "now", state: "open",
             bundle_id: "", pinned_rank: null, important: false, timed: false, summary: "Task summary"},
            {id: neighbor, title: "Synthetic neighbor", source: "manual", section: "now", state: "open",
             bundle_id: "", pinned_rank: null, important: false, timed: false, summary: "Neighbor summary"}
        ], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 412, height: 915
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(1) !== null })
        const done = findChild(list.itemAtIndex(0), "doneButton-" + id)
        verify(done && done.visible && done.enabled)
        mouseClick(done)
        tryVerify(function() { return store.cardIds().indexOf(id) < 0 })
        compare(store.cardIds(), [neighbor])
        verify(!store.bundleArchiveUndoActive)
        verify(!inbox.archiveUndoAvailable)
        compare(inbox.undoBundleArchive(), false)
        compare(store.cardIds(), [neighbor], "non-mail action must not offer fake restoration")
        inbox.close()
    }
    function test_archiveBundleRejectionShowsError() {
        const bundle = "sender:reject@example.test"
        const ids = ["reject-leader", "reject-member"]
        verify(store.applyRemoteCards({now: [card(ids[0], bundle, "now", {state: "open"}), card(ids[1], bundle, "now", {state: "open"})], later: [], missed: []}))
        store.online = true
        store.setBundleArchiveUndoDurationForTest(600)
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 520, height: 900
        })
        verify(inbox)
        const rejectedCards = []
        const stub = function(opId, path, body) {
            rejectedCards.push(body.card_id)
            store.reportPostResult(opId, 422, {})
        }
        store.requestPost.connect(stub)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 30000
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const archive = findChild(list.itemAtIndex(0), "archiveBundle-" + ids[0])
        verify(archive && archive.visible)
        mouseClick(archive, archive.width / 2, archive.height / 2)
        tryVerify(function() { return rejectedCards.indexOf(ids[0]) >= 0 && rejectedCards.indexOf(ids[1]) >= 0 }, 5000, "Stub server must reject both archive POSTs with 422")
        tryVerify(function() { return inbox.bundleArchiveStatus.indexOf("422") >= 0 }, 5000)
        const bar = findChild(inbox, "bundleArchiveUndoBar")
        verify(bar && bar.visible, "Rejected archive must keep the error status visible")
        compare(findChild(bar, "bundleArchiveUndoMessage").text, inbox.bundleArchiveStatus)
        tryVerify(function() { return store.cardIds().indexOf(ids[0]) >= 0 && store.cardIds().indexOf(ids[1]) >= 0 }, 5000, "Rejected ops must restore both bundle members")
        store.requestPost.disconnect(stub)
        store.online = false
        tryVerify(function() { return !store.bundleArchiveUndoActive }, 5000, "Rejected bundle entry must expire before later tests")
        store.setBundleArchiveUndoDurationForTest(8000)
        inbox.close()
    }

    function test_gmailReconnectMessage() {
        const ids = ["expired-first", "expired-second"]
        const bundle = "expired-bundle"
        api.accounts = [{id: "expired", address: "darafei@maumap.com", status: "reauth_required"}]
        verify(store.applyRemoteCards({now: ids.map(id => card(id, bundle, "now", {state: "open"})), later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 1440, height: 900
        })
        verify(inbox)
        let notifications = 0
        const failed = function() { ++notifications }
        store.operationFailed.connect(failed)
        const stub = function(opId, path, body) {
            if (ids.indexOf(body.card_id) >= 0)
                store.reportPostResult(opId, 422, {code: "gmail_reconnect_required", account_address: "darafei@maumap.com"})
            else
                store.reportPostResult(opId, 200, {ok: true})
        }
        store.requestPost.connect(stub)
        store.online = true
        verify(inbox.archiveBundle(bundle))
        tryCompare(store, "pendingOps", 0, 5000)
        compare(notifications, 1)
        compare(store.cardIds().length, 2)
        verify(inbox.reconnectStatus.indexOf(api.accounts[0].address) >= 0, "reconnect feedback must identify the failed account")
        const reconnectStatus = inbox.reconnectStatus
        const bar = findChild(inbox, "reconnectBanner")
        verify(bar.visible)
        compare(findChild(bar, "reconnectBannerMessage").text, inbox.reconnectStatus)
        capture(inbox, "gmail-reconnect-toast-1440x900")
        inbox.showOperationFailure("single", 422, {code: "gmail_reconnect_required", account_address: "other@example.test"})
        compare(inbox.reconnectStatus, reconnectStatus, "another failure must not replace the persisted failed-account identity")
        inbox.showOperationFailure("generic", 422, {})
        verify(inbox.bundleArchiveStatus.indexOf("422") >= 0)
        store.requestPost.disconnect(stub)
        store.operationFailed.disconnect(failed)
        store.online = false
        api.accounts = []
        inbox.close()
    }

    function test_cardKeyboardFocusAndDeleteArchivesFocusedCard() {
        const id = "11111111-1111-4111-8111-000000000099"
        verify(store.applyRemoteCards({now: [card(id, "", "now", {state: "open", has_body: true})], later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 700, height: 900
        })
        verify(inbox)
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const row = list.itemAtIndex(0)
        const cardOpen = findChild(row, "openCard-" + id)
        verify(cardOpen && cardOpen.activeFocusOnTab)
        cardOpen.forceActiveFocus()
        tryVerify(function() { return cardOpen.activeFocus })
        const frame = findChild(row, "inboxCard")
        verify(frame)
        tryVerify(function() { return frame.background.border.width === 2 }, 5000,
                  "Keyboard focus must have a visible card ring")
        verify(cardOpen.activeFocus, "Focused card remains keyboard-active")
        keyClick(Qt.Key_Delete)
        tryVerify(function() { return store.cardIds().indexOf(id) < 0 }, 5000,
                  "Delete archives the keyboard-focused card")
        inbox.close()
    }
    function test_scrollThenExpandKeepsHeaderInView() {
        const bundle = "sender:messages-noreply@linkedin.com"
        const cards = []
        for (let i = 0; i < 12; ++i) cards.push(card("filler-" + i, "", "now", {title: "Filler " + i}))
        cards.push(card("scroll-leader", bundle, "now"))
        cards.push(card("scroll-member-1", bundle, "now"))
        cards.push(card("scroll-member-2", bundle, "now"))
        for (let i = 0; i < 12; ++i) cards.push(card("tail-" + i, "", "now", {title: "Tail " + i}))
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 1440, height: 1000
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        // ListView only instantiates delegates near its viewport.
        list.positionViewAtIndex(12, ListView.End)
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(12) !== null })
        list.positionViewAtIndex(12, ListView.End)
        list.contentY += 40
        wait(200)
        const row = list.itemAtIndex(12)
        compare(row.cardId, "scroll-leader")
        const toggle = findChild(row, "bundleToggle-scroll-leader")
        verify(toggle)
        const before = toggle.mapToItem(list, 0, 0).y
        console.log("HEADER_Y_BEFORE", before, "listHeight", list.height, "toggleHeight", toggle.height)
        verify(before >= 0 && before + toggle.height <= list.height, "Header must start inside the viewport")
        capture(inbox, "before-expand-1440")
        mouseClick(toggle, 12, toggle.height / 2)
        tryCompare(inbox.expandedBundles, bundle, true)
        wait(200)
        list.forceLayout()
        capture(inbox, "after-expand-1440")
        const after = toggle.mapToItem(list, 0, 0).y
        console.log("HEADER_Y_AFTER", after)
        verify(after >= 0 && after + toggle.height <= list.height, "Header must remain in viewport after expand")
        // Acceptance is literally "same y"; the proposed one-delegate-height
        // tolerance was unsourced and hides real drift, so allow rounding only.
        verify(Math.abs(after - before) <= 2,
               "Header must keep its viewport y after expand (before " + before + ", after " + after + ")")
        inbox.close()
    }
    }
