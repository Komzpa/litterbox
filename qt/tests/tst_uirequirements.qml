import QtQuick
import QtQuick.Controls.Material
import QtTest
import litterbox 1.0 as App

TestCase {
    name: "UiRequirements"
    width: 520
    height: 800

    ListModel {
        id: store
        property bool online: false
        function cardIds() { return ["card-1"] }
        function pinnedCardIds() { return [] }
        function sourceLabel(card) { return card.source }
        function dismiss(cardId) {}
        function refresh() {}
        function createCard(title, summary) { return true }
        function saveNote(cardId, note) {}
        function moveCardTo(cardId, index) { return true }
        function enqueueOp(cardId, operation, args) {}
    }
    QtObject {
        id: api
        property string baseUrl: ""
        property string token: ""
    }
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

    function init() {
        store.clear()
        store.append({cardId: "card-1", title: "UI requirements card", section: "now", card: {
            id: "card-1", title: "UI requirements card", source: "manual", section: "now",
            has_body: false, pinned_rank: null, bundle_id: "", important: false,
            timed: false, note: "", summary: "", account_name: ""
        }})
    }

    function createInbox() {
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules
        })
        verify(inbox)
        inbox.show()
        return inbox
    }


    function relativeLuminance(color) {
        function linear(channel) {
            return channel <= 0.04045 ? channel / 12.92 : Math.pow((channel + 0.055) / 1.055, 2.4)
        }
        return 0.2126 * linear(color.r) + 0.7152 * linear(color.g) + 0.0722 * linear(color.b)
    }

    function actionTextColor(control) {
        const pending = [control.contentItem]
        while (pending.length > 0) {
            const item = pending.shift()
            if (item.color !== undefined && item.text !== undefined &&
                    String(item.text).length > 0) return item.color
            for (const child of item.children || []) pending.push(child)
        }
        return control.contentItem.color !== undefined
            ? control.contentItem.color : control.palette.text
    }

    function actionIcon(control) {
        const pending = [control.contentItem]
        while (pending.length > 0) {
            const item = pending.shift()
            if (item.source !== undefined && item.color !== undefined &&
                    String(item.source).length > 0) return item
            for (const child of item.children || []) pending.push(child)
        }
        return control.icon
    }

    function test_realInboxActionsAreNamedAndLargeAndSheetPaletteIsLight() {
        const inbox = createInbox()
        const list = findChild(inbox, "inboxList")
        verify(list)
        list.forceLayout()
        compare(list.count, 1)
        const row = list.itemAtIndex(0)
        verify(row)
        const moreActions = findChild(row, "inboxActions")
        verify(moreActions)
        compare(moreActions.icon.name, "overflow-menu")
        verify(moreActions.implicitWidth >= 48)
        verify(moreActions.implicitHeight >= 48)

        const done = findChild(row, "doneButton-card-1")
        verify(done)
        verify(done.icon.name.length > 0 || done.text.length > 0)
        verify(done.implicitWidth >= 48)
        verify(done.implicitHeight >= 48)

        mouseClick(moreActions)
        const sheet = findChild(moreActions, "cardActionSheet-card-1")
        verify(sheet)
        tryVerify(function() { return sheet.visible && sheet.contentItem !== null })
        sheet.Material.theme = Material.Dark
        verify(Qt.colorEqual(sheet.contentItem.palette.window, "#ffffff"))
        verify(Qt.colorEqual(sheet.contentItem.palette.base, "#ffffff"))

        const close = findChild(sheet, "actionSheetClose-card-1")
        verify(close)
        verify(close.text.length > 0)
        verify(close.implicitWidth >= 48)
        verify(close.implicitHeight >= 48)
        let checkedActions = 0
        for (const name of ["primary", "open", "cached", "note", "snooze", "pin", "bundle", "takeout"]) {
            const control = findChild(sheet, "actionRow-" + name + "-card-1")
            verify(control, "missing real action row " + name)
            if (!control.visible) continue
            checkedActions++
            const fill = control.background && control.background.color !== undefined
                ? control.background.color : control.palette.button
            verify(fill !== undefined,
                   "action row has no resolved background fill: " + name)
            const fillLuminance = relativeLuminance(fill)
            verify(fillLuminance > 0.8,
                   "action row background is not light: " + name + " luminance=" + fillLuminance + " color=" + fill)
            const textColor = actionTextColor(control)
            const textLuminance = relativeLuminance(textColor)
            verify(textLuminance < 0.4,
                   "action row text is not dark: " + name + " luminance=" + textLuminance + " color=" + textColor)
            const icon = actionIcon(control)
            const iconColor = icon.color
            verify(iconColor !== undefined && iconColor.a > 0,
                   "action row icon has no visible color: " + name + " color=" + iconColor)
            const iconLuminance = relativeLuminance(iconColor)
            verify(iconLuminance < 0.4,
                   "action row icon is not dark: " + name + " luminance=" + iconLuminance + " color=" + iconColor)
            if (icon.isMask !== undefined)
                verify(icon.isMask, "action row icon tint is not applied: " + name)
            verify(String(control.icon.name || "").length > 0 || String(control.text || "").trim().length > 0)
            verify(control.width >= 48 && control.height >= 48,
                   "action row target below 48px: " + name + " " + control.width + "x" + control.height)
        }
        verify(checkedActions > 0)
    }
}
