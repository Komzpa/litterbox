import QtQuick
Item {
    property var store
    property string cardId
    property string cardTitle
    property string accountName
    property var card
    property bool openLinks
    property bool embeddedMode
    property var requestNote
    property var requestArchiveUndo
    property var requestClose
    property var requestArchiveContinuation
    property bool ownerPopupVisible
    property real bottomPadding
    function reload() {}
}
