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
    // R67 sender-photo consent/status, server-reported only. The status
    // contract (GET /v1/gmail/sender-photos/status) carries per-account
    // photo_ready / authorization_required / required_scopes; the consent
    // contract (POST /v1/gmail/photo-consent with {account_id}) returns an
    // authorization_url bound to that account and performs no grant — the
    // callback verifies the approving Google identity against the binding.
    // No scope or Google config checks are invented here: every state shown
    // is what the server said.
    property var photoStatus: [] // [{ account_id, address, photo_ready, authorization_required, required_scopes }]
    property string photoStatusError: ""
    property string photoConsentUrl: ""
    property string photoConsentAccountId: ""
    property string photoConsentAddress: ""
    property string photoConsentError: ""
    property bool requestingPhotoConsent: false

    title: qsTr("Gmail Accounts")

    // Refresh updates in place: the busy spinner only covers the first load,
    // error labels change only when a reply changes them, and an unchanged
    // reply keeps the existing array so ListView/Repeater keep their
    // delegates. Clearing everything per request made the page blink on
    // every window activation and on the 2 s connect poll.
    property bool loadedOnce: false
    function sameJson(a, b) { return JSON.stringify(a) === JSON.stringify(b) }
    // Connect poll stays under the 60/min per-device limit: at most one
    // accounts-only request per 5 s, stopped by any failed reply or after
    // 3 minutes (36 ticks). Photo status is not polled; it loads once with
    // every full reload (activation, connect completion).
    property int connectPollTicks: 0
    readonly property int connectPollMaxTicks: 36
    function reloadAccountsOnly() {
        loading = true
        api.get("/v1/gmail/accounts", function (error, response) {
            loading = false
            loadedOnce = true
            if (error || !response || !Array.isArray(response.body)) {
                connecting = false
                errorText = qsTr("Could not load the account list.")
                return
            }
            errorText = ""
            if (!sameJson(accounts, response.body))
                accounts = response.body
            accountsLoaded(accounts)
            if (accounts.every(account => account.status === "active")) {
                connecting = false
                reloadPhotoStatus()
            }
        })
    }
    function reload() {
        loading = true
        // A consent link that was already opened is consumed server-side
        // (single-use state); drop the stale URL so the page never invites a
        // second click that can only fail, and let the status reload below
        // report the row's real post-grant state.
        photoConsentUrl = ""
        photoConsentAccountId = ""
        photoConsentAddress = ""
        api.get("/v1/gmail/accounts", function (error, response) {
            loading = false
            loadedOnce = true
            if (error || !response || !Array.isArray(response.body)) {
                // A failed reply (offline, 429 rate limit) ends the connect
                // poll: re-requesting only keeps the device token
                // over its rate limit and the page blinking.
                connecting = false
                errorText = qsTr("Could not load the account list.")
                return
            }
            errorText = ""
            if (!sameJson(accounts, response.body))
                accounts = response.body
            accountsLoaded(accounts)
            if (accounts.every(account => account.status === "active")) connecting = false
        })
        reloadPhotoStatus()
    }

    // Server-reported photo readiness per account; never guessed from the
    // account list and never inferred from Google configuration.
    // A failed reply also ends the connect poll so the device stays under
    // the per-device rate limit.
    function reloadPhotoStatus() {
        api.get("/v1/gmail/sender-photos/status", function (error, response) {
            if (error || !response || !response.body || !Array.isArray(response.body.accounts)) {
                connecting = false
                if (photoStatus.length > 0)
                    photoStatus = []
                photoStatusError = qsTr("Could not load sender photo status.")
                return
            }
            photoStatusError = ""
            if (!sameJson(photoStatus, response.body.accounts))
                photoStatus = response.body.accounts
        })
    }

    // Starts the incremental photo grant (Gmail.modify + both People photo
    // scopes) for one intended account. The server binds the consent state to
    // that account_id and returns the authorization_url; it performs no grant:
    // the user approves in their own browser through the explicit open action
    // below and the callback checks the approving Google identity against the
    // binding. This function never opens a browser itself.
    function requestPhotoConsent(accountId, address) {
        photoConsentError = ""
        requestingPhotoConsent = true
        api.post("/v1/gmail/photo-consent", { "account_id": accountId }, function (error, response) {
            requestingPhotoConsent = false
            if (error || !response || !response.body || !response.body.authorization_url) {
                photoConsentError = qsTr("Could not start photo authorization.")
                return
            }
            photoConsentUrl = response.body.authorization_url
            photoConsentAccountId = accountId
            photoConsentAddress = address
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
            connectPollTicks = 0
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
        // Returning to the window (from the browser consent) is the reload
        // that reports the connect result; the 5 s poll is only for while
        // the browser has focus, so it ends here instead of running forever
        // on an account that stays reauth_required.
        function onActiveChanged() {
            if (!root.Window.window.active) return
            root.connecting = false
            root.reload()
        }
    }
    Timer {
        interval: 5000
        repeat: true
        running: root.connecting
        onTriggered: {
            root.connectPollTicks++
            if (root.connectPollTicks >= root.connectPollMaxTicks) {
                root.connecting = false
                return
            }
            root.reloadAccountsOnly()
        }
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
            objectName: "accountsBusy"
            visible: root.loading && !root.loadedOnce
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
                visible: list.count === 0 && root.loadedOnce && root.errorText === ""
                text: qsTr("No accounts connected.")
            }
        }

        // R67 sender-photo consent/status section. Each row's state is the
        // server's (photo_ready / authorization_required), the required
        // scopes are the server's list, and the only action here is asking
        // the server for the incremental grant URL and showing it; the user
        // opens it themselves.
        Kirigami.Heading {
            text: qsTr("Sender photos")
            level: 3
            color: root.ink
            visible: root.photoStatus.length > 0 || root.photoStatusError !== ""
            Layout.fillWidth: true
        }

        QQC2.Label {
            objectName: "photoStatusError"
            visible: root.photoStatusError !== ""
            text: root.photoStatusError
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }

        Repeater {
            model: root.photoStatus
            delegate: ColumnLayout {
                id: photoRow
                objectName: "photoStatusRow"
                Layout.fillWidth: true
                spacing: 6
                readonly property bool needsAuth: !!modelData.authorization_required
                readonly property bool photoReady: !!modelData.photo_ready

                QQC2.Label {
                    objectName: "photoStatusAddress"
                    text: modelData.address
                    color: root.ink
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
                QQC2.Label {
                    objectName: "photoStateLabel"
                    text: photoRow.photoReady ? qsTr("Sender photos ready")
                        : photoRow.needsAuth ? qsTr("Sender photos need authorization")
                        : qsTr("Authorized — photos appear after they sync")
                    color: photoRow.needsAuth ? Kirigami.Theme.negativeTextColor : root.mutedInk
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                QQC2.Label {
                    objectName: "photoRequiredScopes"
                    visible: photoRow.needsAuth && (modelData.required_scopes || []).length > 0
                    text: (modelData.required_scopes || []).join("\n")
                    color: root.mutedInk
                    font: Kirigami.Theme.smallFont
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                MailActionButton {
                    objectName: "photoConsentButton"
                    text: root.requestingPhotoConsent ? qsTr("Starting authorization…") : qsTr("Authorize sender photos…")
                    visible: photoRow.needsAuth
                    enabled: !root.requestingPhotoConsent
                    onClicked: root.requestPhotoConsent(modelData.account_id, modelData.address)
                }
            }
        }

        QQC2.Label {
            objectName: "photoConsentError"
            visible: root.photoConsentError !== ""
            text: root.photoConsentError
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }

        QQC2.Label {
            objectName: "photoConsentPrompt"
            visible: root.photoConsentUrl !== ""
            text: root.photoConsentAddress !== ""
                ? qsTr("Open this authorization link to grant sender photo access for %1. It is bound to that account: approving with another Google account is refused and stores nothing. Litterbox never grants it for you.").arg(root.photoConsentAddress)
                : qsTr("Open this authorization link to grant sender photo access. Litterbox never grants it for you.")
            color: root.ink
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.TextField {
            objectName: "photoConsentUrlText"
            visible: root.photoConsentUrl !== ""
            readOnly: true
            text: root.photoConsentUrl
            color: root.ink
            selectByMouse: true
            Layout.fillWidth: true
        }
        MailActionButton {
            objectName: "photoConsentOpenButton"
            text: qsTr("Open authorization page")
            visible: root.photoConsentUrl !== ""
            icon.name: "document-open"
            onClicked: {
                if (root.openLinks)
                    Qt.openUrlExternally(root.photoConsentUrl)
            }
        }
    }
    }
}
