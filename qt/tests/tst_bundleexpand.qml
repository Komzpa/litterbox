import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    id: testCase
    name: "BundleExpand"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
    property var store: bundleStore
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
        return [
            {tag: "one-sender", names: ["LinkedIn", "LinkedIn"], title: "LinkedIn", preview: "LinkedIn"},
            {tag: "two-senders", names: ["LinkedIn", "LinkedIn Job Alerts", "LinkedIn"], title: "LinkedIn, LinkedIn Job Alerts", preview: "LinkedIn, LinkedIn Job Alerts"},
            {tag: "more-senders", names: ["LinkedIn", "LinkedIn Job Alerts", "LinkedIn", "Recruiting", "Careers"], title: "LinkedIn, LinkedIn Job Alerts +2", preview: "LinkedIn, LinkedIn Job Alerts +2"},
            {tag: "domain-fallback", names: [undefined, ""], title: "linkedin.com", preview: "linkedin.com"},
            {tag: "topic-title", names: ["LinkedIn", "LinkedIn Job Alerts"], bundle: "Career opportunities", title: "Career opportunities", preview: "LinkedIn, LinkedIn Job Alerts"}
        ]
    }
    function test_senderSummary(data) {
        const bundle = data.bundle || "sender:messages-noreply@linkedin.com"
        const cards = data.names.map((name, index) => card("sender-" + index, bundle, "now", {
            sender_name: name, title: index === 0 ? "Your latest career update" : "An earlier update"
        }))
        verify(store.applyRemoteCards({now: cards, later: [], missed: []}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 520, height: 900
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        const row = list.itemAtIndex(0)
        const toggle = findChild(row, "bundleToggle-sender-0")
        tryCompare(toggle, "text", data.title)
        const sender = findChild(row, "bundleAccountLabel-sender-0")
        compare(sender.text, data.preview + " · test@example.test")
        const latest = findChild(row, "bundleLatestLabel-sender-0")
        verify(latest && latest.visible && latest.height > 0, "Collapsed preview must show the latest subject")
        compare(latest.text, "Latest: Your latest career update")
        const title = findChild(row, "bundleTitleLabel-sender-0")
        verify(!title.truncated, "Distinct sender names must remain readable at 520 px")
        mouseClick(toggle, toggle.width / 2, toggle.height / 2)
        tryCompare(inbox.expandedBundles, bundle, true)
        const expandedLatest = findChild(list.itemAtIndex(0), "bundleLatestLabel-sender-0")
        verify(!expandedLatest.visible, "Expanded cards show their subjects instead of the collapsed preview")
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
            compare(archive.Accessible.name, "Archive bundle")
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
        compare(findChild(row, "bundleMemberCount-single-leader").text, "1 email")
        compare(findChild(row, "importantDisclosure-single-leader").text,
            "1 important email stays separate; archive includes all 2 unpinned emails")
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
        tryCompare(inbox, "bundleArchiveStatus", "Undo expired")
        compare(inbox.undoBundleArchive(), false, "Undo must reject the expired CardStore token")
        compare(findChild(bar, "bundleArchiveUndoMessage").text, "Undo expired")
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
        compare(findChild(bar, "bundleArchiveUndoMessage").text, "Archived 2 emails ·")
        const undo = findChild(bar, "bundleArchiveUndoButton")
        verify(undo && undo.visible && undo.width >= 48 && undo.height >= 48)
        mouseClick(undo, undo.width / 2, undo.height / 2)
        tryCompare(inbox, "bundleArchiveStatus", "")
        compare(store.cardIds(), before, "Undo restores every bundle member to its original order")
        verify(!bar.visible, "Undo hides the inline bar")
        inbox.close()
    }
    }
