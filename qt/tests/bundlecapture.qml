import QtQuick
import QtTest
import litterbox 1.0 as App

// Opt-in X11 proof, driven by bundlecapture_driver.py, never a synthetic click.
TestCase {
    id: proof
    name: "BundleCapture"
    when: windowShown
    property url pagesDir: Qt.resolvedUrl("../pages/")
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

    function input(item, action, x) {
        const point = item.mapToGlobal(x === undefined ? item.width / 2 : x, item.height / 2)
        console.log("BUNDLE_INPUT", JSON.stringify({action: action, x: Math.round(point.x), y: Math.round(point.y)}))
        wait(700)
    }
    function capture(inbox, name) {
        grabImage(inbox.contentItem.parent).save(bundleProofDirectory + "/" + name + ".png")
        console.log("BUNDLE_CAPTURE", name)
    }
    function rows(list, ids) {
        const result = []
        for (let i = 0; i < list.count; ++i) {
            const row = list.itemAtIndex(i)
            if (row && ids.indexOf(row.cardId) >= 0)
                result.push({id: row.cardId, index: i, y: row.y, height: row.height, visible: row.visible})
        }
        return result
    }
    function test_matrix_data() {
        return [{tag: "520", width: 520, height: 900}, {tag: "598", width: 598, height: 1200}, {tag: "1440", width: 1440, height: 1000}]
            .filter(viewport => viewport.width === bundleProofWidth)
    }
    function test_matrix(data) {
        verify(bundleCacheLoaded && bundleProofDirectory, "Use a read-only copied real cache and an output directory")
        console.log("BUNDLE_CACHE", JSON.stringify({path: bundleCachePath, width: data.width}))
        const cached = bundleCachedCards.map(card => Object.assign({}, card))
        const leader = cached.find(card => card.bundle_title === "sender:messages-noreply@linkedin.com" && card.bundle_leader === true)
        verify(leader)
        const ids = cached.filter(card => card.bundle_id === leader.bundle_id && card.bundle_leader !== undefined).map(card => card.id)
        compare(ids.length, 8)
        for (const card of cached) {
            if (card.bundle_id === leader.bundle_id)
                card.sender_name = ids.indexOf(card.id) >= 4 ? "LinkedIn Job Alerts" : "LinkedIn"
            else if ((card.bundle_title || "").startsWith("sender:") && card.bundle_title.endsWith("@github.com"))
                card.sender_name = "GitHub"
        }
        verify(bundleStore.applyRemoteCards({now: cached.filter(card => card.section === "now"),
            later: cached.filter(card => card.section === "later"), missed: cached.filter(card => card.section === "missed")}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules,
            width: data.width, height: data.height, title: "Bundle proof " + data.width
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 2 * data.height
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null }, 15000)
        list.positionViewAtBeginning()
        wait(400)
        const row = list.itemAtIndex(0)
        compare(row.cardId, leader.id)
        const toggle = findChild(row, "bundleToggle-" + leader.id)
        verify(toggle)
        const archive = findChild(row, "archiveBundle-" + leader.id)
        verify(archive && archive.visible)
        tryCompare(toggle, "text", "LinkedIn, LinkedIn Job Alerts")
        const account = findChild(row, "bundleAccountLabel-" + leader.id)
        const latest = findChild(row, "bundleLatestLabel-" + leader.id)
        verify(latest && latest.visible && latest.height > 0)
        compare(latest.text, "Latest: " + leader.title)
        console.log("BUNDLE_CONTRACT", JSON.stringify({width: data.width, title: toggle.text,
            senderAccount: account.text, latest: latest.text, archiveVisible: archive.visible, sourceIds: ids}))
        capture(inbox, "collapsed-" + data.width)
        input(toggle, "chevron-click", toggle.width / 2)
        tryCompare(inbox.expandedBundles, leader.bundle_id, true)
        tryVerify(function() { list.forceLayout(); return rows(list, ids).filter(row => row.visible).length === 8 }, 15000)
        list.forceLayout()
        wait(250)
        const expanded = rows(list, ids)
        compare(expanded.length, 8)
        console.log("BUNDLE_EXPANDED", JSON.stringify({width: data.width, rows: expanded}))
        for (let i = 1; i < expanded.length; ++i) {
            compare(expanded[i].index, expanded[0].index + i)
            verify(Math.abs(expanded[i].y - expanded[i - 1].y - expanded[i - 1].height) <= 1)
        }
        capture(inbox, "expanded-" + data.width)
        const expandedToggle = findChild(list.itemAtIndex(expanded[0].index), "bundleToggle-" + leader.id)
        input(expandedToggle, "title-click", Math.min(expandedToggle.width - 12, 100))
        tryCompare(inbox.expandedBundles, leader.bundle_id, false)
        tryVerify(function() { list.forceLayout(); return rows(list, ids).filter(row => row.visible).length === 1 })
        console.log("BUNDLE_COLLAPSED", JSON.stringify({width: data.width, rows: rows(list, ids)}))
        capture(inbox, "after-collapse-" + data.width)
        capture(inbox, "bundle-before-archive-" + data.width)
        input(findChild(list.itemAtIndex(0), "archiveBundle-" + leader.id), "archive-bundle-click")
        tryVerify(function() { return ids.every(id => bundleStore.cardIds().indexOf(id) < 0) }, 15000)
        const undoBar = findChild(inbox, "bundleArchiveUndoBar")
        verify(undoBar && undoBar.visible)
        const undoMessage = findChild(undoBar, "bundleArchiveUndoMessage")
        verify(undoMessage.text.startsWith("Archived ") && undoMessage.text.endsWith(" emails ·"))
        capture(inbox, "bundle-archived-" + data.width)
        const undo = findChild(undoBar, "bundleArchiveUndoButton")
        verify(undo && undo.visible && undo.width >= 48 && undo.height >= 48)
        input(undo, "bundle-undo-click")
        tryVerify(function() { return ids.every(id => bundleStore.cardIds().indexOf(id) >= 0) }, 15000)
        compare(ids, bundleStore.cardIds().filter(id => ids.indexOf(id) >= 0), "Undo restores bundle order")
        tryVerify(function() { return !undoBar.visible })
        capture(inbox, "bundle-restored-" + data.width)
        bundleStore.setBundleArchiveUndoDurationForTest(450)
        const expiryArchive = findChild(list.itemAtIndex(0), "archiveBundle-" + leader.id)
        input(expiryArchive, "bundle-expiry-archive-click")
        tryCompare(inbox, "bundleArchiveStatus", "Undo expired", 3000)
        verify(!bundleStore.bundleArchiveUndoActive)
        verify(!inbox.undoBundleArchive())
        capture(inbox, "bundle-expired-" + data.width)
        inbox.close()
    }
}
