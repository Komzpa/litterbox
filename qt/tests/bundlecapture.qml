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
    function mixed(inbox, list, width) {
        // Additive demonstration controls live only in this disposable store.
        verify(bundleStore.createCard("Review the release plan", "A pinned card stays standalone."))
        const pinned = bundleStore.cardIds()[0]
        verify(bundleStore.enqueueOp(pinned, "pin", {}))
        verify(bundleStore.createCard("Confirm the meeting time", ""))
        verify(bundleStore.createCard("Private journal note", "Normal journal controls stay unchanged.", "journal"))
        list.positionViewAtBeginning()
        wait(400)
        capture(inbox, "mixed-" + width)
    }
    function capture(inbox, name) {
        grabImage(inbox.contentItem.parent).save(bundleProofDirectory + "/" + name + ".png")
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
    function test_matrix_data() { return [{tag: "1440", width: 1440, height: 1000}, {tag: "598", width: 598, height: 1200}] }
    function test_matrix(data) {
        verify(bundleCacheLoaded && bundleProofDirectory, "Use a read-only copied real cache and an output directory")
        const cached = bundleCachedCards.map(card => Object.assign({}, card))
        const leader = cached.find(card => card.bundle_title === "sender:messages-noreply@linkedin.com" && card.bundle_leader === true)
        verify(leader)
        const ids = cached.filter(card => card.bundle_id === leader.bundle_id && card.bundle_leader !== undefined).map(card => card.id)
        compare(ids.length, 8)
        verify(bundleStore.applyRemoteCards({now: cached.filter(card => card.section === "now"),
            later: cached.filter(card => card.section === "later"), missed: cached.filter(card => card.section === "missed")}))
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: bundleStore, api: api, updater: updater, timeRules: timeRules,
            width: data.width, height: data.height, title: "Bundle proof " + data.width
        })
        verify(inbox)
        const list = findChild(inbox, "inboxList")
        list.cacheBuffer = 50000
        tryVerify(function() { list.forceLayout(); return rows(list, ids).length === 8 }, 15000)
        list.positionViewAtBeginning()
        wait(400)
        const row = list.itemAtIndex(0)
        compare(row.cardId, leader.id)
        const toggle = findChild(row, "bundleToggle-" + leader.id)
        verify(toggle)
        const archive = findChild(row, "archiveBundle-" + leader.id)
        console.log("BUNDLE_CONTRACT", JSON.stringify({width: data.width, title: toggle.text,
            archiveVisible: !!archive && archive.visible, sourceIds: ids}))
        capture(inbox, "collapsed-" + data.width)
        input(toggle, "chevron-click", 12)
        if (archive) tryCompare(inbox.expandedBundles, leader.bundle_id, true)
        list.forceLayout()
        wait(250)
        const expanded = rows(list, ids)
        console.log("BUNDLE_EXPANDED", JSON.stringify({width: data.width, rows: expanded}))
        for (let i = 1; i < expanded.length && inbox.expandedBundles[leader.bundle_id]; ++i) {
            compare(expanded[i].index, expanded[0].index + i)
            verify(Math.abs(expanded[i].y - expanded[i - 1].y - expanded[i - 1].height) <= 1)
        }
        capture(inbox, "expanded-" + data.width)
        if (!archive) {
            // The negative control records the old surface even when its pointer
            // path fails; do not abort before collecting the neighboring view.
            input(toggle, "chevron-collapse", 12)
            mixed(inbox, list, data.width)
            inbox.close()
            return
        }
        input(toggle, "chevron-collapse", 12)
        tryCompare(inbox.expandedBundles, leader.bundle_id, false)
        list.forceLayout()
        input(toggle, "title-click", Math.min(toggle.width - 12, 100))
        tryCompare(inbox.expandedBundles, leader.bundle_id, true)
        console.log("BUNDLE_TITLE_CLICK", JSON.stringify({width: data.width, expanded: true, focused: toggle.activeFocus}))
        input(toggle, "Return")
        tryCompare(inbox.expandedBundles, leader.bundle_id, false)
        input(toggle, "space")
        tryCompare(inbox.expandedBundles, leader.bundle_id, true)
        input(toggle, "Return")
        tryCompare(inbox.expandedBundles, leader.bundle_id, false)
        wait(200)
        console.log("BUNDLE_COLLAPSED", JSON.stringify({width: data.width, rows: rows(list, ids)}))
        if (archive && archive.visible) {
            input(archive, "archive-click")
            const dialog = findChild(row, "archiveDialog")
            tryCompare(dialog, "visible", true)
            capture(inbox, "archive-confirmation-" + data.width)
            input(findChild(dialog, "archiveCancelButton"), "archive-cancel")
            tryCompare(dialog, "visible", false)
        }
        mixed(inbox, list, data.width)
        inbox.close()
    }
}
