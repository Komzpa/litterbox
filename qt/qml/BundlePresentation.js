.pragma library

function title(card, summary) {
    card = card || {}
    summary = summary || {}

    const bundleTitle = card.bundle_title || ""
    if (!bundleTitle.startsWith("sender:"))
        return bundleTitle || qsTr("Bundle")

    const latestTitle = summary.latestTitle || ""
    if (latestTitle.trim().length > 0)
        return latestTitle

    const sender = card.sender_name || card.sender_address || bundleTitle.slice(7)
    const cleanSender = sender.replace(/[<>]/g, "").trim()
    const at = cleanSender.lastIndexOf("@")
    const domain = at >= 0 ? cleanSender.slice(at + 1) : ""
    return domain || cleanSender || qsTr("Bundle")
}
