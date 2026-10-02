// SPDX-License-Identifier: MIT
// Gmail accounts: connect via OAuth URL, list, disconnect.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root
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
    padding: Kirigami.Units.largeSpacing
    background: Rectangle { color: canvas }

    // Contract: { get(path, cb), post(path, body, cb), del(path, cb) }.
    // Set openLinks to false in tests to avoid spawning a browser.
    property var api
    property bool openLinks: true

    property var accounts: [] // { id, address }
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

    ColumnLayout {
        width: Math.min(parent.width, 1200)
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: Kirigami.Units.largeSpacing

        QQC2.Button {
            id: connectButton
            objectName: "connectButton"
            text: qsTr("Connect account…")
            icon.name: "mail-receive"
            icon.width: Kirigami.Units.iconSizes.small
            icon.height: Kirigami.Units.iconSizes.small
            icon.color: root.surface
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            leftPadding: Kirigami.Units.largeSpacing
            rightPadding: Kirigami.Units.largeSpacing
            Material.foreground: root.surface
            palette.buttonText: root.surface
            background: Rectangle {
                radius: Kirigami.Units.cornerRadius
                color: connectButton.down ? "#286358" : connectButton.hovered ? "#326f65" : root.accent
                border.width: connectButton.visualFocus ? 2 : 0
                border.color: root.ink
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
            clip: true
            keyNavigationEnabled: true
            model: root.accounts
            activeFocusOnTab: true

            delegate: QQC2.ItemDelegate {
                width: list.width
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                contentItem: RowLayout {
                    spacing: Kirigami.Units.smallSpacing
                    QQC2.Label {
                        text: modelData.address
                        Layout.fillWidth: true
                        elide: Text.ElideRight
                    }
                    QQC2.Button {
                        objectName: "disconnectButton"
                        text: qsTr("Disconnect")
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        onClicked: root.disconnect(modelData.id)
                    }
                }
            }

            QQC2.Label {
                anchors.centerIn: parent
                visible: list.count === 0 && !root.loading
                text: qsTr("No accounts connected.")
            }
        }
    }
}
