// SPDX-License-Identifier: MIT
// Visible exit for every page pushed onto the inbox stack; same button as the mail reader.
import QtQuick
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

Item {
    id: bar
    // The pushed page this bar belongs to; its StackView is popped on click.
    required property Item stackPage
    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3) + 2 * Math.max(18, Kirigami.Units.largeSpacing)
    MailActionButton {
        objectName: "backToInbox"
        text: qsTr("Back to inbox")
        icon.name: "go-previous"
        icon.color: "#263b3a"
        x: Math.max(Math.max(18, Kirigami.Units.largeSpacing), (bar.width - 1200) / 2)
        y: Math.max(18, Kirigami.Units.largeSpacing)
        onClicked: bar.stackPage.QQC2.StackView.view.pop()
    }
}
