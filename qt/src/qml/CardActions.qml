import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Controls.Material
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
    property var snoozePresets: []
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
    // Navigation follows the optimistic mutation, never its HTTP acknowledgement.
    signal cardHandled(string operation, string action)
    readonly property string primaryName: source === "mail" ? qsTr("Archive in Gmail%1").arg(accountName ? " · " + accountName : "") : source === "journal" ? qsTr("Archive private note") : source === "home_assistant" ? qsTr("Dismiss Home Assistant notification") : source === "manual" ? qsTr("Complete task in Litterbox") : qsTr("Dismiss in Litterbox only")
    function primaryAction() {
        const archive = source === "mail" || source === "journal"
        const operation = archive ? store.enqueueOp(cardKey, "archive", {}) : store.dismiss(cardKey)
        // Only mail archive has a registered inverse after transmission.
        // Do not advertise Undo for completed tasks or local dismissals.
        if (operation) cardHandled(source === "mail" ? operation : "", archive ? "archive" : "done")
    }
    readonly property bool pinStateKnown: pinnedRank !== undefined
    readonly property bool pinned: pinStateKnown && pinnedRank !== null

    function localDateTime(date) {
        const pad = value => String(value).padStart(2, "0")
        return date.getFullYear() + "-" + pad(date.getMonth() + 1) + "-" + pad(date.getDate()) +
            " " + pad(date.getHours()) + ":" + pad(date.getMinutes())
    }
    // Presets are built from local Date parts, so the named wall-clock times
    // survive DST changes; only strictly future times are ever offered.
    function buildSnoozePresets() {
        const now = new Date()
        const at = (dayOffset, hour, minute) => new Date(now.getFullYear(), now.getMonth(), now.getDate() + dayOffset, hour, minute, 0, 0)
        const presets = [{ key: "hour", name: qsTr("In one hour"), until: new Date(now.getTime() + 3600000) }]
        const evening = at(0, 18, 0)
        if (evening.getTime() > now.getTime())
            presets.push({ key: "evening", name: qsTr("This evening"), until: evening })
        presets.push({ key: "tomorrow", name: qsTr("Tomorrow morning"), until: at(1, 9, 0) })
        presets.push({ key: "monday", name: qsTr("Next Monday morning"), until: at(((8 - now.getDay()) % 7) || 7, 9, 0) })
        return presets
    }
    function snoozePreset(key) {
        for (const preset of snoozePresets) if (preset.key === key) return preset
        return null
    }
    function snoozeWhenText(date) {
        if (!date) return ""
        return Qt.formatDateTime(date, date.getFullYear() === new Date().getFullYear() ? "d MMM HH:mm" : "d MMM yyyy HH:mm")
    }
    // Menu rows carry the computed instant, so the owner always sees the
    // real local date and time; an unknown preset hides its row.
    function snoozePresetLabel(key) {
        const preset = snoozePreset(key)
        return preset ? preset.name + " · " + snoozeWhenText(preset.until) : ""
    }
    // One rejection rule for menu, picker and capture: round-trip exact local
    // parts (no Feb 30) and never snooze into the past.
    function snoozeAcceptable(until, parts) {
        return !!until && (!parts || (until.getFullYear() === parts[0] && until.getMonth() === parts[1] &&
            until.getDate() === parts[2] && until.getHours() === parts[3] && until.getMinutes() === parts[4])) &&
            until.getTime() > Date.now()
    }
    function commitSnooze(until) {
        snoozeError = ""
        if (store.enqueueOp(cardKey, "snooze", { until: until.toISOString() }))
            cardHandled("", "snooze")
    }
    function chooseSnoozeDateTime() {
        snoozeError = ""
        snoozePresets = buildSnoozePresets()
        const menu = snoozeMenu()
        const point = root.mapToItem(Controls.Overlay.overlay, 0, root.height)
        menu.popup(point.x, point.y)
    }
    function pickSnooze(key) {
        const preset = snoozePreset(key)
        if (preset && snoozeAcceptable(preset.until, null)) commitSnooze(preset.until)
    }
    function openCustomSnooze() {
        snoozeError = ""
        const soon = new Date(Date.now() + 3600000)
        const dialog = snoozePicker()
        dialog.loadTarget(soon.getFullYear(), soon.getMonth() + 1, soon.getDate(), soon.getHours(), soon.getMinutes())
        dialog.open()
    }
    // Capture drives the real picker accept path with structured fields, never
    // a raw string. The raw parts are validated before touching the dialog so
    // spin-range clamping can never rewrite the requested local date and time.
    function captureSnooze(localDateTimeValue) {
        const parts = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(String(localDateTimeValue))
        snoozeError = ""
        if (!parts) {
            snoozeError = qsTr("Enter a valid future local date and time.")
            return false
        }
        const expected = [Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]), Number(parts[4]), Number(parts[5])]
        const until = new Date(expected[0], expected[1], expected[2], expected[3], expected[4])
        if (!snoozeAcceptable(until, expected)) {
            snoozeError = qsTr("Enter a valid future local date and time.")
            return false
        }
        const dialog = snoozePicker()
        dialog.loadTarget(expected[0], expected[1] + 1, expected[2], expected[3], expected[4])
        dialog.open()
        dialog.accept()
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
    function actionSheetItem() { actionSheetLoader.active = true; return actionSheetLoader.item }
    function snoozeMenu() { snoozeMenuLoader.active = true; return snoozeMenuLoader.item }
    function snoozePicker() { snoozeLoader.active = true; return snoozeLoader.item }
    // MailDetailPage gates its archive shortcut on this item's `visible`; it
    // must report either snooze surface while that surface is open.
    function snoozeDialogItem() {
        snoozeLoader.active = true
        snoozeMenuLoader.active = true
        return snoozeLoader.item.visible ? snoozeLoader.item : snoozeMenuLoader.item
    }
    function moveCard(delta) { store.moveCard(cardKey, delta) }

    text: qsTr("More actions")
    icon.name: "overflow-menu"
    icon.width: 20
    icon.height: 20
    icon.color: actionInk
    display: Controls.AbstractButton.IconOnly
    implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
    Accessible.name: qsTr("More actions for %1").arg(cardTitle)
    Controls.ToolTip.text: Accessible.name
    Controls.ToolTip.visible: hovered || visualFocus
    onClicked: actionSheetItem().open()

    component ActionRow: Controls.ItemDelegate {
        id: actionRow
        Layout.fillWidth: true
        Layout.minimumWidth: 0
        implicitHeight: Math.max(48, implicitContentHeight + topPadding + bottomPadding)
        leftPadding: 12
        rightPadding: 12
        topPadding: 8
        bottomPadding: 8
        icon.width: 20
        icon.height: 20
        icon.color: root.actionInk
        Accessible.name: text
        Controls.ToolTip.text: text
        Controls.ToolTip.visible: hovered || visualFocus
        contentItem: RowLayout {
            spacing: 12
            Kirigami.Icon {
                source: actionRow.icon.name
                isMask: true
                color: actionRow.icon.color
                implicitWidth: 20
                implicitHeight: 20
            }
            Controls.Label {
                text: actionRow.text
                textFormat: Text.PlainText
                color: root.actionInk
                font: Kirigami.Theme.defaultFont
                Layout.fillWidth: true
                Layout.minimumWidth: 0
                wrapMode: Text.Wrap
            }
        }
        background: Rectangle {
            radius: 6
            color: actionRow.down ? "#e8eeed" : actionRow.hovered ? "#eef3f2" : root.actionSurface
            border.width: actionRow.visualFocus ? 2 : 0
            border.color: root.actionAccent
        }
    }
    // Created on first use: three dialogs per card made every new inbox row
    // cost ~8 ms to instantiate (R63 density regression, scroll collapse).
    Loader {
        id: actionSheetLoader
        active: false
        sourceComponent: Component {
            Controls.Dialog {
                id: actionSheet
                parent: Controls.Overlay.overlay
                objectName: "cardActionSheet-" + root.cardKey
                title: root.cardTitle
                modal: true
                width: Math.min(360, root.Window.window ? root.Window.window.width - 2 * Kirigami.Units.largeSpacing : 360)
                implicitWidth: width
                height: Math.min(implicitHeight, root.Window.window ? root.Window.window.height - 32 : 640)
                // Explicit title: the style-provided header label follows the host
                // theme (invisible white on our white surface under Breeze and
                // dark-Material), so pin ink directly.
                header: Controls.Label {
                    text: root.cardTitle
                    color: root.actionInk
                    font.pointSize: Kirigami.Theme.defaultFont.pointSize + 4
                    font.weight: Font.DemiBold
                    padding: Kirigami.Units.largeSpacing
                    wrapMode: Text.Wrap
                    maximumLineCount: 3
                    elide: Text.ElideRight
                }
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
                            implicitWidth: Math.max(96, Kirigami.Units.gridUnit * 6)
                            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                            // Explicit content: the Breeze and dark-Material styles
                            // resolve the default button label from the host theme
                            // (white on our white footer), so pin ink directly with
                            // plain color bindings no style can override.
                            contentItem: RowLayout {
                                spacing: Kirigami.Units.smallSpacing
                                Kirigami.Icon {
                                    source: "dialog-close"
                                    isMask: true
                                    color: root.actionInk
                                    implicitWidth: Kirigami.Units.iconSizes.small
                                    implicitHeight: Kirigami.Units.iconSizes.small
                                }
                                Controls.Label {
                                    text: sheetCloseButton.text
                                    color: root.actionInk
                                }
                            }
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
                onClosed: root.forceActiveFocus()
                // The actions, not the dialog, scroll on short phones or large text.
                contentItem: Controls.ScrollView {
                    id: actionScroll
                    clip: true
                    implicitHeight: actionRows.implicitHeight
                    contentWidth: availableWidth
                    Controls.ScrollBar.horizontal.policy: Controls.ScrollBar.AlwaysOff
                    ColumnLayout {
                        id: actionRows
                        width: actionScroll.availableWidth
                        spacing: 4
                        ActionRow {
                            objectName: "actionRow-primary-" + root.cardKey
                            text: root.primaryName
                            icon.name: root.source === "mail" || root.source === "journal" ? "mail-archive" : "dialog-ok"
                            onClicked: { actionSheet.close(); root.primaryAction() }
                        }
                        ActionRow {
                            objectName: "actionRow-spam-" + root.cardKey
                            text: qsTr("Mark as spam%1").arg(root.accountName ? " · " + root.accountName : "")
                            icon.name: "mail-mark-junk"
                            icon.color: "#a33f32"
                            visible: root.source === "mail"
                            onClicked: {
                                actionSheet.close()
                                const operation = root.store.enqueueOp(root.cardKey, "spam", {})
                                if (operation) root.cardHandled(operation, "spam")
                            }
                        }
                        ActionRow {
                            objectName: "actionRow-open-" + root.cardKey
                            text: root.source === "mail" ? qsTr("Open in Gmail") : qsTr("Open source")
                            icon.name: "document-open"
                            visible: root.canOpenSource
                            onClicked: { actionSheet.close(); root.openRequested() }
                        }
                        ActionRow {
                            objectName: "actionRow-cached-" + root.cardKey
                            text: qsTr("Read cached · stays in Litterbox")
                            icon.name: "document-preview"
                            visible: root.source === "mail" || root.hasBody
                            onClicked: { actionSheet.close(); root.readCachedRequested() }
                        }
                        ActionRow {
                            objectName: "actionRow-note-" + root.cardKey
                            text: qsTr("Note · instruction for this card")
                            icon.name: "document-edit"
                            onClicked: { actionSheet.close(); root.noteRequested() }
                        }
                        ActionRow {
                            objectName: "actionRow-snooze-" + root.cardKey
                            text: qsTr("Snooze · choose when")
                            icon.name: "appointment-new"
                            onClicked: { actionSheet.close(); root.chooseSnoozeDateTime() }
                        }
                        ActionRow {
                            objectName: "actionRow-pin-" + root.cardKey
                            text: root.pinned ? qsTr("Unpin") : qsTr("Pin")
                            icon.name: "pin"
                            visible: root.pinStateKnown
                            onClicked: { actionSheet.close(); root.store.enqueueOp(root.cardKey, root.pinned ? "unpin" : "pin", {}) }
                        }
                        ActionRow {
                            objectName: "actionRow-takeout-" + root.cardKey
                            text: qsTr("Take out of bundle")
                            icon.name: "list-remove"
                            visible: root.bundleId.length > 0
                            onClicked: { actionSheet.close(); root.store.enqueueOp(root.cardKey, "take_out", { card: root.cardKey }) }
                        }
                    }
                }
            }
        }
    }

    // Created on first use: popups per card made every new inbox row cost
    // ~8 ms to instantiate (R63 density regression, scroll collapse).
    // Shared desktop/native Android menu: QQC2 Menu renders as the platform
    // menu on both hosts. Presets come from a refreshed model so every row
    // shows its actual local date and time; cancelling closes without action.
    Loader {
        id: snoozeMenuLoader
        active: false
        sourceComponent: Component {
            Controls.Menu {
                id: snoozeMenuPopup
                objectName: "snoozeMenu"
                parent: Controls.Overlay.overlay
                modal: true
                // Material derives Menu width from its background/contentWidth,
                // not the item labels. Our custom background has no intrinsic
                // width, so derive it from the rows before bounding the popup.
                implicitWidth: Math.max(200, hourPresetItem.implicitWidth,
                    eveningPresetItem.visible ? eveningPresetItem.implicitWidth : 0,
                    tomorrowPresetItem.implicitWidth, mondayPresetItem.implicitWidth,
                    customSnoozeItem.implicitWidth) + leftPadding + rightPadding
                margins: 12
                width: Math.min(implicitWidth, Math.max(0, parent.width - 2 * margins))
                height: Math.min(implicitHeight, Math.max(0, parent.height - 2 * margins))
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
                // Fixed rows bound to the refreshed preset list: only the
                // conditional evening row toggles visibility, so no dynamic
                // menu-item machinery is needed.
                Controls.MenuItem {
                    id: hourPresetItem
                    objectName: "snoozePreset-hour"
                    text: root.snoozePresetLabel("hour")
                    visible: text.length > 0
                    icon.name: "appointment-new"
                    icon.width: 20
                    icon.height: 20
                    icon.color: root.actionInk
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Accessible.name: text
                    onClicked: root.pickSnooze("hour")
                }
                Controls.MenuItem {
                    id: eveningPresetItem
                    objectName: "snoozePreset-evening"
                    text: root.snoozePresetLabel("evening")
                    visible: text.length > 0
                    icon.name: "appointment-new"
                    icon.width: 20
                    icon.height: 20
                    icon.color: root.actionInk
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Accessible.name: text
                    onClicked: root.pickSnooze("evening")
                }
                Controls.MenuItem {
                    id: tomorrowPresetItem
                    objectName: "snoozePreset-tomorrow"
                    text: root.snoozePresetLabel("tomorrow")
                    visible: text.length > 0
                    icon.name: "appointment-new"
                    icon.width: 20
                    icon.height: 20
                    icon.color: root.actionInk
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Accessible.name: text
                    onClicked: root.pickSnooze("tomorrow")
                }
                Controls.MenuItem {
                    id: mondayPresetItem
                    objectName: "snoozePreset-monday"
                    text: root.snoozePresetLabel("monday")
                    visible: text.length > 0
                    icon.name: "appointment-new"
                    icon.width: 20
                    icon.height: 20
                    icon.color: root.actionInk
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Accessible.name: text
                    onClicked: root.pickSnooze("monday")
                }
                Controls.MenuSeparator { }
                Controls.MenuItem {
                    id: customSnoozeItem
                    objectName: "snoozeCustom"
                    text: qsTr("Custom date and time…")
                    icon.name: "document-edit"
                    icon.width: 20
                    icon.height: 20
                    icon.color: root.actionInk
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Accessible.name: text
                    onClicked: root.openCustomSnooze()
                }
            }
        }
    }
    Loader {
        id: snoozeLoader
        active: false
        sourceComponent: Component {
            Controls.Dialog {
                parent: Controls.Overlay.overlay
                anchors.centerIn: parent
                id: snoozeDialog
                objectName: "snoozeDialog"
                function loadTarget(year, month, day, hour, minute) {
                    snoozeYear.value = year
                    snoozeMonth.value = month
                    snoozeDay.value = day
                    snoozeHour.value = hour
                    snoozeMinute.value = minute
                }
                title: qsTr("Snooze until")
                header: Controls.Label {
                    text: snoozeDialog.title
                    color: root.actionInk
                    font: Kirigami.Theme.defaultFont
                    padding: Kirigami.Units.largeSpacing
                    wrapMode: Text.Wrap
                }
                modal: true
                // Explicit English buttons: StandardButton labels follow the system
                // locale while the app is English. accept()/reject() semantics unchanged.
                footer: Controls.DialogButtonBox {
                    background: Rectangle { color: root.actionSurface }
                    Controls.Button {
                        id: snoozeOkButton
                        objectName: "snoozeOkButton"
                        text: qsTr("OK")
                        // Explicit width and content item: the custom background
                        // below replaces the style plate that paints the label,
                        // so without these the footer buttons lay out at zero
                        // width (unpainted, unclickable). Size and pin ink like
                        // the action sheet Close button.
                        implicitWidth: Math.max(96, Kirigami.Units.gridUnit * 6)
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        contentItem: Controls.Label {
                            text: snoozeOkButton.text
                            color: root.actionInk
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            elide: Text.ElideRight
                        }
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
                        // Explicit width and content item (see snoozeOkButton).
                        implicitWidth: Math.max(96, Kirigami.Units.gridUnit * 6)
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        contentItem: Controls.Label {
                            text: snoozeCancelButton.text
                            color: root.actionInk
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            elide: Text.ElideRight
                        }
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
                Kirigami.Theme.inherit: false
                Kirigami.Theme.textColor: root.actionInk
                Kirigami.Theme.backgroundColor: root.actionSurface
                Kirigami.Theme.alternateBackgroundColor: root.actionCanvas
                Kirigami.Theme.highlightColor: root.actionAccent
                Kirigami.Theme.focusColor: root.actionAccent
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
                height: Math.min(implicitHeight, root.Window.window ? root.Window.window.height - 32 : 640)
                background: Rectangle { color: root.actionSurface; radius: 8; border.color: "#dce5e3" }
                onOpened: snoozeDay.forceActiveFocus()
                contentItem: Controls.ScrollView {
                    id: snoozeScroll
                    clip: true
                    implicitHeight: snoozeFields.implicitHeight
                    contentWidth: availableWidth
                    Controls.ScrollBar.horizontal.policy: Controls.ScrollBar.AlwaysOff
                    ColumnLayout {
                        id: snoozeFields
                        width: snoozeScroll.availableWidth
                        // Structured local date/time picker: typed digits and
                        // arrow keys step values, so no raw string is parsed.
                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8
                            Controls.Label { text: qsTr("Date"); color: root.actionInk; font: Kirigami.Theme.defaultFont }
                            Item { Layout.fillWidth: true }
                            Controls.SpinBox {
                                id: snoozeYear
                                objectName: "snoozeYear"
                                from: 2000
                                to: 2100
                                editable: true
                                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                Layout.fillWidth: true
                                Accessible.name: qsTr("Year")
                            }
                            Controls.SpinBox {
                                id: snoozeMonth
                                objectName: "snoozeMonth"
                                from: 1
                                to: 12
                                editable: true
                                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                Layout.fillWidth: true
                                Accessible.name: qsTr("Month")
                            }
                            Controls.SpinBox {
                                id: snoozeDay
                                objectName: "snoozeDay"
                                from: 1
                                to: 31
                                editable: true
                                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                Layout.fillWidth: true
                                Accessible.name: qsTr("Day of month")
                            }
                        }
                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8
                            Controls.Label { text: qsTr("Time"); color: root.actionInk; font: Kirigami.Theme.defaultFont }
                            Item { Layout.fillWidth: true }
                            Controls.SpinBox {
                                id: snoozeHour
                                objectName: "snoozeHour"
                                from: 0
                                to: 23
                                editable: true
                                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                Layout.fillWidth: true
                                Accessible.name: qsTr("Hour")
                            }
                            Controls.SpinBox {
                                id: snoozeMinute
                                objectName: "snoozeMinute"
                                from: 0
                                to: 59
                                editable: true
                                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                Layout.fillWidth: true
                                Accessible.name: qsTr("Minute")
                            }
                        }
                        Controls.Label {
                            text: qsTr("Brings the card back %1").arg(root.snoozeWhenText(new Date(snoozeYear.value, snoozeMonth.value - 1, snoozeDay.value, snoozeHour.value, snoozeMinute.value)))
                            color: root.actionInk
                            font: Kirigami.Theme.defaultFont
                            Layout.fillWidth: true
                            wrapMode: Text.Wrap
                        }
                        Controls.Label {
                            text: root.snoozeError
                            visible: root.snoozeError.length > 0
                            color: Kirigami.Theme.negativeTextColor
                            Layout.fillWidth: true
                            wrapMode: Text.Wrap
                        }
                    }
                }
                onAccepted: {
                    const parts = [snoozeYear.value, snoozeMonth.value - 1, snoozeDay.value, snoozeHour.value, snoozeMinute.value]
                    const until = new Date(parts[0], parts[1], parts[2], parts[3], parts[4])
                    if (!root.snoozeAcceptable(until, parts)) {
                        root.snoozeError = qsTr("Enter a valid future local date and time.")
                        Qt.callLater(function() { snoozeDialog.open() })
                        return
                    }
                    root.commitSnooze(until)
                }
            }
        }
    }
    // Created on first use: three dialogs per card made every new inbox row
    // cost ~8 ms to instantiate (R63 density regression, scroll collapse).
    Loader {
        id: completeLoader
        active: false
        sourceComponent: Component {
            Controls.Dialog {
        parent: root
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
        }
    }
}
