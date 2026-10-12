// SPDX-License-Identifier: MIT
// Gmail accounts: connect via OAuth URL, list, disconnect.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root
    objectName: "gmailAccountsPage"
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    Kirigami.Theme.inherit: false
    Kirigami.Theme.textColor: ink
    Kirigami.Theme.disabledTextColor: mutedInk
    Kirigami.Theme.backgroundColor: canvas
    Kirigami.Theme.alternateBackgroundColor: surface
    Kirigami.Theme.highlightColor: accent
    Kirigami.Theme.focusColor: accent
    Kirigami.Theme.hoverColor: accent
    Material.theme: Material.Light
    Material.background: surface
    Material.foreground: ink
    Material.accent: accent
    Material.primary: accent
    palette.window: canvas
    palette.windowText: ink
    palette.base: surface
    palette.text: ink
    palette.button: surface
    palette.buttonText: ink
    palette.highlight: accent
    palette.highlightedText: surface
    padding: Math.max(18, Kirigami.Units.largeSpacing)
    background: Rectangle { color: canvas }
    header: BackToInboxBar { stackPage: root }

    // Contract: { get(path, cb), post(path, body, cb), del(path, cb) }.
    // Set openLinks to false in tests to avoid spawning a browser.
    property var api
    property bool openLinks: true
    property var accountsLoaded: function(accounts) {}
    property bool connecting: false
    property var accounts: [] // { id, address, status }
    property bool loading: false
    property string errorText: ""
    property string lastConnectUrl: ""

    title: qsTr("Gmail Accounts")

    function reload() {
        loading = true
        errorText = ""
        api.get("/v1/gmail/accounts", function (error, response) {
            loading = false
            if (error || !response || !Array.isArray(response.body)) {
                errorText = qsTr("Could not load the account list.")
                return
            }
            accounts = response.body
            accountsLoaded(accounts)
            if (accounts.every(account => account.status === "active")) connecting = false
        })
    }

    function connectAccount() {
        errorText = ""
        api.post("/v1/gmail/connect", {}, function (error, response) {
            if (error || !response || !response.body || !response.body.authorization_url) {
                errorText = qsTr("Could not start the Gmail connection.")
                return
            }
            lastConnectUrl = response.body.authorization_url
            connecting = true
            reload()
            if (root.openLinks)
                Qt.openUrlExternally(response.body.authorization_url)
        })
    }

    function disconnect(id) {
        api.del("/v1/gmail/accounts/" + id, function (error) {
            if (error) {
                errorText = qsTr("Could not disconnect the account.")
                return
            }
            reload()
        })
    }

    Component.onCompleted: {
        if (api)
            reload()
    }

    Connections {
        target: root.Window.window
        function onActiveChanged() { if (root.Window.window.active) root.reload() }
    }
    Timer {
        interval: 2000
        repeat: true
        running: root.connecting
        onTriggered: root.reload()
    }

    Item {
        implicitHeight: accountColumn.implicitHeight
    ColumnLayout {
        id: accountColumn
        objectName: "accountsColumn"
        width: Math.min(parent.width, 720)
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 18

        Kirigami.Heading { text: root.title; level: 2; color: root.ink; Layout.fillWidth: true }
        QQC2.Button {
            id: connectButton
            objectName: "connectButton"
            text: qsTr("Connect account…")
            icon.name: "mail-receive"
            icon.width: Kirigami.Units.iconSizes.small
            icon.height: Kirigami.Units.iconSizes.small
            icon.color: root.surface
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            Layout.alignment: Qt.AlignLeft
            leftPadding: Kirigami.Units.largeSpacing
            rightPadding: Kirigami.Units.largeSpacing
            Material.theme: Material.Light
            Material.foreground: root.surface
            palette.button: root.accent
            palette.buttonText: root.surface
            background: Rectangle {
                radius: Kirigami.Units.cornerRadius
                color: connectButton.down ? "#286358" : connectButton.hovered ? "#326f65" : root.accent
                border.width: connectButton.visualFocus ? 2 : 0
                border.color: root.ink
            }
            contentItem: RowLayout {
                spacing: Kirigami.Units.smallSpacing
                Kirigami.Icon {
                    source: connectButton.icon.name
                    color: root.surface
                    implicitWidth: connectButton.icon.width
                    implicitHeight: connectButton.icon.height
                }
                QQC2.Label {
                    text: connectButton.text
                    color: root.surface
                    font: connectButton.font
                    Layout.fillWidth: true
                    verticalAlignment: Text.AlignVCenter
                }
            }
            onClicked: root.connectAccount()
        }

        QQC2.BusyIndicator {
            visible: root.loading
            Layout.alignment: Qt.AlignHCenter
        }

        QQC2.Label {
            visible: root.errorText !== ""
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }

        ListView {
            id: list
            objectName: "accountsList"
            Layout.fillWidth: true
            Layout.preferredHeight: Math.max(160, contentHeight)
            spacing: 12
            clip: true
            keyNavigationEnabled: true
            model: root.accounts
            activeFocusOnTab: true

            delegate: QQC2.ItemDelegate {
                id: accountRow
                width: list.width
                implicitHeight: 84
                padding: 12
                readonly property bool expired: modelData.status === "reauth_required"
                background: Rectangle { color: root.surface; radius: 6; border.color: "#dce5e3" }
                contentItem: RowLayout {
                    spacing: 12
                    ColumnLayout {
                        Layout.fillWidth: true
                        QQC2.Label { text: modelData.address; color: root.ink; Layout.fillWidth: true; elide: Text.ElideRight }
                        QQC2.Label { text: accountRow.expired ? qsTr("Access expired") : modelData.status === "active" ? qsTr("Connected") : ""; visible: text.length > 0; color: root.mutedInk; font: Kirigami.Theme.smallFont }
                    }
                    MailActionButton {
                        objectName: "reconnectButton"
                        text: qsTr("Reconnect")
                        visible: accountRow.expired
                        onClicked: root.connectAccount()
                    }
                    MailActionButton {
                        objectName: "disconnectButton"
                        text: qsTr("Disconnect")
                        onClicked: root.disconnect(modelData.id)
                    }
                }
            }

            QQC2.Label {
                anchors.centerIn: parent
                objectName: "emptyStateLabel"
                visible: list.count === 0 && !root.loading && root.errorText === ""
                text: qsTr("No accounts connected.")
            }
        }
    }
    }
}
