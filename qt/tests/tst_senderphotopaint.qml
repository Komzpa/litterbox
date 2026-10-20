import QtQuick
import QtTest
import litterbox 1.0 as App
import BundleTest 1.0

// R67 sender-photo consumer regression (CF2: pixel-counted paint claims).
// One scoped run collects every photo consumer state: first ONLINE paint
// after cache-download completion WITHOUT a manual refresh(), the offline
// device-cache paint after reopen, and the default/missing/auth-state
// initials fallbacks. The API leg mirrors main.cpp exactly:
// requestSenderPhotoGet -> applyRemoteSenderPhoto (2xx bytes) or
// reportSenderPhotoFailed (404 unavailable / 403 authorization_required).
TestCase {
    id: testCase
    name: "SenderPhotoPaint"
    when: windowShown

    property var store: bundleStore
    readonly property string grantedId: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
    readonly property string defaultId: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
    readonly property string missingId: "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
    readonly property string absentId: "dddddddd-dddd-4ddd-8ddd-dddddddddddd"

    QtObject {
        id: api
        property string baseUrl: ""
        property string token: ""
        property var accounts: []
        function get(path, callback) { callback(null, {body: accounts}) }
    }
    QtObject {
        id: updater
        property bool supported: false
        property bool busy: false
        signal noUpdateAvailable()
        signal installConsentRequired()
        signal errorOccurred(string message)
        signal updateReady(string version, string sha256)
    }
    QtObject { id: timeRules; function display(value) { return value } }
    Component { id: inboxComponent; App.InboxView {} }
    Component { id: storeComponent; CardStore {} }

    // Model/role notification probes: the DelegateModel cached role values are
    // the exact bridge production rebuildPresentation consumes (items.get(i).
    // model). A stale card map here means the cache-download completion never
    // reached the model/role layer.
    DelegateModel { id: probeModel; model: store }
    function probePhoto(cardId) {
        for (let i = 0; i < probeModel.items.count; ++i) {
            const roles = probeModel.items.get(i).model
            if (roles && roles.cardId === cardId) return roles.sender_photo || ""
        }
        return ""
    }

    SignalSpy { id: photoChangedSpy; target: store; signalName: "senderPhotoChanged" }
    SignalSpy { id: photoFailedSpy; target: store; signalName: "senderPhotoFailed" }
    SignalSpy { id: dataChangedSpy; target: store; signalName: "dataChanged" }

    property var photoRequests: ({})
    property int cardRequests: 0

    function card(id, extra) {
        return Object.assign({id: id, title: "Mail from " + id, source: "mail", section: "now",
            state: "open", bundle_id: "", bundle_title: "", pinned_rank: null, important: false,
            timed: false, has_body: false, summary: "", note: "", account_name: "test@example.test",
            sender_name: "Sender " + id.slice(0, 1)}, extra || {})
    }

    // Exact main.cpp signal wiring. requestCards is deliberately counted but
    // never answered: any paint claimed below must come from cache-download
    // completion, not from a snapshot or manual refresh().
    function installApiLeg(targetStore) {
        targetStore.requestCards.connect(function(path, requestId) {
            if (targetStore === store) ++cardRequests
        })
        targetStore.requestSenderPhotoGet.connect(function(cardId, path) {
            if (targetStore === store) {
                photoRequests[cardId] = (photoRequests[cardId] || 0) + 1
                photoRequests = photoRequests
            }
            // Binary network completion occurs after the request/initial layout,
            // not synchronously inside setOnline or the first count rebuild.
            Qt.callLater(function() {
                if (cardId === grantedId)
                    targetStore.applyRemoteSenderPhoto(cardId, senderPhotoFixtureBytes, "image/png")
                else if (cardId === missingId)
                    targetStore.reportSenderPhotoFailed(cardId, 403)
                else
                    targetStore.reportSenderPhotoFailed(cardId, 404)
            })
        })
    }

    function rowFor(inbox, cardId) {
        const list = findChild(inbox, "inboxList")
        list.forceLayout()
        for (let i = 0; i < list.count; ++i) {
            const row = list.itemAtIndex(i)
            if (row && row.cardId === cardId) return row
        }
        return null
    }
    function avatarImage(row) { return findChild(findChild(row, "reorderHandle-" + row.cardId), "senderAvatarImage") }
    function initialsOf(row) { return findChild(findChild(row, "reorderHandle-" + row.cardId), "senderAvatarInitials") }

    function verifyPaint(inbox, row) {
        const image = avatarImage(row)
        verify(image && image.visible, "real inbox avatar image is visible")
        tryCompare(image, "status", Image.Ready)
        verify(waitForRendering(image), "Image.Ready must reach a rendered frame before pixel sampling")
        const canvas = inbox.contentItem.parent
        const painted = grabImage(canvas)
        const position = image.mapToItem(canvas, 0, 0)
        const left = Math.floor(position.x + image.width / 4)
        const right = Math.floor(position.x + image.width * 3 / 4)
        const middle = Math.floor(position.y + image.height / 2)
        compare(painted.red(left, middle), 220, "left photo pixels are painted, not initials")
        compare(painted.green(left, middle), 30)
        compare(painted.blue(left, middle), 30)
        compare(painted.red(right, middle), 30)
        compare(painted.green(right, middle), 160, "right photo pixels are painted")
        compare(painted.blue(right, middle), 60)
    }

    function capture(inbox, name) {
        if (!bundleProofDirectory) return
        grabImage(inbox.contentItem.parent).save(bundleProofDirectory + "/" + name + ".png")
    }

    function test_senderPhotoConsumers() {
        // ---- Phase A: cold online inbox; no photo anywhere yet. ----
        verify(store.applyRemoteCards({now: [
            card(grantedId, {sender_has_photo: true}),
            card(defaultId),
            card(missingId, {sender_has_photo: true}),
            card(absentId, {sender_has_photo: true})
        ], later: [], missed: []}))
        installApiLeg(store)
        const inbox = createTemporaryObject(inboxComponent, this, {
            store: store, api: api, updater: updater, timeRules: timeRules, width: 1440, height: 1000
        })
        verify(inbox)
        tryVerify(function() {
            return rowFor(inbox, grantedId) !== null && rowFor(inbox, defaultId) !== null
                && rowFor(inbox, missingId) !== null && rowFor(inbox, absentId) !== null
        }, 10000)
        capture(inbox, "online-00-baseline-initials")

        // Baseline: initials fallback everywhere before any download.
        for (const id of [grantedId, defaultId, missingId, absentId]) {
            const row = rowFor(inbox, id)
            compare(row.senderPhoto, "", "baseline row " + id + " must start on initials")
            verify(!avatarImage(row).visible || avatarImage(row).source.toString() === "",
                   "baseline avatar image hidden/empty for " + id)
            const init = initialsOf(row)
            verify(init && init.visible,
                   "baseline initials visible for " + id + " (found=" + (init !== null)
                   + " visible=" + (init ? init.visible : "?")
                   + " photoSource=" + (init && init.parent ? init.parent.photoSource : "?") + ")")
        }

        // ---- Phase B: go online. The photo lands mid-session, like the
        // staged smoke prefetch; NO refresh() is ever called in this test. ----
        dataChangedSpy.clear()
        photoChangedSpy.clear()
        store.online = true

        // THE regression assertion: first online paint arrives from
        // cache-download completion alone.
        tryVerify(function() {
            const row = rowFor(inbox, grantedId)
            return row && row.senderPhoto.length > 0
        }, 20000, "first online sender photo must paint without a manual refresh()")

        // Model/role notification chain (diagnostic + contract).
        verify(dataChangedSpy.count > 0,
               "cache-download completion must notify the model (dataChanged)")
        verify(photoChangedSpy.count > 0,
               "cache-download completion must emit senderPhotoChanged")
        verify(probePhoto(grantedId).length > 0,
               "DelegateModel's notifying sender_photo role carries the downloaded URL")
        compare(cardRequests, 1, "only setOnline's initial card request; no manual refresh")

        // The real consumer image actually binds and loads the cached bytes.
        const grantedRow = rowFor(inbox, grantedId)
        const image = avatarImage(grantedRow)
        verify(image.visible, "granted avatar image visible without refresh")
        compare(image.status, Image.Ready, "granted avatar image decodes the cached photo")
        verify(image.source.toString().indexOf("sender-photos/" + grantedId) > 0,
               "granted avatar binds the device-local cached file URL")
        verify(!initialsOf(grantedRow).visible, "initials hide once the photo paints")
        verifyPaint(inbox, grantedRow)
        capture(inbox, "online-01-first-paint-no-refresh")

        // ---- Phase C: no-photo flag avoids a request; unavailable and
        // authorization-required responses keep initials, never bad bytes. ----
        tryVerify(function() {
            return (photoRequests[absentId] || 0) >= 1 && (photoRequests[missingId] || 0) >= 1
        }, 20000, "unavailable and authorization-required responses reach the fallback consumer")
        compare(photoRequests[defaultId] || 0, 0, "default-only sender has no advertised photo or request")
        for (const id of [defaultId, missingId, absentId]) {
            const row = rowFor(inbox, id)
            compare(row.senderPhoto, "", "fallback row " + id + " stays on initials")
            verify(initialsOf(row).visible, "fallback initials visible for " + id)
            compare(store.cachedSenderPhoto(id), "", "no cache entry for fallback row " + id)
        }
        capture(inbox, "online-02-default-missing-fallback")

        // ---- Phase D: negative control — malformed bytes never persist or
        // paint (bad-fixture defense stays; the valid fixture paints above). ----
        const failedBefore = photoFailedSpy.count
        store.applyRemoteSenderPhoto(missingId, senderPhotoGarbageBytes, "image/png")
        verify(photoFailedSpy.count > failedBefore, "garbage bytes are rejected")
        compare(store.cachedSenderPhoto(missingId), "", "garbage bytes never reach the device cache")
        compare(rowFor(inbox, missingId).senderPhoto, "", "garbage bytes never paint")

        // ---- Phase E: offline reopen — device cache paints with NO request. ----
        // Persistence is queued on the storage worker; poll a real reopen.
        let reopened = null
        for (let attempt = 0; attempt < 20 && !reopened; ++attempt) {
            wait(250)
            const candidate = storeComponent.createObject(null)
            verify(candidate)
            if (candidate.open(bundleCachePath) && candidate.cachedSenderPhoto(grantedId) !== "")
                reopened = candidate
            else
                candidate.destroy()
        }
        verify(reopened, "photo row must survive reopen in the durable device cache")
        compare(reopened.cachedSenderPhoto(grantedId).toString().indexOf("sender-photos/" + grantedId) > 0,
                true, "reopened cache keeps the device-local file URL")

        const offlineGets = []
        reopened.requestSenderPhotoGet.connect(function(cardId, path) { offlineGets.push(cardId) })
        const offline = createTemporaryObject(inboxComponent, this, {
            store: reopened, api: api, updater: updater, timeRules: timeRules, width: 1440, height: 1000
        })
        verify(offline)
        verify(!reopened.online, "reopen stays offline")
        tryVerify(function() {
            const row = rowFor(offline, grantedId)
            return row && row.senderPhoto.length > 0
        }, 10000, "offline cached photo paints on reopen")
        const offlineRow = rowFor(offline, grantedId)
        const offlineImage = avatarImage(offlineRow)
        verify(offlineImage.visible, "offline avatar image visible")
        compare(offlineImage.status, Image.Ready, "offline avatar decodes the cached photo")
        verifyPaint(offline, offlineRow)
        compare(offlineGets.length, 0, "offline paint needs no request")
        for (const id of [defaultId, missingId, absentId]) {
            const row = rowFor(offline, id)
            compare(row.senderPhoto, "", "offline fallback row " + id + " stays on initials")
            verify(initialsOf(row).visible, "offline fallback initials visible for " + id)
        }
        capture(offline, "offline-01-cached-paint-fallbacks")
    }
}
