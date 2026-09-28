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
                anchors.fill: parent
                clip: true
                model: store
                spacing: 8
                section.property: "section"
                section.delegate: Kirigami.Heading {
                    width: ListView.view.width
                    level: 3
                    text: section === "pinned" ? "Pinned" : section === "now" ? "Now" : section === "later" ? "Later" : "Missed"
                    padding: 8
                }
                delegate: Frame {
                    required property var card
                    required property string cardId
                    required property string title
                    width: ListView.view.width - 20
                    x: 10
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
                                store: window.cardStore
                                cardKey: cardId
                                source: card.source || ""
                                bundleId: card.bundle_id || ""
                                pinnedRank: card.pinned_rank
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
