import QtQuick
import QtTest

TestCase {
    name: "ReaderIcons"
    when: windowShown
    Component { id: imageComponent; Image { width: 24; height: 24 } }
    function test_bundledNavigationIcons_data() {
        return [{tag: "back", name: "go-previous"}, {tag: "archive", name: "mail-archive"}]
    }
    function test_bundledNavigationIcons(row) {
        const image = createTemporaryObject(imageComponent, this, {
            source: Qt.resolvedUrl("../icons/litterbox/actions/24/" + row.name + ".svg")
        })
        tryCompare(image, "status", Image.Ready)
        verify(image.sourceSize.width > 0)
    }
}
