import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

QQC2.Button {
    id: root
    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
    // Explicit padding and width: the Breeze desktop style otherwise sizes the
    // button to its own label and lets the custom content touch the border.
    leftPadding: Kirigami.Units.largeSpacing
    rightPadding: Kirigami.Units.largeSpacing
    implicitWidth: implicitContentWidth + leftPadding + rightPadding
    icon.color: "#263b3a"
    contentItem: RowLayout {
        spacing: Kirigami.Units.smallSpacing
        Kirigami.Icon {
            source: root.icon.name
            isMask: true
            color: "#263b3a"
            implicitWidth: Kirigami.Units.iconSizes.small
            implicitHeight: Kirigami.Units.iconSizes.small
        }
        QQC2.Label { text: root.text; color: "#263b3a" }
    }
    background: Rectangle {
        radius: Kirigami.Units.cornerRadius
        color: root.down ? "#e8eeed" : root.hovered ? "#eef3f2" : "#ffffff"
        border.width: root.visualFocus ? 2 : 1
        border.color: root.visualFocus ? "#263b3a" : "#dce5e3"
    }
}
