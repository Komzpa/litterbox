import QtQuick
import QtQuick.Controls as QQC2
import QtTest
import QtWebEngine
import "../pages" as Pages

// Real Gmail bodies (tests/fixtures/mail-*.html, read once from the endpoint,
// links and addresses scrubbed) rendered by the page the app ships.
TestCase {
    id: testCase
    name: "MailDetailPage"
    width: 598
    height: 879
    when: windowShown

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
        storeStub.online = false
    }

    function relativeLuminance(color) {
        function channel(value) { return value <= 0.03928 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4) }
        return 0.2126 * channel(color.r) + 0.7152 * channel(color.g) + 0.0722 * channel(color.b)
    }

    // Lays out one message at the given width and returns what a reader sees:
    // document width against its column, images against the column, any
    // image painted over text, and the longest GUI-thread stall while opening.
    function openMessage(html, viewWidth) {
        storeStub.cached = {"card": {html: html, source_url: "https://mail.google.com/mail/?authuser=reader%40example.test#all/fixture"}}
        testCase.width = viewWidth
        mailFixtures.startHeartbeat()
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card", width: viewWidth, height: 879, openLinks: false
        })
        verify(page)
        let body = null
        tryVerify(function() { body = findChild(page, "body"); return body !== null }, 5000)
        if (body.runJavaScript === undefined) {
            // Qt rich text (the renderer before this change): reuse its own
            // layout measurements; it has no image-over-text geometry to query.
            wait(300)
            const blocked = mailFixtures.stopHeartbeat()
            return {page: page, body: body, blockedMs: blocked, overflow: body.contentWidth - body.width, imageOverflow: 0, overlaps: [], resized: [], text: body.getText(0, body.length)}
        }
        // contentsSize is not reported under the offscreen platform; the
        // finished load plus the document's own geometry below is the oracle.
        tryVerify(function() { return !body.loading && body.loadProgress === 100 && body.height > 100 }, 15000)
        wait(200)
        const blocked = mailFixtures.stopHeartbeat()
        let geometry = null
        // ApplicationWorld runs measurement only; the message's own scripts stay disabled.
        body.runJavaScript(`(function() {
            const doc = document.documentElement
            const images = Array.from(document.images).filter(i => i.getBoundingClientRect().width > 0)
                .map(i => i.getBoundingClientRect())
            const textRects = []
            const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
            while (walker.nextNode()) {
                if (!walker.currentNode.nodeValue.trim()) continue
                const range = document.createRange()
                range.selectNodeContents(walker.currentNode)
                for (const r of range.getClientRects()) if (r.width > 0 && r.height > 0) textRects.push({r: r, t: walker.currentNode.nodeValue.trim().slice(0, 40)})
            }
            const overlaps = []
            for (const i of images) for (const x of textRects) {
                const w = Math.min(i.right, x.r.right) - Math.max(i.left, x.r.left)
                const h = Math.min(i.bottom, x.r.bottom) - Math.max(i.top, x.r.top)
                if (w > 2 && h > 2) overlaps.push(x.t)
            }
            // Gmail honors a sender's inline pixel height unless the column is
            // too narrow for the image; report every image drawn at another size.
            const resized = Array.from(document.images).filter(i => {
                const declared = /(?:^|;)\s*height\s*:\s*(\d+)px/i.exec(i.getAttribute("style") || "")
                const box = i.getBoundingClientRect()
                return declared && box.width > 0 && box.width < doc.clientWidth - 1 &&
                    Math.abs(box.height - Number(declared[1])) > 1
            }).map(i => (i.alt || "image") + " drawn " + Math.round(i.getBoundingClientRect().height) + "px tall")
            return {overflow: doc.scrollWidth - doc.clientWidth, resized: resized,
                    imageOverflow: Math.max(0, ...images.map(i => i.right - doc.clientWidth)),
                    overlaps: overlaps, text: document.body.innerText}
        })()`, WebEngineScript.ApplicationWorld, function(result) { geometry = result })
        tryVerify(function() { return geometry !== null }, 5000)
        geometry.page = page
        geometry.body = body
        geometry.blockedMs = blocked
        return geometry
    }

    function test_realMailFitsColumnWithoutOverlap_data() {
        const rows = []
        for (const fixture of ["google", "github", "nyt", "linkedin", "plain"])
            for (const viewWidth of [598, 1440])
                rows.push({tag: fixture + "-" + viewWidth, fixture: fixture, viewWidth: viewWidth})
        return rows
    }

    function test_realMailFitsColumnWithoutOverlap(row) {
        const shown = openMessage(mailFixtures.read(row.fixture), row.viewWidth)
        console.log("gui-thread-block", row.tag, shown.blockedMs.toFixed(1), "ms")
        verify(shown.overflow <= 1, row.tag + " runs " + shown.overflow + "px past the card's right edge")
        verify(shown.imageOverflow <= 1, row.tag + " image runs " + shown.imageOverflow + "px past the column")
        compare(shown.overlaps, [], row.tag + " images are painted over text")
        compare(shown.resized, [], row.tag + " images ignore the sender's pixel height")
        verify(shown.blockedMs < 250, row.tag + " blocked the GUI thread for " + shown.blockedMs.toFixed(1) + " ms")
        // The message column follows the window; it is never squeezed narrow.
        verify(shown.body.width >= Math.min(row.viewWidth, 1200) - 120,
               row.tag + " body is squeezed to " + shown.body.width + "px")
    }

    function test_longMessageWheelScroll() {
        const html = "<html><body>" + "<p>Long cached message paragraph.</p>".repeat(100) + "</body></html>"
        const shown = openMessage(html, 598)
        let position = null
        function readPosition() {
            shown.body.runJavaScript("document.scrollingElement.scrollTop", WebEngineScript.ApplicationWorld,
                function(value) { position = value })
        }
        readPosition()
        tryVerify(function() { return position !== null })
        compare(position, 0)
        mouseWheel(shown.body, shown.body.width / 2, shown.body.height / 2, 0, -120, Qt.NoButton)
        tryVerify(function() { readPosition(); return position > 0 }, 5000,
                  "A wheel event must scroll the open long message")
        console.log("MAIL_WHEEL", 0, "->", position)
        mouseWheel(shown.body, shown.body.width / 2, shown.body.height / 2, 0, 120, Qt.NoButton)
        tryVerify(function() { readPosition(); return position === 0 }, 5000,
                  "Reverse wheel must return to the message start")
    }

    function test_googleHeadingAndCallToActionStayReadable() {
        const shown = openMessage(mailFixtures.read("google"), 598)
        verify(shown.text.indexOf("Datasets structured data issues detected in") >= 0)
        verify(shown.text.indexOf("Fix Datasets structured data issues") >= 0)
    }

    function test_navigationButtonsAreLightWithVisibleIcons() {
        const shown = openMessage("<p>Hello <b>world</b></p>", 598)
        for (const name of ["backToInbox", "openInGmail"]) {
            const button = findChild(shown.page, name)
            verify(button && button.visible, name)
            verify(button.background && button.background.color !== undefined, name + " has no light fill")
            verify(relativeLuminance(button.background.color) > 0.8, name + " fill is dark: " + button.background.color)
            verify(button.icon.name.length > 0, name + " has no icon")
            verify(relativeLuminance(button.icon.color) < 0.1, name + " icon is not dark ink on the light fill")
        }
    }

    function test_failedRefreshKeepsReadableCachedMessage() {
        storeStub.online = true
        storeStub.cached = {"card-8": {html: "<p>Keep reading while offline.</p>"}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card-8", openLinks: false
        })
        compare(page.loading, true)
        storeStub.mailBodyFailed("card-8")
        compare(page.loading, false)
        compare(page.html, "<p>Keep reading while offline.</p>")
        compare(page.errorText, "")
    }

    // Fixed arrival instant 2026-10-04T07:43:00Z rendered in the pinned test
    // zone (Asia/Tbilisi, UTC+4): 11:43. Branches cover today, this year and
    // older mail.
    function test_arrivalLineShowsSenderAndLocalArrival() {
        storeStub.cached = {"card": {html: "<p>Hi</p>"}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card", openLinks: false,
            card: {source: "mail", sender_name: "Cerebras Systems",
                   sender_address: "welcome@cerebras.net", received_at: "2026-10-04T07:43:00Z"},
            arrivalNow: new Date("2026-10-04T08:00:00Z").getTime()
        })
        verify(page)
        const line = findChild(page, "mailArrival")
        verify(line, "arrival line missing under the subject")
        compare(line.text, "Cerebras Systems <welcome@cerebras.net> · 11:43")
        // Same year, another day: day + month + time.
        page.arrivalNow = new Date("2026-11-20T08:00:00Z").getTime()
        compare(line.text, "Cerebras Systems <welcome@cerebras.net> · 4 Oct 11:43")
        // Another year: full date.
        page.arrivalNow = new Date("2027-01-02T08:00:00Z").getTime()
        compare(line.text, "Cerebras Systems <welcome@cerebras.net> · 4 Oct 2026 11:43")
    }

    function test_missingArrivalShowsNoStraySeparator() {
        storeStub.cached = {"card": {html: "<p>Hi</p>"}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card", openLinks: false,
            card: {source: "mail", sender_name: "Cerebras Systems",
                   sender_address: "welcome@cerebras.net"}
        })
        verify(page)
        const line = findChild(page, "mailArrival")
        verify(line, "sender line missing under the subject")
        compare(line.text, "Cerebras Systems <welcome@cerebras.net>")
        verify(line.text.indexOf("·") < 0, "stray separator without arrival time: " + line.text)
    }
}
