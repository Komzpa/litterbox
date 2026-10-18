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
    function openMessage(html, viewWidth, renderer) {
        storeStub.cached = {"card": {html: html, source_url: "https://mail.google.com/mail/?authuser=reader%40example.test#all/fixture"}}
        testCase.width = viewWidth
        mailFixtures.startHeartbeat()
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card", width: viewWidth, height: 879, openLinks: false,
            bodySource: renderer === "android" ? "MailBodyAndroid.qml" : "MailBodyDesktop.qml"
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

    function test_phoneMailWidth_data() {
        const rows = []
        for (const fixture of ["googleone", "github", "newsletter", "plain"])
            for (const renderer of ["android", "desktop"])
                for (const viewWidth of [360, 412, 1440])
                    rows.push({tag: fixture + "-" + renderer + "-" + viewWidth, fixture: fixture, renderer: renderer, viewWidth: viewWidth})
        return rows
    }
    function test_phoneMailWidth(row) {
        const fixture = row.fixture === "plain" ? "plain" : "phone-" + row.fixture
        const shown = openMessage(mailFixtures.read(fixture), row.viewWidth, row.renderer)
        if (row.viewWidth !== 360) {
            const shot = grabImage(shown.page)
            shot.save(mailFixtures.screenshotPath(row.tag + ".png"))
        }
        let right = 0
        for (const name of ["backToInbox", "openInGmail", "archiveMail"]) {
            const button = findChild(shown.page, name)
            const position = button.mapToItem(shown.page, 0, 0)
            right = Math.max(right, position.x + button.width)
        }
        console.log("PHONE_WIDTH", row.tag, "actionsRight=" + right, "viewport=" + row.viewWidth,
                    "bodyWidth=" + shown.body.width, "overflow=" + shown.overflow)
        verify(right <= row.viewWidth, "action row exceeds viewport: " + right)
        verify(shown.body.width <= row.viewWidth, "mail view exceeds viewport")
        verify(shown.overflow <= 1, "mail content exceeds viewport: " + shown.overflow)
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

    function test_embeddedBackInvokesCloseCallbackAndCanReopen() {
        storeStub.cached = {"embedded": {html: "<p>Retained reader</p>"}}
        let closeCount = 0
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "embedded", cardTitle: "Retained field report",
            embeddedMode: true, width: 598, height: 879, openLinks: false,
            requestClose: function() { ++closeCount; page.visible = false }
        })
        verify(page)
        const back = findChild(page, "backToInbox")
        verify(back && back.visible)
        mouseClick(back)
        compare(closeCount, 1)
        compare(page.visible, false, "embedded Back must call the owner's real visibility transition")
        page.closeReader()
        compare(closeCount, 1, "a repeated close must not invoke the callback twice")
        page.visible = true
        tryCompare(page, "closing", false)
        mouseClick(back)
        compare(closeCount, 2, "the same retained page must be closable after reopening")
        compare(page.visible, false)
    }

    // Fixed arrival instant 2026-10-04T07:43:00Z rendered in the pinned test
    // zone (Asia/Tbilisi, UTC+4): 11:43. Branches cover today, this year and
    // older mail.
    function test_arrivalLineShowsSenderAndLocalArrival() {
        storeStub.cached = {"card": {html: "<p>Hi</p>"}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card", cardTitle: "Synthetic arrival report", openLinks: false,
            width: 598, height: 879,
            card: {source: "mail", sender_name: "Cerebras Systems",
                   sender_address: "welcome@cerebras.net", received_at: "2026-10-04T07:43:00Z"},
            arrivalNow: new Date("2026-10-04T08:00:00Z").getTime()
        })
        verify(page)
        const line = findChild(page, "mailArrival")
        verify(line, "arrival line missing under the subject")
        compare(line.text, "Cerebras Systems <welcome@cerebras.net> · 11:43")
        const title = findChild(page, "mailBarTitle")
        const bar = findChild(page, "mailAppBar")
        const document = findChild(page, "messageDocument")
        verify(title && title.visible && line.visible && bar && document)
        tryVerify(function() { return title.width > 0 && line.width > 0 && document.height > 0 })
        for (const landmark of [title, line]) {
            const p = landmark.mapToItem(bar, 0, 0)
            verify(landmark.height > 0 && p.x >= 0 && p.y >= 0 &&
                   p.x + landmark.width <= bar.width + 1 && p.y + landmark.height <= bar.height + 1,
                   landmark.objectName + " must fit inside the reader header")
        }
        const titleBox = title.mapToItem(bar, 0, 0)
        const arrivalBox = line.mapToItem(bar, 0, 0)
        verify(arrivalBox.y >= titleBox.y + title.height - 1, "source/arrival landmark belongs below the subject")
        verify(title.font.pointSize > line.font.pointSize && title.font.weight > line.font.weight,
               "reader subject must dominate source/arrival metadata")
        const barBox = bar.mapToItem(page, 0, 0)
        const documentBox = document.mapToItem(page, 0, 0)
        verify(documentBox.y >= barBox.y + bar.height - 1, "message document must not overlap header landmarks")
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
    function test_originalThreadUsesNativeDisclosureInsideSingleDocumentScroll() {
        const historyText = "Second field note. " + "The survey records preserve the river level, bridge observations, and the original field measurements. ".repeat(80)
        storeStub.cached = {"thread": {source_url: "https://mail.synthetic.test/thread/42", messages: [
            {id: "orig-1", gmail_message_id: "gmail-1", sender_name: "Older historian", sender_address: "old@synthetic.test",
             received_at: "2026-10-02T10:00:00Z", html: "<p>First <b>field</b> note</p>"},
            {id: "orig-2", gmail_message_id: "gmail-2", sender_name: "Newer historian", sender_address: "new@synthetic.test",
             received_at: "2026-10-03T11:00:00Z", html: "<p>" + historyText + "</p>"},
            {id: "orig-3", gmail_message_id: "gmail-3", sender_name: "Latest historian", sender_address: "latest@synthetic.test",
             received_at: "2026-10-04T12:00:00Z", html: "<p>Latest field note</p>"}
        ]}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "thread", cardTitle: "Synthetic field report", openLinks: false,
            card: {source: "mail", title: "Synthetic field report"}, width: 598, height: 879,
            arrivalNow: new Date("2026-10-05T12:00:00Z").getTime()
        })
        verify(page)
        compare(page.html, "<p>Latest field note</p>")
        const body = findChild(page, "body")
        verify(body && body.runJavaScript !== undefined)
        tryVerify(function() { return !body.loading && body.loadProgress === 100 }, 15000)
        let result = null
        body.runJavaScript(`({details: Array.from(document.querySelectorAll('details')).map(d => ({open: d.open,
            summary: d.querySelector('summary').innerText, body: d.querySelector('.lb-original').textContent.trim()})),
            latest: document.querySelector('.lb-latest').innerText, originalHtml: Array.from(document.querySelectorAll('.lb-original')).map(e => e.innerHTML),
            sections: document.querySelectorAll('.lb-original').length,
            scrollHeight: document.scrollingElement.scrollHeight,
            clientHeight: document.scrollingElement.clientHeight,
            nestedScrolls: Array.from(document.querySelectorAll('*')).filter(e => e.scrollHeight > e.clientHeight + 2 && getComputedStyle(e).overflowY !== 'visible').length})`,
            WebEngineScript.ApplicationWorld, value => result = value)
        tryVerify(function() { return result !== null })
        compare(result.details.map(d => d.open), [false, false])
        compare(result.details.map(d => d.body), ["First field note", historyText.trim()])
        verify(result.originalHtml[0].indexOf("<b>field</b>") >= 0, "Collapsed originals retain their HTML")
        verify(result.details[0].summary.indexOf("Older historian <old@synthetic.test>") >= 0)
        verify(result.details[1].summary.indexOf("Newer historian <new@synthetic.test>") >= 0)
        verify(result.latest.indexOf("Latest historian <latest@synthetic.test>") >= 0)
        verify(result.latest.indexOf("Latest field note") < 0)
        compare(result.sections, 3)
        compare(result.nestedScrolls, 0)
        let summaryBox = null
        body.runJavaScript(`(() => { const summary = document.querySelector('details summary');
            summary.scrollIntoView(); const box = summary.getBoundingClientRect();
            return {x: box.x + box.width / 2, y: box.y + box.height / 2}; })()`,
            WebEngineScript.ApplicationWorld, value => summaryBox = value)
        tryVerify(function() { return summaryBox !== null })
        mouseClick(body, summaryBox.x, summaryBox.y)
        let opened = null
        body.runJavaScript(`(() => { const details = document.querySelector('details');
            const original = details.querySelector('.lb-original');
            return {open: details.open, text: original.innerText, height: original.getBoundingClientRect().height,
                nestedScrolls: Array.from(document.querySelectorAll('*')).filter(e => e.scrollHeight > e.clientHeight + 2 && getComputedStyle(e).overflowY !== 'visible').length}; })()`,
            WebEngineScript.ApplicationWorld, value => opened = value)
        tryVerify(function() { return opened !== null })
        verify(opened.open && opened.height > 0)
        compare(opened.text.trim(), "First field note", "Clicking the native summary renders the original")
        compare(opened.nestedScrolls, 0)
        summaryBox = null
        body.runJavaScript(`(() => { const summary = document.querySelectorAll('details summary')[1];
            summary.scrollIntoView(); const box = summary.getBoundingClientRect();
            return {x: box.x + box.width / 2, y: box.y + box.height / 2}; })()`,
            WebEngineScript.ApplicationWorld, value => summaryBox = value)
        tryVerify(function() { return summaryBox !== null })
        mouseClick(body, summaryBox.x, summaryBox.y)
        let scroll = null
        body.runJavaScript(`(() => { const details = document.querySelectorAll('details')[1];
            const documentScroll = document.scrollingElement;
            documentScroll.scrollTop = documentScroll.scrollHeight;
            return {open: details.open, top: documentScroll.scrollTop,
                height: documentScroll.scrollHeight, viewport: documentScroll.clientHeight,
                nestedScrolls: Array.from(document.querySelectorAll('*')).filter(e => e.scrollHeight > e.clientHeight + 2 && getComputedStyle(e).overflowY !== 'visible').length}; })()`,
            WebEngineScript.ApplicationWorld, value => scroll = value)
        tryVerify(function() { return scroll !== null })
        verify(scroll.open && scroll.height > scroll.viewport && scroll.top > 0,
               "Expanded long history scrolls in the original document")
        compare(scroll.nestedScrolls, 0)
    }
}
