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
        mouseClick(toggle, 12, toggle.height / 2)
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
}
