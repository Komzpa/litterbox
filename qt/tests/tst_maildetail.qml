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
        property var requested: []
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) { requested.push(cardId) }
    }

    Component { id: pageComponent; Pages.MailDetailPage {} }

    function init() {
        visible = true
        storeStub.cached = ({})
        storeStub.online = true
        storeStub.requested = []
    }

    function test_loadsCachedBodyAndGmailLink() {
        storeStub.cached = {
            "card-7": { html: "<p>Hello <b>world</b></p>", source_url: "https://mail.google.com/mail/u/0/#all/thread-123" }
        }
        var page = createTemporaryObject(pageComponent, this, {
            store: storeStub, cardId: "card-7", openLinks: false
        })
        verify(page)
        compare(storeStub.requested, ["card-7"])
        compare(page.html, "<p>Hello <b>world</b></p>")
        compare(page.sourceUrl, "https://mail.google.com/mail/u/0/#all/thread-123")
        compare(page.errorText, "")
        verify(findChild(page, "openInGmail").visible)
    }

    function test_errorDoesNotExposeStaleBody() {
        storeStub.online = false
        var page = createTemporaryObject(pageComponent, this, {
            store: storeStub, cardId: "card-8", openLinks: false
        })
        verify(page)
        compare(page.html, "")
        compare(page.sourceUrl, "")
        verify(page.errorText.length > 0)
    }
}
