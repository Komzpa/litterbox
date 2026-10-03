import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "MailDetailPage"
    width: 500
    height: 500

    QtObject {
        id: storeStub
        property var cached: ({})
        property bool online: false
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) {}
    }

    Component { id: pageComponent; Pages.MailDetailPage {} }

    function init() {
        visible = true
        storeStub.cached = ({})
        storeStub.online = true
    }

    function test_loadsCachedBodyAndGmailLink() {
        storeStub.cached = {
            "card-7": { html: "<p>Hello <b>world</b></p>", source_url: "https://mail.google.com/mail/u/0/#all/thread-123" }
        }
        var page = createTemporaryObject(pageComponent, this, {
            store: storeStub, cardId: "card-7", openLinks: false
        })
        verify(page)
        compare(page.html, "<p>Hello <b>world</b></p>")
        compare(page.sourceUrl, "https://mail.google.com/mail/u/0/#all/thread-123")
        compare(page.errorText, "")
        verify(findChild(page, "openInGmail").visible)
    }

    function test_failedRefreshKeepsReadableCachedMessage() {
        storeStub.cached = {"card-8": {html: "<p>Keep reading while offline.</p>"}}
        const page = createTemporaryObject(pageComponent, this, {
            store: storeStub, cardId: "card-8", openLinks: false
        })
        compare(page.loading, true)
        storeStub.mailBodyFailed("card-8")
        compare(page.loading, false)
        const body = findChild(page, "body")
        compare(body.getText(0, body.length), "Keep reading while offline.")
        compare(page.errorText, "")
    }

    function test_plainTextWrapsWithoutLosingLiteralMarkup() {
        storeStub.online = false
        const line = "A long paragraph keeps literal <not HTML> and A & B visible. ".repeat(8)
        storeStub.cached = {"plain": {html: "<pre>Hello reader,\n\n" + line.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;") + "\n\nGoodbye.</pre>"}}
        const page = createTemporaryObject(pageComponent, this, {
            store: storeStub, cardId: "plain", width: 400, height: 500, openLinks: false
        })
        const body = findChild(page, "body")
        verify(body)
        wait(100)
        verify(body.width > 0)
        verify(body.contentWidth <= body.width + 1, "Plain text must wrap within the reading column")
        verify(body.getText(0, body.length).indexOf("<not HTML> and A & B") >= 0)
        verify(body.lineCount > 5, "Long paragraphs need additional wrapped lines")
    }
}
