// SPDX-License-Identifier: MIT
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import QtCore
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root
    objectName: "settingsPage"
    property var api
    property var updater
    property var accounts: []
    property string accountError: ""
    property string saveStatus: ""
    readonly property bool enrolled: !!(api && api.token)
    readonly property bool serverUrlValid: /^https?:\/\/[a-z0-9.-]+(?::[0-9]+)?(?:[/?#][^\s]*)?$/i.test(serverField.text.trim())
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    title: qsTr("Settings")
    padding: Math.max(18, Kirigami.Units.largeSpacing)
    Kirigami.Theme.inherit: false
    Kirigami.Theme.textColor: ink
    Kirigami.Theme.disabledTextColor: mutedInk
    Kirigami.Theme.backgroundColor: canvas
    Kirigami.Theme.alternateBackgroundColor: surface
    Kirigami.Theme.highlightColor: accent
    Material.theme: Material.Light
    Material.background: surface
    Material.foreground: ink
    Material.accent: accent
    palette.window: canvas
    palette.windowText: ink
    palette.base: surface
    palette.text: ink
    palette.buttonText: ink
    background: Rectangle { color: root.canvas }
    header: BackToInboxBar { stackPage: root }
    Settings {
        id: identity
        category: "enrollment"
        property string deviceId: ""
        property string tenantId: ""
        property string deviceName: ""
        property string serverUrl: ""
    }
    function reloadAccounts() {
        accounts = []
        accountError = ""
        if (!enrolled) return
        api.get("/v1/gmail/accounts", function(error, response) {
            if (error || !response || !Array.isArray(response.body)) {
                accountError = qsTr("Account details unavailable while offline.")
                return
            }
            accounts = response.body
        })
    }
    Component.onCompleted: reloadAccounts()
    Item {
        implicitHeight: settingsColumn.implicitHeight
    ColumnLayout {
        id: settingsColumn
        width: Math.min(parent.width, 720)
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 12
        Kirigami.Heading { text: root.title; level: 2; color: root.ink; Layout.fillWidth: true }
        QQC2.Label { text: qsTr("Server URL"); color: root.ink }
        QQC2.TextField {
            id: serverField
            objectName: "settingsServerField"
            text: root.api ? root.api.baseUrl : ""
            Layout.fillWidth: true
            implicitHeight: 54
            color: root.ink
            inputMethodHints: Qt.ImhUrlCharactersOnly | Qt.ImhNoPredictiveText
            background: Rectangle { radius: 4; color: root.surface; border.color: "#879b99" }
        }
        QQC2.Label {
            visible: !root.serverUrlValid
            text: qsTr("Enter a valid HTTP or HTTPS server URL.")
            color: "#b3261e"
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        MailActionButton {
            objectName: "saveServerButton"
            text: qsTr("Save server URL")
            enabled: root.serverUrlValid
            Layout.fillWidth: true
            onClicked: {
                root.api.baseUrl = serverField.text.trim()
                root.saveStatus = qsTr("Server URL saved.")
                root.reloadAccounts()
            }
        }
        QQC2.Label { text: root.saveStatus; visible: text !== ""; color: root.mutedInk; Layout.fillWidth: true }
        Kirigami.Heading { text: qsTr("Enrollment"); level: 3; color: root.ink; Layout.topMargin: 12 }
        QQC2.Label {
            objectName: "enrollmentStatus"
            text: root.enrolled ? qsTr("Enrolled") : qsTr("Not enrolled")
            color: root.enrolled ? root.accent : root.mutedInk
            Layout.fillWidth: true
        }
        QQC2.Label {
            objectName: "settingsDeviceSummary"
            text: !root.enrolled ? qsTr("Use Enroll device in the inbox menu to connect.")
                : identity.serverUrl === root.api.baseUrl && identity.deviceId
                ? qsTr("Device: %1\nDevice ID: %2\nTenant: %3").arg(identity.deviceName).arg(identity.deviceId).arg(identity.tenantId)
                : qsTr("Enrollment is saved on this device. Device details were not saved by the previous version.")
            textFormat: Text.PlainText
            color: root.mutedInk
            wrapMode: Text.WrapAnywhere
            Layout.fillWidth: true
        }
        QQC2.Label { text: qsTr("Connected accounts"); color: root.ink; Layout.topMargin: 12 }
        QQC2.Label {
            objectName: "accountSummary"
            text: root.accountError || (root.accounts.length ? root.accounts.map(account => account.address).join("\n") : qsTr("No accounts connected."))
            textFormat: Text.PlainText
            color: root.mutedInk
            wrapMode: Text.WrapAnywhere
            Layout.fillWidth: true
        }
        MailActionButton {
            objectName: "settingsCheckUpdates"
            text: root.updater && root.updater.busy ? qsTr("Checking updates…") : qsTr("Check updates")
            enabled: !!root.updater && root.updater.supported && !root.updater.busy
            Layout.fillWidth: true
            Layout.topMargin: 12
            onClicked: root.updater.checkForUpdates()
        }
        QQC2.Label {
            visible: !root.updater || !root.updater.supported
            text: qsTr("In-app updates are available on Android.")
            color: root.mutedInk
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
    }
    }
}
