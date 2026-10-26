.pragma library

// Version 1 synthetic newsletter projection. No owner records, tokens or URLs.
// Fixed identifiers/dates and distinct prose make this a permanent release oracle.
var version = 1
function cards() {
    const records = [
        {id: "newsletter-v1-1", title: "Module field notes: pricing", received_at: "2020-03-03T12:00:00Z", snippet: "Compare module prices before choosing the next evaluation board.", body: "Pricing article. Retain the complete comparison and its original punctuation."},
        {id: "newsletter-v1-2", title: "Module field notes: throughput", received_at: "2020-03-02T12:00:00Z", snippet: "Measure sustained throughput with the antenna mounted in its enclosure.", body: "Throughput article. Retain the full measurement procedure, not only its preview."},
        {id: "newsletter-v1-3", title: "Module field notes: standalone networks", received_at: "2020-03-01T12:00:00Z", snippet: "Standalone networks use a dedicated core rather than the legacy anchor.", body: "Network article. Retain both standalone and anchored deployment explanations."}
    ];
    return records.map(function(record, index) {
        return Object.assign({source: "mail", section: "now", state: "open", summary: "", note: "", pinned_rank: null, important: false, timed: false, has_body: true,
            bundle_id: "newsletter-bundle-v1", bundle_title: "Module field notes", bundle_leader: index === 0, bundle_member_count: 3,
            account_name: "reader@example.test", sender_name: "Module newsletter", sender_address: "newsletter@example.test"}, record);
    });
}
