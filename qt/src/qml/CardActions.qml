import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Controls.Material
import org.kde.kirigami.dialogs as KirigamiDialogs
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

Controls.ToolButton {
    id: root
    required property var store
    required property string cardKey
    property string source: ""
    property string sourceLabel: source
    property bool hasBody: false
    property string bundleId: ""
    property var pinnedRank: undefined
    property string snoozeError: ""
    property string cardTitle: ""
    property string accountName: ""
    property bool canOpenSource: false
    readonly property color actionInk: "#263b3a"
    readonly property color actionSurface: "#ffffff"
    readonly property color actionCanvas: "#f3f7f6"
    readonly property color actionAccent: "#397d73"
    signal noteRequested()
    signal openRequested()
    signal readCachedRequested()
    readonly property string primaryName: source === "mail" ? qsTr("Archive in Gmail%1").arg(accountName ? " · " + accountName : "") : source === "home_assistant" ? qsTr("Dismiss Home Assistant notification") : qsTr("Done · dismiss in Litterbox only")
    function primaryAction() {
        if (source === "mail") store.enqueueOp(cardKey, "archive", {})
        else store.dismiss(cardKey)
    }
    readonly property bool pinStateKnown: pinnedRank !== undefined
    readonly property bool pinned: pinStateKnown && pinnedRank !== null

    function localDateTime(date) {
        const pad = value => String(value).padStart(2, "0")
        return date.getFullYear() + "-" + pad(date.getMonth() + 1) + "-" + pad(date.getDate()) +
            " " + pad(date.getHours()) + ":" + pad(date.getMinutes())
    }
    function chooseSnoozeDateTime() {
        snoozeError = ""
        snoozeDateTime.text = localDateTime(new Date(Date.now() + 3600000))
        snoozeDialog.open()
    }

    function captureSnooze(localDateTimeValue) {
        chooseSnoozeDateTime()
        snoozeDateTime.text = localDateTimeValue
        snoozeDialog.accept()
        return snoozeError.length === 0
    }
    function movePin(delta) {
        const current = Array.from(store.pinnedCardIds())
        const index = current.indexOf(cardKey)
        const target = index + delta
        if (index < 0 || target < 0 || target >= current.length) return
        const value = current[index]
        current[index] = current[target]
        current[target] = value
        store.enqueueOp(current[0], "reorder_pins", { cards: current })
    }
    function moveCard(delta) { store.moveCard(cardKey, delta) }

    text: qsTr("More actions")
    icon.name: "overflow-menu"
    icon.width: Kirigami.Units.iconSizes.smallMedium
    icon.height: Kirigami.Units.iconSizes.smallMedium
    icon.color: actionInk
    display: Controls.AbstractButton.IconOnly
    implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
    Accessible.name: qsTr("More actions for %1").arg(cardTitle)
    Controls.ToolTip.text: Accessible.name
    Controls.ToolTip.visible: hovered
    onClicked: actionSheet.open()
    KirigamiDialogs.MenuDialog {
        id: actionSheet
        parent: Controls.Overlay.overlay
        objectName: "cardActionSheet-" + root.cardKey
        title: root.cardTitle
        modal: true
        width: Math.min(360, root.Window.window ? root.Window.window.width - 2 * Kirigami.Units.largeSpacing : 360)
        implicitWidth: width
        // No StandardButton labels: Qt localizes them per system locale while
        // the app is English. Explicit footer buttons keep the text stable and
        // an explicit light background keeps the KDE style from painting a dark
        // slab from the host color scheme.
        footer: Controls.Pane {
            background: Rectangle { color: root.actionSurface }
            contentItem: RowLayout {
                Item { Layout.fillWidth: true }
                Controls.Button {
                    id: sheetCloseButton
                    objectName: "actionSheetClose-" + root.cardKey
                    text: qsTr("Close")
                    icon.name: "dialog-close"
                    icon.width: Kirigami.Units.iconSizes.small
                    icon.height: Kirigami.Units.iconSizes.small
                    icon.color: root.actionInk
                    implicitWidth: Math.max(96, Kirigami.Units.gridUnit * 6)
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Material.foreground: root.actionInk
                    palette.button: root.actionSurface
                    palette.buttonText: root.actionInk
                    background: Rectangle {
                        radius: Kirigami.Units.cornerRadius
                        color: sheetCloseButton.down ? "#e8eeed" : sheetCloseButton.hovered ? "#eef3f2" : root.actionSurface
                        border.width: sheetCloseButton.visualFocus ? 2 : 1
                        border.color: sheetCloseButton.visualFocus ? root.actionInk : "#dce5e3"
                    }
                    onClicked: actionSheet.close()
                }
            }
        }
        Material.theme: Material.Light
        Material.background: root.actionSurface
        Material.foreground: root.actionInk
        Material.accent: root.actionAccent
        Material.primary: root.actionAccent
        Kirigami.Theme.inherit: false
        Kirigami.Theme.textColor: root.actionInk
        Kirigami.Theme.backgroundColor: root.actionSurface
        Kirigami.Theme.alternateBackgroundColor: root.actionCanvas
        Kirigami.Theme.highlightColor: root.actionAccent
        Kirigami.Theme.focusColor: root.actionAccent
        palette.window: root.actionSurface
        palette.windowText: root.actionInk
        palette.base: root.actionSurface
        palette.text: root.actionInk
        palette.button: root.actionSurface
        palette.buttonText: root.actionInk
        palette.highlight: root.actionAccent
        palette.highlightedText: root.actionSurface
        background: Rectangle { color: root.actionSurface; radius: Kirigami.Units.cornerRadius; border.color: "#dce5e3" }
        anchors.centerIn: parent
        // Kirigami's internal ScrollView pins Platform.Theme.inherit:false and
        // colorSet:View, so the system dark scheme paints the action list
        // (#141618) regardless of the palette set on this dialog. Pin the light
        // theme on the content pane itself; the action delegates inherit it.
        Component.onCompleted: {
            const sv = actionSheet.contentItem
            sv.Kirigami.Theme.inherit = false
            sv.Kirigami.Theme.backgroundColor = root.actionSurface
            sv.Kirigami.Theme.alternateBackgroundColor = root.actionCanvas
            sv.Kirigami.Theme.textColor = root.actionInk
            sv.Kirigami.Theme.highlightColor = root.actionAccent
            sv.Kirigami.Theme.focusColor = root.actionAccent
            sv.palette.window = root.actionSurface
            sv.palette.base = root.actionSurface
            sv.palette.text = root.actionInk
            sv.palette.windowText = root.actionInk
            sv.palette.button = root.actionSurface
            sv.palette.buttonText = root.actionInk
        }
        onClosed: root.forceActiveFocus()
        actions: [
            Kirigami.Action {
                text: root.primaryName
                icon.name: root.source === "mail" ? "mail-mark-read-symbolic" : "dialog-ok"
                tooltip: root.primaryName
                onTriggered: { actionSheet.close(); root.primaryAction() }
            },
            Kirigami.Action {
                text: root.source === "mail" ? qsTr("Open in Gmail") : qsTr("Open source")
                icon.name: "document-open"
                tooltip: text
                visible: root.canOpenSource
                onTriggered: { actionSheet.close(); root.openRequested() }
            },
            Kirigami.Action {
                text: qsTr("Read cached · stays in Litterbox")
                icon.name: "document-preview"
                tooltip: text
                visible: root.source === "mail" || root.hasBody
                onTriggered: { actionSheet.close(); root.readCachedRequested() }
            },
            Kirigami.Action {
                objectName: "noteButton-" + root.cardKey
                text: qsTr("Note · instruction for this card")
                icon.name: "document-edit"
                tooltip: text
                onTriggered: { actionSheet.close(); root.noteRequested() }
            },
            Kirigami.Action {
                text: qsTr("Snooze · choose date and time")
                icon.name: "appointment-new"
                tooltip: text
                onTriggered: { actionSheet.close(); root.chooseSnoozeDateTime() }
            },
            Kirigami.Action {
                text: root.pinned ? qsTr("Unpin") : qsTr("Pin")
                icon.name: "pin"
                tooltip: text
                visible: root.pinStateKnown
                onTriggered: { actionSheet.close(); root.store.enqueueOp(root.cardKey, root.pinned ? "unpin" : "pin", {}) }
            },
            Kirigami.Action {
                text: root.source === "mail" ? qsTr("Archive unpinned bundle members") : qsTr("Complete unpinned bundle members")
                icon.name: root.source === "mail" ? "mail-mark-read-symbolic" : "folder"
                tooltip: text
                visible: root.bundleId.length > 0
                onTriggered: { actionSheet.close(); if (root.source === "mail") archiveDialog.open(); else completeDialog.open() }
            },
            Kirigami.Action {
                text: qsTr("Take out of bundle")
                icon.name: "list-remove"
                tooltip: text
                visible: root.bundleId.length > 0
                onTriggered: { actionSheet.close(); root.store.enqueueOp(root.cardKey, "take_out", { card: root.cardKey }) }
            }
        ]
    }

    Controls.Dialog {
        id: snoozeDialog
        objectName: "snoozeDialog"
        title: qsTr("Snooze until")
        modal: true
        // Explicit English buttons: StandardButton labels follow the system
        // locale while the app is English. accept()/reject() semantics unchanged.
        footer: Controls.DialogButtonBox {
            background: Rectangle { color: root.actionSurface }
            Controls.Button {
                id: snoozeOkButton
                objectName: "snoozeOkButton"
                text: qsTr("OK")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: root.actionSurface
                palette.buttonText: root.actionInk
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: snoozeOkButton.down ? "#e8eeed" : snoozeOkButton.hovered ? "#eef3f2" : root.actionSurface
                    border.width: snoozeOkButton.visualFocus ? 2 : 1
                    border.color: snoozeOkButton.visualFocus ? root.actionInk : "#dce5e3"
                }
                onClicked: snoozeDialog.accept()
            }
            Controls.Button {
                id: snoozeCancelButton
                objectName: "snoozeCancelButton"
                text: qsTr("Cancel")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: root.actionSurface
                palette.buttonText: root.actionInk
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: snoozeCancelButton.down ? "#e8eeed" : snoozeCancelButton.hovered ? "#eef3f2" : root.actionSurface
                    border.width: snoozeCancelButton.visualFocus ? 2 : 1
                    border.color: snoozeCancelButton.visualFocus ? root.actionInk : "#dce5e3"
                }
                onClicked: snoozeDialog.reject()
            }
        }
        Material.theme: Material.Light
        Material.background: root.actionSurface
        Material.foreground: root.actionInk
        Material.accent: root.actionAccent
        Material.primary: root.actionAccent
        palette.window: root.actionSurface
        palette.windowText: root.actionInk
        palette.base: root.actionSurface
        palette.text: root.actionInk
        palette.button: root.actionSurface
        palette.buttonText: root.actionInk
        palette.highlight: root.actionAccent
        palette.highlightedText: root.actionSurface
        width: Math.min(392, root.Window.window ? root.Window.window.width - 32 : 392)
        implicitWidth: width
        background: Rectangle { color: root.actionSurface; radius: 8; border.color: "#dce5e3" }
        onOpened: snoozeDateTime.forceActiveFocus()
        contentItem: ColumnLayout {
            Controls.Label { text: qsTr("Local date and time (YYYY-MM-DD HH:MM)"); Layout.fillWidth: true; wrapMode: Text.Wrap }
            Controls.TextField {
                id: snoozeDateTime
                objectName: "snoozeDateTime"
                Layout.fillWidth: true
                placeholderText: "YYYY-MM-DD HH:MM"
                inputMethodHints: Qt.ImhDate | Qt.ImhTime
                Accessible.name: qsTr("Local date and time")
            }
            Controls.Label {
                text: root.snoozeError
                visible: root.snoozeError.length > 0
                color: Kirigami.Theme.negativeTextColor
            }
        }
        onAccepted: {
            const parts = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(snoozeDateTime.text)
            const until = parts ? new Date(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]), Number(parts[4]), Number(parts[5])) : null
            const valid = until && until.getFullYear() === Number(parts[1]) &&
                until.getMonth() === Number(parts[2]) - 1 && until.getDate() === Number(parts[3]) &&
                until.getHours() === Number(parts[4]) && until.getMinutes() === Number(parts[5]) &&
                until.getTime() > Date.now()
            if (!valid) {
                root.snoozeError = qsTr("Enter a valid future local date and time.")
                Qt.callLater(function() { snoozeDialog.open() })
                return
            }
            root.snoozeError = ""
            root.store.enqueueOp(root.cardKey, "snooze", { until: until.toISOString() })
        }
    }
    Controls.Dialog {
        id: completeDialog
        objectName: "completeDialog"
        title: qsTr("Complete all unpinned cards in this bundle?")
        modal: true
        // Explicit English buttons (see snoozeDialog).
        footer: Controls.DialogButtonBox {
            background: Rectangle { color: root.actionSurface }
            Controls.Button {
                id: completeOkButton
                objectName: "completeOkButton"
                text: qsTr("OK")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: root.actionSurface
                palette.buttonText: root.actionInk
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: completeOkButton.down ? "#e8eeed" : completeOkButton.hovered ? "#eef3f2" : root.actionSurface
                    border.width: completeOkButton.visualFocus ? 2 : 1
                    border.color: completeOkButton.visualFocus ? root.actionInk : "#dce5e3"
                }
                onClicked: completeDialog.accept()
            }
            Controls.Button {
                id: completeCancelButton
                objectName: "completeCancelButton"
                text: qsTr("Cancel")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: root.actionSurface
                palette.buttonText: root.actionInk
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: completeCancelButton.down ? "#e8eeed" : completeCancelButton.hovered ? "#eef3f2" : root.actionSurface
                    border.width: completeCancelButton.visualFocus ? 2 : 1
                    border.color: completeCancelButton.visualFocus ? root.actionInk : "#dce5e3"
                }
                onClicked: completeDialog.reject()
            }
        }
        Material.theme: Material.Light
        Material.background: root.actionSurface
        Material.foreground: root.actionInk
        Material.accent: root.actionAccent
        Material.primary: root.actionAccent
        palette.window: root.actionSurface
        palette.windowText: root.actionInk
        palette.base: root.actionSurface
        palette.text: root.actionInk
        palette.button: root.actionSurface
        palette.buttonText: root.actionInk
        palette.highlight: root.actionAccent
        palette.highlightedText: root.actionSurface
        background: Rectangle { color: root.actionSurface; radius: 8; border.color: "#dce5e3" }
        onAccepted: root.store.enqueueOp(root.cardKey, "bundle_done", { bundle_id: root.bundleId })
    }
    Controls.Dialog {
        id: archiveDialog
        objectName: "archiveDialog"
        title: qsTr("Archive all unpinned cards in this bundle?")
        modal: true
        // Explicit English buttons (see snoozeDialog).
        footer: Controls.DialogButtonBox {
            background: Rectangle { color: root.actionSurface }
            Controls.Button {
                id: archiveOkButton
                objectName: "archiveOkButton"
                text: qsTr("OK")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: root.actionSurface
                palette.buttonText: root.actionInk
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: archiveOkButton.down ? "#e8eeed" : archiveOkButton.hovered ? "#eef3f2" : root.actionSurface
                    border.width: archiveOkButton.visualFocus ? 2 : 1
                    border.color: archiveOkButton.visualFocus ? root.actionInk : "#dce5e3"
                }
                onClicked: archiveDialog.accept()
            }
            Controls.Button {
                id: archiveCancelButton
                objectName: "archiveCancelButton"
                text: qsTr("Cancel")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: root.actionSurface
                palette.buttonText: root.actionInk
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: archiveCancelButton.down ? "#e8eeed" : archiveCancelButton.hovered ? "#eef3f2" : root.actionSurface
                    border.width: archiveCancelButton.visualFocus ? 2 : 1
                    border.color: archiveCancelButton.visualFocus ? root.actionInk : "#dce5e3"
                }
                onClicked: archiveDialog.reject()
            }
        }
        Material.theme: Material.Light
        Material.background: root.actionSurface
        Material.foreground: root.actionInk
        Material.accent: root.actionAccent
        Material.primary: root.actionAccent
        palette.window: root.actionSurface
        palette.windowText: root.actionInk
        palette.base: root.actionSurface
        palette.text: root.actionInk
        palette.button: root.actionSurface
        palette.buttonText: root.actionInk
        palette.highlight: root.actionAccent
        palette.highlightedText: root.actionSurface
        background: Rectangle { color: root.actionSurface; radius: 8; border.color: "#dce5e3" }
        onAccepted: root.store.enqueueOp(root.cardKey, "bundle_archive", { bundle_id: root.bundleId })
    }
}
