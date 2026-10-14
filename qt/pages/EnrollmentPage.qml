// SPDX-License-Identifier: MIT
// Device enrollment: redeem an invite code for a bearer token.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami
import QtCore

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
    padding: Math.max(18, Kirigami.Units.largeSpacing)
    background: Rectangle { color: canvas }
    header: BackToInboxBar { stackPage: root }

    // api.post(path, body, cb) calls cb(error, {status, body}).
    property var api
    readonly property bool alreadyEnrolled: !!(token || (api && api.token))
    Settings {
        id: identity
        category: "enrollment"
        property string deviceId: ""
        property string tenantId: ""
        property string deviceName: ""
        property string serverUrl: ""
    }

    property bool busy: false
    readonly property bool serverUrlValid: /^https?:\/\/[a-z0-9.-]+(?::[0-9]+)?(?:[/?#][^\s]*)?$/i.test(serverField.text.trim())
    property string errorText: ""
    // Set on successful enrollment; stored bearer tokens are never displayed.
    property string token: ""
    property string deviceId: ""
    property string tenantId: ""

    signal enrolled(var device) // { token, device_id, tenant_id }

    title: qsTr("Connect to Server")
    // Text inputs otherwise consume Alt+Left as cursor movement before it
    // can bubble to the inbox stack's navigation handler.
    function handleFieldNavigation(event) {
        const view = root.QQC2.StackView.view
        if (event.key === Qt.Key_Left && (event.modifiers & Qt.AltModifier) && view && view.depth > 1) {
            view.pop()
            event.accepted = true
        }
    }

    function enroll() {
        errorText = ""
        const server = serverField.text.trim()
        const invite = inviteField.text.trim()
        const name = deviceNameField.text.trim()
        if (!serverUrlValid) {
            errorText = qsTr("Enter a valid HTTP or HTTPS server URL.")
            return false
        }
        if (invite === "" || name === "") {
            errorText = qsTr("Invite code and device name are required.")
            return false
        }
        // Persisted to QSettings server_url by the baseUrlChanged connection
        // in main.cpp; the next launch reuses it without environment variables.
        api.baseUrl = server
        api.post("/v1/devices/enroll", {
            invite_code: invite,
            device_name: name,
            platform: platformBox.currentValue
        }, function (error, response) {
            busy = false
            if (error || !response || !response.body || !response.body.token) {
                const status = response && response.status ? response.status : 0
                if (status === 0) {
                    errorText = qsTr("Can't reach the server at %1. Check the address and that you're on the home Wi-Fi.").arg(server)
                } else if (status === 410 || status === 404 || status === 400) {
                    errorText = qsTr("Invite is used, expired, or invalid.")
                } else {
                    errorText = qsTr("Server error %1.").arg(status)
                }
                return
            }
            token = response.body.token
            deviceId = response.body.device_id
            tenantId = response.body.tenant_id
            identity.setValue("deviceId", deviceId)
            identity.setValue("tenantId", tenantId)
            identity.setValue("deviceName", name)
            identity.setValue("serverUrl", server)
            identity.sync()
            api.token = token
            root.enrolled(response.body)
        })
        return true
    }

    Kirigami.FormLayout {
        visible: !root.alreadyEnrolled
        width: Math.min(parent.width, 1200)
        Layout.alignment: Qt.AlignHCenter

        Kirigami.Heading {
            text: root.title
            level: 2
            color: root.ink
            Kirigami.FormData.isSection: true
        }
        QQC2.TextField {
            id: serverField
            objectName: "serverField"
            Keys.onPressed: event => root.handleFieldNavigation(event)
            Kirigami.FormData.label: qsTr("Server:")
            text: api && api.baseUrl ? api.baseUrl : ""
            Layout.fillWidth: true
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            Material.theme: Material.Light
            Material.background: root.surface
            Material.foreground: root.ink
            palette.base: root.surface
            palette.text: root.ink
            color: root.ink
            background: Rectangle { radius: 4; color: root.surface; border.color: "#879b99" }
            // A committed "http" suggestion would silently corrupt the URL.
            inputMethodHints: Qt.ImhUrlCharactersOnly | Qt.ImhNoPredictiveText
        }
        QQC2.Label {
            objectName: "serverUrlError"
            visible: serverField.text.trim() !== "" && !root.serverUrlValid
            text: qsTr("Enter a valid HTTP or HTTPS server URL.")
            color: "#b3261e"
            wrapMode: Text.Wrap
            Kirigami.FormData.isSection: true
        }
        QQC2.TextField {
            id: inviteField
            objectName: "inviteField"
            Keys.onPressed: event => root.handleFieldNavigation(event)
            Kirigami.FormData.label: qsTr("Invite code:")
            // Invite codes are exact secrets: never autocorrect or uppercase them.
            inputMethodHints: Qt.ImhNoAutoUppercase | Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
            Layout.fillWidth: true
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            Material.theme: Material.Light
            Material.background: root.surface
            Material.foreground: root.ink
            palette.base: root.surface
            color: root.ink
            background: Rectangle { radius: 4; color: root.surface; border.color: "#879b99" }
            palette.text: root.ink
        }
        QQC2.TextField {
            id: deviceNameField
            objectName: "deviceNameField"
            Keys.onPressed: event => root.handleFieldNavigation(event)
            Kirigami.FormData.label: qsTr("Device name:")
            text: Qt.platform.os === "android" ? qsTr("Phone") : qsTr("Desktop")
            Layout.fillWidth: true
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            Material.theme: Material.Light
            Material.background: root.surface
            Material.foreground: root.ink
            palette.base: root.surface
            color: root.ink
            background: Rectangle { radius: 4; color: root.surface; border.color: "#879b99" }
            palette.text: root.ink
        }
        QQC2.ComboBox {
            id: platformBox
            objectName: "platformBox"
            Kirigami.FormData.label: qsTr("Platform:")
            model: ["linux", "android"]
            Component.onCompleted: currentIndex = Qt.platform.os === "android" ? 1 : 0
            Material.theme: Material.Light
            Material.background: root.surface
            Material.foreground: root.ink
            palette.base: root.surface
            background: Rectangle { radius: 4; color: root.surface; border.color: "#879b99" }
            palette.text: root.ink
            contentItem: QQC2.TextField {
                text: platformBox.displayText
                color: root.ink
                readOnly: true
                selectByMouse: false
                leftPadding: 8
                verticalAlignment: Text.AlignVCenter
                background: null
            }
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
        }

        QQC2.Label {
            visible: root.errorText !== ""
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Kirigami.FormData.isSection: true
        }

        QQC2.Button {
            id: enrollButton
            objectName: "enrollButton"
            text: root.busy ? qsTr("Enrolling…") : qsTr("Enroll")
            enabled: !root.busy && root.serverUrlValid
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            Layout.fillWidth: true
            Material.theme: Material.Light
            Material.background: root.accent
            Material.foreground: root.surface
            palette.button: root.accent
            palette.buttonText: root.surface
            background: Rectangle {
                radius: Kirigami.Units.cornerRadius
                color: !parent.enabled ? "#cbd5d3" : parent.down ? "#286358" : parent.hovered ? "#326f65" : root.accent
                border.width: parent.visualFocus ? 2 : 0
                border.color: root.ink
            }
            contentItem: QQC2.Label {
                text: enrollButton.text
                color: enrollButton.enabled ? root.surface : root.mutedInk
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                font: enrollButton.font
            }
            onClicked: root.enroll()
        }
    }

    ColumnLayout {
        visible: root.alreadyEnrolled
        width: Math.min(parent.width, 1200)
        Layout.alignment: Qt.AlignHCenter
        spacing: Kirigami.Units.smallSpacing

        QQC2.Label {
            text: qsTr("This device is enrolled. No invite code is needed.")
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.Label {
            objectName: "deviceSummary"
            text: identity.serverUrl === api.baseUrl && identity.deviceId
                ? qsTr("Device: %1 · Tenant: %2").arg(identity.deviceId).arg(identity.tenantId)
                : qsTr("Enrollment is saved on this device.")
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
    }
}
