import QtQuick
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "MailCardOpening"
    when: windowShown
    width: 700
    height: 800
    property url pagesDir: Qt.resolvedUrl("../pages/")

    ListModel {
        id: store
        property bool online: false
        property var cached: ({})
        property var operations: []
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) {}
        function cardIds() { return Array.from({length: count}, (_, i) => get(i).cardId) }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function refresh() {}
        function enqueueOp(cardId, operation, args) {
            operations = operations.concat([{cardId: cardId, type: operation, args: args}])
            if (operation === "archive" || operation === "done" || operation === "snooze") {
                const row = cardIds().indexOf(cardId)
                if (row >= 0) remove(row)
            }
            return "fixture-operation"
        }
        function dismiss(cardId) { return enqueueOp(cardId, "done", {}) }
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

    function mailId(index) {
        return "43111111-1111-4111-8111-" + String(index + 1).padStart(12, "0")
    }
    function positionRow(list, index) {
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null })
        wait(200)
        list.positionViewAtIndex(index, ListView.Beginning)
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(index) !== null })
        wait(200)
        list.positionViewAtIndex(index, ListView.Beginning)
        wait(100)
    }
    function populateStore(source) {
        store.clear()
        store.operations = []
        store.cached = ({[mailId(8)]: {html: "<p>Hello <b>reader</b></p>"}})
        for (let i = 0; i < 40; ++i) {
            const id = mailId(i)
            const title = i < 14 ? "Message " + i : "Additional mail"
            store.append({cardId: id, title: title, section: "now", card: {
                id: id, title: title, source: source, section: "now", account_name: "fixture@example.test",
                summary: "Click this preview to read the message",
                snippet: "Click this preview to read the message", has_body: true,
                pinned_rank: null, bundle_id: "", important: false, timed: false,
                note: "", source_url: ""
            }})
        }
    }
    function init() { populateStore("mail") }

    function test_previewOpensMailAndBackPreservesScroll() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 40)
        positionRow(list, 8)
        const before = list.contentY
        verify(before > 0)
        const row = list.itemAtIndex(8)
        verify(row)
        const summary = findChild(row, "mailSnippet-" + mailId(8))
        verify(summary && summary.visible)
        mouseClick(summary, summary.width / 2, summary.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(inbox.mailDetailPage, "html", "<p>Hello <b>reader</b></p>")
        compare(store.operations, [])
        tryCompare(stack, "busy", false)
        // StackView.Immediate updates depth before hiding the inbox header has
        // polished the reader's app-bar coordinates. busy === false is not a frame.
        verify(waitForPolish(inbox), "Reader layout must settle before clicking Back")
        verify(waitForRendering(inbox.mailDetailPage), "The real reader must render before clicking Back")
        const back = findChild(inbox.mailDetailPage, "backToInbox")
        verify(back)
        verify(back.width >= 48 && back.height >= 48)
        mouseClick(back)
        tryCompare(stack, "depth", 1)
        tryCompare(stack, "busy", false)
        compare(list.contentY, before)
        compare(list.itemAtIndex(8).cardId, mailId(8))
        inbox.close()
    }
    function test_escapeAndMouseBackRestoreInboxPosition_data() {
        return [{tag: "escape", action: "escape"},
                {tag: "mouse-back", action: "back"}]
    }

    function test_escapeAndMouseBackRestoreInboxPosition(row) {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 40)
        positionRow(list, 8)
        const before = list.contentY - list.originY
        verify(before > 0)

        const preview = findChild(list.itemAtIndex(8), "mailSnippet-" + mailId(8))
        verify(preview)
        mouseClick(preview, preview.width / 2, preview.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(inbox.mailDetailPage, "html", "<p>Hello <b>reader</b></p>")

        if (row.action === "escape") {
            inbox.mailDetailPage.forceActiveFocus()
            keyClick(Qt.Key_Escape)
        } else {
            mouseClick(inbox.contentItem, inbox.width / 2, inbox.height / 2, Qt.BackButton)
        }
        tryCompare(stack, "depth", 1)
        tryCompare(list, "contentY", list.originY + before)
        compare(list.itemAtIndex(8).cardId, mailId(8))
        compare(store.operations, [])
        inbox.close()
    }
    function test_narrowDesktopBackClosesReaderWithoutArchive() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 598, height: 900
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 40)
        positionRow(list, 0)
        const row = findChild(list.itemAtIndex(0), "openCard-" + mailId(0))
        verify(row)
        mouseClick(row)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        compare(inbox.readerOpen, true)
        mouseClick(inbox.contentItem, inbox.width / 2, inbox.height / 2, Qt.BackButton)
        tryCompare(stack, "depth", 1)
        tryCompare(inbox, "readerOpen", false)
        compare(inbox.selectedCardId, mailId(0))
        compare(store.operations, [])
        inbox.close()
    }

    function test_keyboardOpensAndClosesMail() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        positionRow(list, 0)
        const card = findChild(list.itemAtIndex(0), "openCard-" + mailId(0))
        verify(card)
        list.forceActiveFocus()
        for (let i = 0; i < 20 && !card.activeFocus; ++i)
            keyClick(Qt.Key_Tab)
        tryVerify(function() { return card.activeFocus })
        keyClick(Qt.Key_Return)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        keyClick(Qt.Key_Escape)
        tryCompare(stack, "depth", 1)
        compare(store.operations, [])
        inbox.close()
    }


    function test_actionReturnsToSameInboxOffset_data() {
        return [{tag: "archive-button", action: "archive"},
                {tag: "archive-key", action: "key"},
                {tag: "snooze", action: "snooze"},
                {tag: "done-body", action: "done"}]
    }

    function test_actionReturnsToSameInboxOffset(row) {
        if (row.action === "done")
            populateStore("manual")
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 40)
        positionRow(list, 6)
        const before = list.contentY - list.originY
        const anchorY = list.itemAtIndex(6).mapToItem(list, 0, 0).y
        verify(before > 0)
        const preview = findChild(list.itemAtIndex(8), "openCard-" + mailId(8))
        verify(preview)
        mouseClick(preview, preview.width / 2, preview.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        tryCompare(stack, "busy", false)
        verify(waitForPolish(inbox), "Reader layout must settle before clicking a primary action")
        verify(waitForRendering(inbox.mailDetailPage), "The real reader must render before pointer delivery")
        const archive = findChild(inbox.mailDetailPage, "archiveMail")
        verify(archive && archive.visible, "Archive must be available while reading")
        if (row.action === "key") {
            archive.forceActiveFocus()
            keyClick(Qt.Key_E)
        } else if (row.action === "snooze") {
            const actions = findChild(inbox.mailDetailPage, "mailActions")
            verify(actions)
            actions.chooseSnoozeDateTime()
            const dateTime = findChild(actions, "snoozeDateTime")
            tryVerify(function() { return dateTime.activeFocus })
            keyClick(Qt.Key_E)
            compare(store.operations, [], "Typing in a dialog must not archive")
            compare(stack.depth, 2)
            const tomorrow = new Date(Date.now() + 86400000)
            verify(actions.captureSnooze(actions.localDateTime(tomorrow)))
        } else {
            mouseClick(archive)
        }
        tryCompare(stack, "depth", 1)
        tryCompare(stack, "busy", false)
        compare(list.contentY - list.originY, before)
        compare(Math.round(list.itemAtIndex(6).mapToItem(list, 0, 0).y), Math.round(anchorY))
        verify(store.cardIds().indexOf(mailId(8)) < 0)
        compare(store.operations.length, 1)
        compare(store.operations[0].type, row.action === "snooze" || row.action === "done" ? row.action : "archive")
        compare(store.operations[0].cardId, mailId(8))
        inbox.close()
    }

    function test_backRestoresExactCardPosition_data() {
        return [{tag: "w1440", width: 1440},
                {tag: "w598", width: 598}]
    }
    function test_backRestoresExactCardPosition(row) {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: row.width, height: 900
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        tryCompare(list, "count", 40)
        positionRow(list, 6)
        verify(list.contentY - list.originY > 0)
        // The reflow inserts a row above the opened card, so index lookups are
        // stale afterwards; track the card by id throughout.
        function rowOf(cardId) {
            for (let i = 0; i < list.count; ++i) {
                const item = list.itemAtIndex(i)
                if (item && item.cardId === cardId) return item
            }
            return null
        }
        const openedCard = rowOf(mailId(8))
        verify(openedCard)
        const beforeY = openedCard.mapToItem(list, 0, 0).y
        const preview = findChild(openedCard, "openCard-" + mailId(8))
        verify(preview)
        mouseClick(preview, preview.width / 2, preview.height / 2)
        const stack = findChild(inbox, "pageStack")
        tryCompare(inbox, "readerOpen", true)
        compare(stack.depth, inbox.desktopReader ? 1 : 2)
        tryCompare(stack, "busy", false)
        // The inbox reflows while the mail is open: a new row arrives above the
        // opened card (a refresh can also re-estimate row heights), which
        // invalidates the raw contentY that the old Back path restored.
        store.insert(0, {cardId: mailId(40), title: "Message reflow", section: "now", card: {
            id: mailId(40), title: "Message reflow", source: "mail", section: "now", account_name: "fixture@example.test",
            summary: "Arrived while reading", snippet: "Arrived while reading", has_body: true,
            pinned_rank: null, bundle_id: "", important: false, timed: false,
            note: "", source_url: ""
        }})
        tryCompare(list, "count", 41, 2000, "the inbox must relayout while the mail is open")
        const back = findChild(inbox.mailDetailPage, "backToInbox")
        verify(back)
        mouseClick(back)
        tryCompare(stack, "depth", 1)
        tryCompare(inbox, "readerOpen", false)
        tryCompare(stack, "busy", false)
        wait(200)
        list.forceLayout()
        const restoredCard = rowOf(mailId(8))
        verify(restoredCard)
        const afterY = restoredCard.mapToItem(list, 0, 0).y
        verify(afterY === beforeY,
               "Back must restore the opened card within 0 px: before=" + beforeY + " after=" + afterY)
        compare(store.operations, [])
        inbox.close()
    }

    function test_bodylessCardDoesNotOpenEmptyDetail() {
        store.clear()
        store.append({cardId: mailId(41), title: "Plain task", section: "now", card: {
            id: mailId(41), title: "Plain task", source: "manual", section: "now", has_body: false,
            pinned_rank: null, bundle_id: "", important: false, timed: false,
            summary: "", note: "", account_name: "", source_url: ""
        }})
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        list.forceLayout()
        const row = list.itemAtIndex(0)
        verify(row)
        mouseClick(row, row.width / 2, row.height / 2)
        compare(findChild(inbox, "pageStack").depth, 1)
        inbox.close()
    }
    function test_desktopSplitSelectionResizeNarrowBackAndArchiveClosure() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 1480, height: 900
        })
        verify(inbox)
        inbox.show()
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        const split = findChild(inbox, "inboxSplit")
        const rail = findChild(inbox, "desktopNavigation")
        const stack = findChild(inbox, "pageStack")
        const reader = findChild(inbox, "desktopReaderHost")
        tryVerify(function() { list.forceLayout(); return list.itemAtIndex(0) !== null && reader.visible })
        verify(split.orientation === Qt.Horizontal)
        verify(rail.visible && reader.visible && reader.width >= 520)
        positionRow(list, 8)
        const anchorY = list.itemAtIndex(8).mapToItem(list, 0, 0).y
        const selected = findChild(list.itemAtIndex(8), "openCard-" + mailId(8))
        mouseClick(selected)
        tryCompare(inbox, "selectedCardId", mailId(8))
        tryVerify(function() { return inbox.mailDetailPage && inbox.mailDetailPage.visible })
        compare(store.operations, [])
        const retainedReader = inbox.mailDetailPage
        compare(retainedReader.embeddedMode, true)
        verify(typeof retainedReader.requestClose === "function")
        const selectedFrame = findChild(list.itemAtIndex(8), "inboxCard")
        verify(selectedFrame.background.border.width >= 2,
               "selected cached card must be visibly highlighted beside reader")
        inbox.width = 1320
        wait(150)
        verify(reader.visible)
        compare(Math.round(list.itemAtIndex(8).mapToItem(list, 0, 0).y), Math.round(anchorY))
        inbox.width = 1180
        tryVerify(function() { return !reader.visible && stack.depth === 2 })
        compare(inbox.mailDetailPage, retainedReader)
        compare(retainedReader.embeddedMode, false)
        const back = findChild(retainedReader, "backToInbox")
        verify(back)
        mouseClick(back)
        tryCompare(stack, "depth", 1)
        compare(inbox.selectedCardId, mailId(8))
        compare(Math.round(list.itemAtIndex(8).mapToItem(list, 0, 0).y), Math.round(anchorY))
        compare(store.operations, [])
        inbox.width = 1480
        tryVerify(function() { return reader.visible && !inbox.mailDetailPage.visible })
        compare(inbox.readerOpen, false, "Back must stay closed when the desktop pane returns")
        mouseClick(findChild(list.itemAtIndex(8), "openCard-" + mailId(8)))
        tryCompare(inbox, "readerOpen", true)
        compare(inbox.mailDetailPage, retainedReader)
        compare(retainedReader.embeddedMode, true)
        mouseClick(findChild(retainedReader, "backToInbox"))
        tryCompare(inbox, "readerOpen", false)
        tryCompare(retainedReader, "visible", false)
        compare(inbox.selectedCardId, mailId(8))
        compare(store.operations, [], "embedded Back callback must close without archiving")
        mouseClick(findChild(list.itemAtIndex(8), "openCard-" + mailId(8)))
        tryCompare(inbox, "readerOpen", true)
        compare(inbox.mailDetailPage, retainedReader)
        store.remove(store.cardIds().indexOf(mailId(13)))
        tryCompare(list, "count", 39)
        compare(inbox.readerOpen, true, "removing a different card must retain the selected reader")
        compare(retainedReader.visible, true)
        compare(store.operations, [])
        // A remote archive/snapshot removes a selected card from the store;
        // it must not be implemented by invoking the local archive action.
        store.remove(store.cardIds().indexOf(mailId(8)))
        tryCompare(list, "count", 38)
        tryCompare(inbox, "readerOpen", false)
        tryCompare(retainedReader, "visible", false)
        compare(inbox.mailDetailPage, retainedReader)
        compare(stack.depth, 1)
        compare(store.operations, [], "external selected-card removal must not enqueue archive")
        inbox.close()
    }
    function test_altLeftRoutesBackWhenFocusIsOutsideStack() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        positionRow(list, 8)
        const opener = findChild(list.itemAtIndex(8), "openCard-" + mailId(8))
        mouseClick(opener)
        const stack = findChild(inbox, "pageStack")
        tryCompare(stack, "depth", 2)
        findChild(inbox, "addCardButton").forceActiveFocus()
        tryVerify(function() { return findChild(inbox, "addCardButton").activeFocus })
        keyClick(Qt.Key_Left, Qt.AltModifier)
        tryCompare(stack, "depth", 1)
        compare(store.operations, [])
        inbox.close()
    }
    function test_searchFiltersWithoutReorderingAndClearRestoresAnchor() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 700, height: 800
        })
        verify(inbox)
        inbox.show()
        const list = findChild(inbox, "inboxList")
        const ids = store.cardIds()
        positionRow(list, 8)
        const anchor = list.itemAtIndex(8).mapToItem(list, 0, 0).y
        mouseClick(findChild(inbox, "toggleSearch"))
        const search = findChild(inbox, "inboxSearch")
        verify(search && findChild(inbox, "inboxSearchRow").visible)
        search.text = "Message 3"
        search.textEdited()
        tryVerify(function() { list.forceLayout(); return list.count > 0 && list.count < ids.length })
        const matches = []
        for (let i = 0; i < list.count; ++i) matches.push(list.itemAtIndex(i).cardId)
        compare(matches, [mailId(3), mailId(13)])
        compare(store.cardIds(), ids, "search must not mutate underlying CardStore order")
        const clear = findChild(inbox, "clearInboxSearch")
        verify(clear.enabled)
        mouseClick(clear)
        tryCompare(list, "count", ids.length)
        compare(store.cardIds(), ids)
        compare(Math.round(list.itemAtIndex(8).mapToItem(list, 0, 0).y), Math.round(anchor))
        inbox.close()
    }
    function test_realPopupsProtectRetainedReaderAndArchiveShortcut_data() {
        return ["drawer", "header", "note", "create", "actions", "snooze"].map(
            popup => ({tag: popup, popup: popup}))
    }

    function test_realPopupsProtectRetainedReaderAndArchiveShortcut(data) {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules,
            width: 1440, height: 900
        })
        verify(inbox)
        inbox.show()
        inbox.requestActivate()
        tryCompare(inbox, "active", true)
        const list = findChild(inbox, "inboxList")
        positionRow(list, 8)
        mouseClick(findChild(list.itemAtIndex(8), "openCard-" + mailId(8)))
        tryCompare(inbox, "readerOpen", true)
        const reader = inbox.mailDetailPage
        const actions = findChild(reader, "mailActions")
        let body = null
        tryVerify(function() { body = findChild(reader, "body"); return body !== null && body.visible })
        const document = reader.html
        verify(document.length > 0)
        compare(store.operations, [])
        // The ambient Controls overlay exists even with no popup. The actual
        // document must be readable before opening any of these real controls.
        verify(body.parent.visible)
        let popup = null
        if (data.popup === "drawer") {
            popup = findChild(inbox, "navigationDrawer")
            popup.open()
        } else if (data.popup === "header") {
            mouseClick(findChild(inbox, "inboxCommands"))
            popup = findChild(inbox, "headerMenu")
        } else if (data.popup === "note") {
            actions.noteRequested()
        } else if (data.popup === "create") {
            mouseClick(findChild(inbox, "addCardButton"))
            popup = findChild(inbox, "createDialog")
        } else if (data.popup === "actions") {
            mouseClick(actions)
            popup = actions.actionSheetItem()
        } else {
            actions.chooseSnoozeDateTime()
            popup = actions.snoozeDialogItem()
        }
        if (data.popup === "note")
            tryCompare(inbox, "actualPopupsVisible", true)
        else
            tryCompare(popup, "visible", true)
        // A native Android WebView must yield to the Qt popup, not unload.
        // Desktop WebEngine stays in the scene underneath the modal surface.
        tryCompare(body.parent, "visible", Qt.platform.os !== "android")
        keyClick(Qt.Key_E)
        compare(store.operations, [], "E must not archive while a real popup is open")
        compare(inbox.readerOpen, true)
        compare(inbox.mailDetailPage, reader)
        compare(findChild(reader, "body"), body, "Popup must not replace the document")
        compare(reader.html, document)
        if (data.popup === "note")
            mouseClick(findChild(inbox, "noteCancelButton"))
        else
            popup.close()
        tryCompare(inbox, "actualPopupsVisible", false)
        tryCompare(reader, "actualPopupsVisible", false)
        tryVerify(function() { return body.parent.visible && body.visible })
        compare(inbox.mailDetailPage, reader)
        compare(findChild(reader, "body"), body)
        compare(reader.html, document)
        findChild(reader, "archiveMail").forceActiveFocus()
        keyClick(Qt.Key_E)
        tryCompare(inbox, "readerOpen", false)
        compare(store.operations.length, 1, "E must resume after the popup closes")
        compare(store.operations[0].type, "archive")
        compare(store.operations[0].cardId, mailId(8))
        inbox.close()
    }
}
