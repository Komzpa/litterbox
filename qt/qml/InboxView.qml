import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

ApplicationWindow {
    id: window
    visible: true
    width: 520
    height: 800
    title: "Litterbox"
    property var cardStore: store
    property var captureActions: ({})
    property var cardHandles: ({})
    property string dragCardId: ""
    property int dragTargetIndex: -1
    property string updateStatus: ""
    property string updateVersion: ""


    // Drop the dragged card at the row the `=` handle was released over.
    function commitCardDrag(cardId, targetIndex) {
        return targetIndex >= 0 && store.moveCardTo(cardId, targetIndex)
    }
    function cardIdAt(row) {
        const ids = store.cardIds()
        return row >= 0 && row < ids.length ? ids[row] : ""
    }

    // The card-controls capture opens the real dialog, records a frame while it is
    // rendered open, and only then accepts it, so the visible dialog itself is
    // evidence rather than just its after-effect.
    function captureOpenCreateDialog(title, summary) {
        createTitle.text = title
        createSummary.text = summary
        createDialog.open()
        return createDialog.visible
    }
    function captureAcceptCreateDialog() {
        const oldCount = store.cardIds().length
        createDialog.accept()
        createDialog.visible = false
        return store.cardIds().length === oldCount + 1
    }
    function captureCloseCreateDialog() { createDialog.close() }
    function captureNoteDialogCardId() { return noteDialog.visible ? noteDialog.cardId : "" }
    function captureCloseNoteDialog() {
        noteDialog.reject()
        noteDialog.visible = false
    }
    function captureCardIds() { return store.cardIds() }
    function captureSnooze(cardId, localDateTimeValue) {
        const action = captureActions[cardId]
        if (!action) return "missing-card-action"
        return action.captureSnooze(localDateTimeValue) ? "accepted" : "rejected"
    }
    function capturePinMoveUp(cardId) {
        const action = captureActions[cardId]
        if (!action) return "missing-card-action"
        if (!action.pinned) return "not-pinned"
        action.movePin(-1)
        return store.pinnedCardIds()[0] === cardId ? "moved" : "unchanged"
    }


    function openPage(name, properties) {
        const page = pagesDir.toString() + name + ".qml"
        stack.push(page, properties || { api: api })
    }
    function clock(card) { return card.timed ? timeRules.display(card.at || "") : "" }

    header: ToolBar {
        RowLayout {
            anchors.fill: parent
            Kirigami.Heading { text: "Litterbox"; level: 2; Layout.fillWidth: true }
            Label { text: store.online ? "Online" : "Offline" }
            Label { text: window.updateStatus; visible: text.length > 0; elide: Text.ElideRight }
            Button { text: qsTr("Check updates"); enabled: updater.supported && !updater.busy; onClicked: updater.checkForUpdates() }
            Button { text: qsTr("+ Add card"); onClicked: createDialog.open() }
            ToolButton { text: "Accounts"; onClicked: openPage("GmailAccountsPage") }
            ToolButton { text: "Enroll"; onClicked: openPage("EnrollmentPage") }
            ToolButton { text: "Refresh"; onClicked: store.refresh() }
        }
    }
    Connections {
        target: updater
        function onBusyChanged() { window.updateStatus = updater.busy ? qsTr("Checking for updates…") : "" }
        function onNoUpdateAvailable() { window.updateStatus = qsTr("No update available") }
        function onUpdateReady(version, sha256) {
            window.updateVersion = version
            window.updateStatus = qsTr("Update %1 downloaded and verified").arg(version)
            updateDialog.open()
        }
        function onInstallConsentRequired() {
            window.updateStatus = qsTr("Allow installs from Litterbox in Android settings, then tap Install again")
        }
        function onErrorOccurred(message) { window.updateStatus = message }
    }
    StackView {
        id: stack
        anchors.fill: parent
        initialItem: Item {
            ListView {
                id: inboxList
                objectName: "inboxList"
                anchors.fill: parent
                clip: true
                Component.onCompleted: forceActiveFocus()
                function captureLayoutMetrics() {
                    function find(item, name) {
                        if (item.objectName === name) return item
                        for (const child of item.children) {
                            const found = find(child, name)
                            if (found) return found
                        }
                        return null
                    }
                    const card = find(contentItem, "inboxCard")
                    const actions = card ? find(card, "inboxActions") : null
                    return ({
                        itemCount: count,
                        cardWidth: card ? card.width : -1,
                        cardX: card ? card.mapToItem(null, 0, 0).x : -1,
                        hasActions: actions !== null,
                        actionsX: actions ? actions.mapToItem(null, 0, 0).x : -1,
                        actionsWidth: actions ? actions.width : -1
                    })
                }
                Keys.onPressed: function(event) {
                    const page = Math.max(1, height * 0.85)
                    switch (event.key) {
                    case Qt.Key_Home: contentY = 0; break
                    case Qt.Key_End: contentY = Math.max(0, contentHeight - height); break
                    case Qt.Key_PageUp: contentY = Math.max(0, contentY - page); break
                    case Qt.Key_PageDown: contentY = Math.max(0, Math.min(contentHeight - height, contentY + page)); break
                    case Qt.Key_Up: contentY = Math.max(0, contentY - 48); break
                    case Qt.Key_Down: contentY = Math.max(0, Math.min(contentHeight - height, contentY + 48)); break
                    default: return
                    }
                    event.accepted = true
                }
                WheelHandler {
                    target: null
                    acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                    onWheel: function(event) {
                        const delta = event.pixelDelta.y !== 0 ? event.pixelDelta.y * 3 : event.angleDelta.y / 120 * 48
                        inboxList.contentY = Math.max(0, Math.min(inboxList.contentHeight - inboxList.height, inboxList.contentY - delta))
                        event.accepted = true
                    }
                }
                model: store
                spacing: 8
                section.delegate: Item {
                    width: ListView.view.width
                    height: sectionHeading.implicitHeight + 16
                    Kirigami.Heading {
                        id: sectionHeading
                        width: Math.min(parent.width - 32, 1200)
                        anchors.horizontalCenter: parent.horizontalCenter
                        level: 3
                        text: section === "pinned" ? "Pinned" : section === "now" ? "Now" : section === "later" ? "Later" : "Missed"
                        padding: 8
                    }
                }
                delegate: Item {
                    required property var card
                    required property string cardId
                    required property string title
                    width: ListView.view.width
                    height: cardFrame.implicitHeight
                    Frame {
                        id: cardFrame
                        objectName: "inboxCard"
                        width: Math.min(parent.width - 32, 1200)
                        x: (parent.width - width) / 2
                        ColumnLayout {
                            width: parent.width
                            RowLayout {
                                Layout.fillWidth: true
                                Label {
                                    text: title
                                    font.bold: true
                                    wrapMode: Text.Wrap
                                    Layout.fillWidth: true
                                    TapHandler {
                                        enabled: card.source === "mail"
                                        onTapped: window.openPage("MailDetailPage", { api: api, cardId: cardId })
                                    }
                                }
                                Label { text: window.clock(card) }
                            }
                            Label { text: card.summary || ""; visible: text.length > 0; wrapMode: Text.Wrap; Layout.fillWidth: true }
                            Label { text: card.note || ""; visible: text.length > 0; font.italic: true; wrapMode: Text.Wrap; Layout.fillWidth: true }
                            RowLayout {
                                Item {
                                    id: dragHandle
                                    objectName: "reorderHandle-" + cardId
                                    implicitWidth: 32
                                    implicitHeight: 32
                                    readonly property bool pinned: card.pinned_rank != null || card.section === "pinned"
                                    readonly property bool reorderable: pinned || !card.timed
                                    opacity: reorderable ? 1.0 : 0.4

                                    // The handle is the visible affordance and the hit-test
                                    // target; the drag itself is driven by the list-level
                                    // surface below, because under delivered pointer events a
                                    // pressed item only receives the pointer while it stays
                                    // inside its own 32px bounds.
                                    Component.onCompleted: window.cardHandles[cardId] = dragHandle
                                    Component.onDestruction: delete window.cardHandles[cardId]

                                    Label {
                                        anchors.centerIn: parent
                                        text: "="
                                        font.bold: true
                                        color: Kirigami.Theme.textColor
                                    }
                                    Accessible.name: qsTr("Drag to reorder")
                                    Accessible.description: reorderable ? qsTr("Hold and drag to a new position") : qsTr("Position is fixed by pin or time")
                                    Accessible.role: Accessible.Button
                                    // No attached ToolTip on this handle: its popup closes on
                                    // mouse press, and that close cancels the press on the
                                    // drag surface a millisecond in, so grabbing `=` while
                                    // hovering it never started a drag. The cursor carries
                                    // the affordance instead.
                                    HoverHandler { cursorShape: dragHandle.reorderable ? Qt.OpenHandCursor : Qt.ArrowCursor }
                                }
                                Label { text: card.source || ""; Layout.fillWidth: true }
                                Button {
                                    objectName: "noteButton-" + cardId
                                    text: "Note"
                                    onClicked: { noteDialog.cardId = cardId; noteField.text = card.note || ""; noteDialog.open() }
                                }
                                Button {
                                    objectName: "doneButton-" + cardId
                                    text: "Done"
                                    visible: card.source !== "mail"
                                    onClicked: store.dismiss(cardId)
                                }
                                CardActions {
                                    id: cardActions
                                    objectName: "inboxActions"
                                    store: window.cardStore
                                    cardKey: cardId
                                    source: card.source || ""
                                    bundleId: card.bundle_id || ""
                                    pinnedRank: card.pinned_rank
                                    Component.onCompleted: window.captureActions[cardId] = cardActions
                                    Component.onDestruction: delete window.captureActions[cardId]
                                }
                            }
                        }
                    }
                    // Overlay, not a Frame child: a second declared child collapses the
                    // Frame's implicit height (probe: 87px with one child, 18px with two).
                    Rectangle {
                        objectName: "dropIndicator"
                        visible: window.dragCardId.length > 0 && window.dragCardId !== cardId && window.cardIdAt(window.dragTargetIndex) === cardId
                        anchors.top: cardFrame.top
                        anchors.left: cardFrame.left
                        anchors.right: cardFrame.right
                        height: 2
                        color: Kirigami.Theme.highlightColor
                        z: 1
                    }
                }
            }

            // The accepting surface for the `=` drag. It must be as tall as the drag
            // travel: under delivered pointer events a pressed item stops receiving the
            // pointer as soon as it leaves its own 32px bounds, so a handle-sized press
            // target can never drive a reorder (probe15). It is confined to the handle
            // column instead of covering the list, so it covers no button, no card
            // action and no card body and those keep working unchanged (a full-area
            // surface was measured to swallow a Controls button click, probe17); a
            // press that is not on a reorderable handle is not accepted.
            MouseArea {
                id: reorderSurface
                objectName: "reorderSurface"
                property string draggingId: ""
                // Same column as the `=` handle inside the centred card frame.
                x: (parent.width - Math.min(parent.width - 32, 1200)) / 2
                y: 0
                width: 48
                height: parent.height
                // A press on a handle keeps the pointer for the whole travel: the list
                // is a Flickable and would otherwise take the grab mid-drag, which
                // cancels the drag before the drop can be committed. Flicking is
                // unaffected elsewhere: a press that is not on a reorderable handle is
                // not accepted by this surface, so the list still gets it.
                preventStealing: true
                function startDragAt(mouse) {
                    const ids = Object.keys(window.cardHandles)
                    for (let i = 0; i < ids.length; ++i) {
                        const handle = window.cardHandles[ids[i]]
                        if (!handle || !handle.reorderable)
                            continue
                        const p = handle.mapToItem(reorderSurface, 0, 0)
                        if (mouse.x >= p.x && mouse.x <= p.x + handle.width
                                && mouse.y >= p.y && mouse.y <= p.y + handle.height) {
                            // Take the card id from the rendered handle: a recycled
                            // delegate keeps its original registry key, so the map
                            // entry can be stale.
                            const cardKey = String(handle.objectName).replace("reorderHandle-", "")
                            if (cardKey !== ids[i]) {
                                delete window.cardHandles[ids[i]]
                                window.cardHandles[cardKey] = handle
                            }
                            reorderSurface.draggingId = cardKey
                            window.dragCardId = cardKey
                            window.dragTargetIndex = -1
                            mouse.accepted = true
                            return
                        }
                    }
                    mouse.accepted = false
                }
                onPressed: function (mouse) { reorderSurface.startDragAt(mouse) }
                // A press that lands on the same point as the previous release within
                // the double-click interval arrives as a double click, not as a press;
                // grabbing `=` twice in a row must still start a drag.
                onDoubleClicked: function (mouse) {
                    reorderSurface.startDragAt(mouse)
                }
                onPositionChanged: function (mouse) {
                    if (reorderSurface.draggingId.length === 0) {
                        mouse.accepted = false
                        return
                    }
                    const point = reorderSurface.mapToItem(inboxList, mouse.x, mouse.y)
                    window.dragTargetIndex = inboxList.indexAt(point.x, point.y + inboxList.contentY)
                }
                onReleased: function (mouse) {
                    if (reorderSurface.draggingId.length === 0) {
                        mouse.accepted = false
                        return
                    }
                    const cardId = reorderSurface.draggingId
                    // Take the landing row from the release point itself, so the drop
                    // does not depend on the last position update having been delivered.
                    const point = reorderSurface.mapToItem(inboxList, mouse.x, mouse.y)
                    const target = inboxList.indexAt(point.x, point.y + inboxList.contentY)
                    reorderSurface.draggingId = ""
                    window.dragCardId = ""
                    window.dragTargetIndex = -1
                    if (target >= 0)
                        window.commitCardDrag(cardId, target)
                }
                // canceled() carries no event argument; a drag that was taken away is
                // never committed from here.
                onCanceled: function () {
                    reorderSurface.draggingId = ""
                    window.dragCardId = ""
                    window.dragTargetIndex = -1
                }
            }
        }
    }
    Dialog {
        id: noteDialog
        property string cardId
        title: "Card note"
        modal: true
        anchors.centerIn: parent
        standardButtons: Dialog.Save | Dialog.Cancel
        onOpened: noteField.forceActiveFocus()
        // KDE Breeze TextArea assigns its TextArea target to a TextInput-only
        // mobile toolbar and aborts desktop root creation. Keep multiline edit
        // semantics using QtQuick.TextEdit inside a styled, scrollable frame.
        contentItem: Frame {
            implicitWidth: 360
            implicitHeight: 120
            padding: Kirigami.Units.smallSpacing
            Flickable {
                id: noteScroll
                anchors.fill: parent
                clip: true
                contentWidth: width
                contentHeight: Math.max(height, noteField.contentHeight)
                boundsBehavior: Flickable.StopAtBounds
                TextEdit {
                    id: noteField
                    width: noteScroll.width
                    height: Math.max(noteScroll.height, contentHeight)
                    wrapMode: TextEdit.Wrap
                    selectByMouse: true
                    color: Kirigami.Theme.textColor
                    selectionColor: Kirigami.Theme.highlightColor
                    selectedTextColor: Kirigami.Theme.highlightedTextColor
                    Accessible.role: Accessible.EditableText
                }
            }
        }
        onAccepted: store.saveNote(cardId, noteField.text)
    }
    Dialog {
        id: createDialog
        title: qsTr("Create card")
        modal: true
        anchors.centerIn: parent
        standardButtons: Dialog.Save | Dialog.Cancel
        onOpened: createTitle.forceActiveFocus()
        contentItem: ColumnLayout {
            TextField { id: createTitle; placeholderText: qsTr("Title"); Layout.fillWidth: true }
            TextField { id: createSummary; placeholderText: qsTr("Details (optional)"); Layout.fillWidth: true }
        }
        onAccepted: if (store.createCard(createTitle.text, createSummary.text)) { createTitle.clear(); createSummary.clear() }
    }
    Dialog {
        id: updateDialog
        title: qsTr("Install Android update")
        modal: true
        anchors.centerIn: parent
        contentItem: ColumnLayout {
            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: qsTr("Version %1 is downloaded and checksum-verified. Android will ask you to confirm installation.").arg(window.updateVersion)
            }
            RowLayout {
                Layout.alignment: Qt.AlignRight
                Button { text: qsTr("Later"); onClicked: updateDialog.close() }
                Button {
                    text: qsTr("Install")
                    onClicked: updater.installDownloadedUpdate()
                }
            }
        }
    }
}
