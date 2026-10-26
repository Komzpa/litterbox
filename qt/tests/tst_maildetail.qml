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
        property var bodyRequests: []
        property var operations: []
        signal mailBodyChanged(string cardId)
        signal mailBodyFailed(string cardId)
        function cachedMailBody(cardId) { return cached[cardId] || {} }
        function requestMailBody(cardId) { bodyRequests = bodyRequests.concat([cardId]) }
        function enqueueOp(cardId, action, payload) {
            operations = operations.concat([{cardId: cardId, action: action, payload: payload}])
            return "test-operation"
        }
    }

    Component { id: pageComponent; Pages.MailDetailPage {} }

    function init() {
        visible = true
        storeStub.cached = ({})
        storeStub.online = false
        storeStub.bodyRequests = []
        storeStub.operations = []
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
        for (const name of ["backToInbox", "archiveMail", "openInGmail"]) {
            const button = findChild(shown.page, name)
            verify(button && button.visible, name)
            verify(button.background && button.background.color !== undefined, name + " has no light fill")
            verify(relativeLuminance(button.background.color) > 0.8, name + " fill is dark: " + button.background.color)
            verify(button.icon.name.length > 0, name + " has no icon")
            verify(relativeLuminance(button.icon.color) < 0.1, name + " icon is not dark ink on the light fill")
            verifyPaintedBarIcon(button, name)
        }
    }

    function verifyPaintedBarIcon(button, name) {
        const icon = findChild(button, "mailBarIcon")
        verify(icon && icon.visible, name + " has no visible icon item")
        verify(waitForRendering(icon), name + " icon did not render")
        const image = grabImage(icon)
        let inkPixels = 0
        for (let y = 0; y < image.height; ++y)
            for (let x = 0; x < image.width; ++x)
                if (image.alpha(x, y) > 128 && image.red(x, y) < 128 &&
                    image.green(x, y) < 128 && image.blue(x, y) < 128) ++inkPixels
        verify(inkPixels >= 12, name + " declares " + button.icon.name + " but paints no ink")
    }

    function test_compactHeaderPaintAndDisjointTargets_data() {
        return [
            {tag: "phone-412", viewWidth: 412, embedded: false},
            {tag: "phone-480", viewWidth: 480, embedded: false},
            {tag: "embedded-412", viewWidth: 412, embedded: true},
            {tag: "embedded-480", viewWidth: 480, embedded: true}
        ]
    }

    function test_compactHeaderPaintAndDisjointTargets(row) {
        const subject = "Your meeting recap - MapLibre Immersive 3D Maps Workgroup Kickoff"
        testCase.width = row.viewWidth
        testCase.height = 915
        storeStub.cached = {"header": {html: "<p>Original <b>formatted</b> message</p>",
            source_url: "https://mail.synthetic.test/header"}}
        let closed = 0
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "header", cardTitle: subject,
            card: {source: "mail", sender_name: "Sender", sender_address: "sender@example.test",
                received_at: "2026-10-25T12:00:00Z"},
            width: row.viewWidth, height: 915, embeddedMode: row.embedded,
            openLinks: false, requestClose: function() { ++closed }
        })
        verify(page)
        verify(waitForPolish(page))
        const bar = findChild(page, "mailAppBar")
        verify(bar.height >= 48 && bar.height < 64, "phone header must stay one slim row")
        let right = 0
        for (const name of ["backToInbox", "archiveMail", "openInGmail", "mailActions"]) {
            const button = findChild(page, name)
            verify(button && button.visible, name + " is not reachable")
            const box = button.mapToItem(bar, 0, 0)
            verify(button.width >= 48 && button.height >= 48, name + " is smaller than 48dp")
            verify(box.x >= right && box.y >= 0 && box.x + button.width <= bar.width &&
                box.y + button.height <= bar.height, name + " overlaps or leaves the app bar")
            right = box.x + button.width
            if (name !== "mailActions") verifyPaintedBarIcon(button, name)
        }
        const title = findChild(page, "mailBarTitle")
        verify(title.width > 0 && title.truncated, "long toolbar title must elide, not steal action targets")
        verify(page.headerHtml.indexOf(subject) >= 0, "full subject must remain in the scrolling document")
        const shot = grabImage(page)
        shot.save(mailFixtures.screenshotPath("reader-header-" + row.tag + ".png"))
        mouseClick(findChild(page, "backToInbox"))
        compare(closed, 1, "the painted Back target must receive the pointer")
    }

    function test_cachedReadIsImmediate_data() {
        return [{tag: "online", online: true}, {tag: "offline", online: false}]
    }

    function test_cachedReadIsImmediate(row) {
        storeStub.online = row.online
        storeStub.cached = {"card-8": {html: "<p>Keep reading while offline.</p>"}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "card-8", openLinks: false
        })
        verify(page)
        compare(page.loading, false)
        compare(page.html, "<p>Keep reading while offline.</p>")
        compare(page.errorText, "")
        compare(storeStub.bodyRequests, [])
        compare(storeStub.operations, [], "reading must never archive or enqueue an operation")
        storeStub.mailBodyFailed("card-8")
        compare(page.loading, false)
        compare(page.html, "<p>Keep reading while offline.</p>")
        compare(page.errorText, "")
        storeStub.cached = {"card-8": {html: "<p>Later body update.</p>"}}
        storeStub.mailBodyChanged("card-8")
        compare(page.html, "<p>Later body update.</p>")
        compare(page.loading, false)
        compare(storeStub.bodyRequests, [])
        compare(storeStub.operations, [])
    }

    function test_missingReadRequestsOnlyOnline_data() {
        return [{tag: "online", online: true}, {tag: "offline", online: false}]
    }

    function test_missingReadRequestsOnlyOnline(row) {
        storeStub.online = row.online
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "missing", openLinks: false
        })
        verify(page)
        compare(page.hasDocument, false)
        compare(page.loading, row.online)
        verify(page.errorText.length > 0, "missing offline content must not look cached")
        compare(storeStub.bodyRequests, row.online ? ["missing"] : [])
        compare(storeStub.operations, [])
        if (!row.online) return
        // Delivery is deferred through the store signal, not a direct reload.
        Qt.callLater(function() {
            storeStub.cached = {"missing": {html: "<p>Newly cached body.</p>"}}
            storeStub.mailBodyChanged("missing")
        })
        tryCompare(page, "loading", false)
        compare(page.html, "<p>Newly cached body.</p>")
        compare(page.errorText, "")
        compare(storeStub.bodyRequests, ["missing"], "body arrival must not request again")
        compare(storeStub.operations, [], "fetching a body must never archive or enqueue an operation")
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
    function test_scmPlainTextThreadFallsBackToCombinedBody() {
        // Live SCM shape (card 8415c61a): body.html carries the <pre> plain-text
        // fallback while the single thread message stores html:"". The reader
        // must show the combined body, not a blank document.
        const scmBody = "<pre>This is an automated email from the git hooks/post-receive script. It was\n"
            + "generated because a ref change was pushed to the repository containing\n"
            + "the project \"PostGIS\".\n\nThe branch, master has been updated</pre>"
        storeStub.cached = {"scm-card": {html: scmBody, source_url: "https://mail.google.com/mail/?authuser=reader%40example.test#all/thread",
            messages: [{id: "msg-1", gmail_message_id: "gmail-1", sender_name: "git", sender_address: "git@osgeo.org",
                received_at: "2026-10-21T03:06:03Z", html: ""}]}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "scm-card", cardTitle: "[SCM] PostGIS branch master updated", openLinks: false,
            card: {source: "mail", title: "[SCM] PostGIS branch master updated"}, width: 598, height: 879
        })
        verify(page)
        verify(page.html.indexOf("automated email from the git hooks") >= 0,
               "SCM plain-text thread shows blank: " + page.html.slice(0, 120))
        verify(page.hasDocument, "SCM plain-text thread reports no document")
        compare(page.errorText, "")
        // Owned-Xvfb proof: the real reader paints the SCM body (not blank).
        const scmView = findChild(page, "body")
        verify(scmView && scmView.runJavaScript !== undefined, "reader body surface missing")
        tryVerify(function() { return !scmView.loading && scmView.loadProgress === 100 }, 15000)
        let scmText = null
        scmView.runJavaScript("document.body.innerText", WebEngineScript.ApplicationWorld, function(v) { scmText = v })
        tryVerify(function() { return scmText !== null }, 5000)
        verify(String(scmText).indexOf("automated email from the git hooks") >= 0,
               "rendered SCM reader is blank: " + String(scmText).slice(0, 120))
        const scmShot = grabImage(page)
        scmShot.save(mailFixtures.screenshotPath("scm-plain-text-598.png"))
    }

    function test_threadPrefersLastNonEmptyMessage() {
        // Plain control: a thread whose latest message has HTML keeps it; an
        // empty trailing slot must not shadow the earlier good message.
        storeStub.cached = {"thread-ctl": {html: "<p>Combined fallback</p>", messages: [
            {id: "m-1", gmail_message_id: "g-1", html: "<p>First original</p>"},
            {id: "m-2", gmail_message_id: "g-2", html: ""}]}}
        const page = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "thread-ctl", cardTitle: "Control", openLinks: false,
            card: {source: "mail"}, width: 598, height: 879
        })
        verify(page)
        compare(page.html, "<p>First original</p>")
        storeStub.cached = {"thread-latest": {html: "<p>Combined fallback</p>", messages: [
            {id: "m-1", gmail_message_id: "g-1", html: "<p>First original</p>"},
            {id: "m-2", gmail_message_id: "g-2", html: "<p>Latest original</p>"}]}}
        const latest = createTemporaryObject(pageComponent, testCase, {
            store: storeStub, cardId: "thread-latest", cardTitle: "Control", openLinks: false,
            card: {source: "mail"}, width: 598, height: 879
        })
        verify(latest)
        compare(latest.html, "<p>Latest original</p>")
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

    // Newsletter colors/layout criterion: readable colors while the author's
    // meaningful HTML, images, quotes and data-table horizontal scrolling all
    // survive. The scroll criterion has its own evidence in
    // test_newsletterSingleVerticalViewport below (the next scroll row consumes it).
    function countMailTags(source) {
        function count(pattern) { return (source.match(pattern) || []).length }
        return {
            a: count(/<a[\s>]/gi), img: count(/<img[\s>]/gi), table: count(/<table[\s>]/gi),
            pre: count(/<pre[\s>]/gi), quote: count(/<(?:q|blockquote)[\s>]/gi)
        }
    }

    // DOM oracle over the rendered document: link/text contrast against the
    // painted backgrounds, dark slabs, image decode, table scrolling, tag
    // census and whether the stylesheet conditions are pinned to light-host
    // semantics (so a dark-preferring host renders the same readable theme).
    function newsletterReport(body) {
        let out = null
        body.runJavaScript(`(function() {
            function chan(v) { return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4) }
            function parse(c) {
                const m = /rgba?\\(([^)]+)\\)/.exec(c || '')
                if (!m) return null
                const p = m[1].split(',').map(s => parseFloat(s))
                return {r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1}
            }
            function lum(c) { return c ? 0.2126 * chan(c.r / 255) + 0.7152 * chan(c.g / 255) + 0.0722 * chan(c.b / 255) : null }
            function effBg(el) {
                let e = el
                while (e) {
                    const c = parse(getComputedStyle(e).backgroundColor)
                    if (c && c.a > 0.05) return c
                    e = e.parentElement
                }
                return {r: 255, g: 255, b: 255, a: 1}
            }
            function contrast(el) {
                const fg = parse(getComputedStyle(el).color)
                const bg = effBg(el)
                if (!fg) return null
                const l1 = lum(fg), l2 = lum(bg)
                return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05)
            }
            function visible(el) {
                const r = el.getBoundingClientRect()
                const cs = getComputedStyle(el)
                return r.width > 0 && r.height > 0 && cs.visibility !== 'hidden' && cs.display !== 'none'
            }
            const links = Array.from(document.querySelectorAll('a')).filter(visible)
            const linkContrasts = links.map(contrast).filter(c => c !== null)
            const textEls = Array.from(document.querySelectorAll('h1,h2,h3,h4,h5,h6,p,li,td,th,span,div,pre,q,blockquote,summary,a'))
                .filter(e => visible(e) && e.innerText.trim() && e.children.length === 0)
            const pale = textEls.map(e => ({tag: e.tagName, text: e.innerText.trim().slice(0, 24), contrast: contrast(e)}))
                .filter(x => x.contrast !== null && x.contrast < 3)
            const slabs = Array.from(document.querySelectorAll('*')).filter(e => {
                if (!visible(e)) return false
                const r = e.getBoundingClientRect()
                return r.height > 4 && lum(effBg(e)) < 0.12
            }).map(e => ({tag: e.tagName, cls: (e.className || '').toString().slice(0, 24),
                text: e.innerText.trim().slice(0, 24)}))
            const doc = document.documentElement
            return JSON.stringify({
                minLinkContrast: linkContrasts.length ? Math.min.apply(null, linkContrasts) : null,
                linkCount: links.length,
                paleText: pale.slice(0, 10),
                darkSlabs: slabs.slice(0, 10),
                images: Array.from(document.images).map(i => ({nw: i.naturalWidth, w: Math.round(i.getBoundingClientRect().width)})),
                tables: Array.from(document.querySelectorAll('table')).map(t => ({
                    display: getComputedStyle(t).display, ox: getComputedStyle(t).overflowX,
                    oy: getComputedStyle(t).overflowY, sw: t.scrollWidth, cw: t.clientWidth})),
                tags: {
                    a: document.querySelectorAll('a').length,
                    img: document.querySelectorAll('img').length,
                    table: document.querySelectorAll('table').length,
                    pre: document.querySelectorAll('pre').length,
                    quote: document.querySelectorAll('q,blockquote').length
                },
                nestedVertical: Array.from(document.querySelectorAll('*')).filter(e =>
                    e.scrollHeight > e.clientHeight + 2 && getComputedStyle(e).overflowY !== 'visible').length,
                doc: {sw: doc.scrollWidth, cw: doc.clientWidth, sh: doc.scrollHeight, ch: doc.clientHeight},
                scrollbar: getComputedStyle(doc).scrollbarWidth,
                colorScheme: getComputedStyle(doc).colorScheme,
                pinned: Array.from(document.querySelectorAll('style'))
                    .every(s => (s.textContent || '').indexOf('prefers-color-scheme') < 0)
            })
        })()`, WebEngineScript.ApplicationWorld, function(value) { out = value })
        tryVerify(function() { return out !== null })
        return JSON.parse(out)
    }

    function test_newsletterColorLayout_data() {
        const rows = []
        for (const fixture of ["newsletter-lwn", "newsletter-claude"])
            for (const viewWidth of [598, 360])
                rows.push({tag: fixture + "-" + viewWidth, fixture: fixture, viewWidth: viewWidth,
                           strictLight: fixture === "newsletter-lwn"})
        rows.push({tag: "plain-control-598", fixture: "plain", viewWidth: 598, strictLight: false, control: true})
        return rows
    }

    function test_newsletterColorLayout(row) {
        const source = mailFixtures.read(row.fixture)
        const shown = openMessage(source, row.viewWidth)
        const report = newsletterReport(shown.body)
        console.log("NEWSLETTER_COLOR_LAYOUT", row.tag, JSON.stringify({
            minLinkContrast: report.minLinkContrast, links: report.linkCount, pale: report.paleText.length,
            darkSlabs: report.darkSlabs.length, images: report.images.length, tags: report.tags,
            pinned: report.pinned, colorScheme: report.colorScheme, doc: report.doc}))
        const shot = grabImage(shown.page)
        shot.save(mailFixtures.screenshotPath("newsletter-" + row.tag + ".png"))
        // Readable colors: links and every visible text run clear 3:1 against
        // what is actually painted behind them.
        if (report.linkCount > 0)
            verify(report.minLinkContrast >= 3,
                   row.tag + " paints an unreadable link at " + report.minLinkContrast + ":1")
        compare(report.paleText, [], row.tag + " paints unreadable text")
        // The reader canvas is light, so a dark-palette slab standing on it is
        // the reported defect. An author band may stay dark only where its own
        // text stays readable, which the paleText oracle above covers.
        if (row.strictLight)
            compare(report.darkSlabs, [], row.tag + " paints dark slabs on the light canvas")
        // Meaningful HTML survives: the same links, images, tables, pre blocks
        // and quotes render as the message itself defines them.
        const expected = countMailTags(source)
        for (const key of ["a", "img", "table", "pre", "quote"])
            compare(report.tags[key], expected[key], row.tag + " lost " + key + " markup")
        for (const image of report.images)
            verify(image.nw > 0 && image.w > 0, row.tag + " dropped an image")
        // Actual data tables keep their column relationships and scroll locally.
        for (const table of report.tables)
            if (table.sw > table.cw + 1)
                compare(table.ox, "auto", row.tag + " data table lost horizontal scrolling")
        verify(report.doc.sw <= report.doc.cw + 1, row.tag + " document overflows its column")
        // A dark-preferring host must evaluate the message's own stylesheet on
        // light-host semantics and render the same readable theme.
        verify(report.pinned, row.tag + " stylesheet conditions are not pinned to light-host semantics")
        verify(String(report.colorScheme).indexOf("only") >= 0, row.tag + " color scheme is not light-only")
    }

    // Scroll criterion, evidenced separately from the colors/layout row above:
    // one vertical viewport — the document's — with its scrollbar left visible.
    // The only nested scroll surface is the data table's horizontal port that
    // the layout criterion keeps; the next scroll row consumes this proof.
    function test_newsletterSingleVerticalViewport_data() {
        return [{tag: "newsletter-lwn-598", fixture: "newsletter-lwn", wide: false},
                {tag: "newsletter-claude-598", fixture: "newsletter-claude", wide: true}]
    }

    function test_newsletterSingleVerticalViewport(row) {
        const shown = openMessage(mailFixtures.read(row.fixture), 598)
        const report = newsletterReport(shown.body)
        const nestedHorizontal = report.tables.filter(function(t) { return t.sw > t.cw + 1 })
        console.log("NEWSLETTER_SCROLL_IDENT", row.tag, JSON.stringify({
            doc: report.doc, nestedVertical: report.nestedVertical,
            nestedHorizontalTables: nestedHorizontal.length, scrollbar: report.scrollbar,
            tables: report.tables}))
        compare(report.nestedVertical, 0, row.tag + " has a nested vertical scrollport")
        // Computed overflow-y is the vertical-scroller oracle: a message table
        // may never compute a user-scrollable vertical axis (overflow-x:auto
        // alone computes overflow-y to auto, which nests a vertical scroller).
        for (const t of report.tables)
            verify(t.oy !== "auto" && t.oy !== "scroll",
                   row.tag + " table computes overflow-y:" + t.oy + " (nested vertical scroller)")
        // The wide data table keeps its horizontal port and really scrolls on
        // that axis only.
        if (row.wide) {
            verify(nestedHorizontal.length > 0, row.tag + " lost the wide data table's horizontal port")
            let hscroll = null
            shown.body.runJavaScript(`(function() {
                return Array.from(document.querySelectorAll('table'))
                    .filter(e => e.scrollWidth > e.clientWidth + 1)
                    .map(e => { const was = e.scrollLeft; e.scrollLeft = e.scrollWidth;
                        const moved = e.scrollLeft; e.scrollLeft = was;
                        return {moved: moved, sw: e.scrollWidth, cw: e.clientWidth,
                                ox: getComputedStyle(e).overflowX, oy: getComputedStyle(e).overflowY} })
            })()`, WebEngineScript.ApplicationWorld, function(v) { hscroll = v })
            tryVerify(function() { return hscroll !== null })
            verify(hscroll.length > 0, row.tag + " wide data table is not identified")
            for (const t of hscroll) {
                verify(t.moved > 0, row.tag + " wide data table no longer scrolls horizontally")
                console.log("NEWSLETTER_TABLE_SCROLL", row.tag, JSON.stringify(t))
            }
        }
        const shot = grabImage(shown.page)
        shot.save(mailFixtures.screenshotPath("scroll-" + row.tag + ".png"))
        verify(report.doc.sh > report.doc.ch, row.tag + " message is too short to prove scrolling")
        verify(report.scrollbar !== "none", row.tag + " hides the document scrollbar")
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
                  "A wheel event must scroll the open newsletter (" + row.tag + ")")
        console.log("NEWSLETTER_WHEEL", row.tag, 0, "->", position)
        mouseWheel(shown.body, shown.body.width / 2, shown.body.height / 2, 0, 120, Qt.NoButton)
        tryVerify(function() { readPosition(); return position === 0 }, 5000,
                  "Reverse wheel must return to the newsletter start")
    }
}
