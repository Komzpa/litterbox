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

    function test_collapsedMembersLeaveNoHolesAndExpandInPlace() {
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
}
