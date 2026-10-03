import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "InboxLayout"
    when: windowShown
    width: 572
    height: 881

    ListModel {
        id: store
        property bool online: false
        function cardIds() { return ["bundle-0", "bundle-1", "bundle-2", "bundle-3", "standalone"] }
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

    function test_collapsedMembersLeaveNoHolesAndExpandInPlace() {
        for (let i = 0; i < 4; ++i) {
            store.append({cardId: "bundle-" + i, title: "A bundled message " + i, section: "now", card: {
                id: "bundle-" + i, title: "A bundled message " + i, source: "mail", section: "now",
                bundle_id: "github", bundle_title: "sender:notifications@github.com", bundle_leader: i === 0,
                bundle_member_count: 4, pinned_rank: null, important: false, timed: false,
                has_body: false, summary: "", note: "", account_name: "owner@example.test"
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
        mouseClick(toggle)
        tryVerify(function() { list.forceLayout(); return !list.itemAtIndex(1).visible })
        compare(next.y - leader.y - frame.y - frame.height, gap)
    }
}
