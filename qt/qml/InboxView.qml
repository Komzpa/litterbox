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
            ToolButton { text: "Accounts"; onClicked: openPage("GmailAccountsPage") }
            ToolButton { text: "Enroll"; onClicked: openPage("EnrollmentPage") }
            ToolButton { text: "Refresh"; onClicked: store.refresh() }
        }
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
                                Label { text: card.source || ""; Layout.fillWidth: true }
                                Button { text: "Note"; onClicked: { noteDialog.cardId = cardId; noteField.text = card.note || ""; noteDialog.open() } }
                                Button { text: "Done"; visible: card.source !== "mail"; onClicked: store.dismiss(cardId) }
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
}
