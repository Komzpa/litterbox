.pragma library

// Immutable sanitized trace/API projection from techship-preview-20261014.
// Only presentation roles (section/leader/count) are added for a disposable inbox.
function cards() {
    const records = [
        {id: "e2976b11-cba2-42d1-8bc9-3477615cd2d0", title: "Techship Newsletter October 14, 2026", received_at: "2026-10-14T15:03:26+04:00", snippet: "[Techship logo in orange and burgundy color <https://439e97966357f0118ee5000d3ab41d47.103.eu.prod.marketingusercontent.com/api/orgs/439e9796-6357-f011-8ee5-000d3ab41d47/r/fWbCYmszA0WsoyNsBwwAABAAAAA?[REDACTED_QUERY]>]"},
        {id: "e537dde9-caf0-4565-81b9-801adcee8418", title: "Techship Newsletter October 7, 2026", received_at: "2026-10-07T11:01:04Z", snippet: "[Techship logo in orange and burgundy color <https://439e97966357f0118ee5000d3ab41d47.103.eu.prod.marketingusercontent.com/api/orgs/439e9796-6357-f011-8ee5-000d3ab41d47/r/PDhQfMEwcUWuXBORWlkBABUAAAA?[REDACTED_QUERY]>]"},
        {id: "30123be7-9327-4733-89a6-df69dc4569d4", title: "Techship Newsletter September 23, 2026", received_at: "2026-09-23T10:05:08Z", snippet: "[Techship logo in orange and burgundy color <https://439e97966357f0118ee5000d3ab41d47.103.eu.prod.marketingusercontent.com/api/orgs/439e9796-6357-f011-8ee5-000d3ab41d47/r/8e7WlojDpUmqbZZmBD4AAA8AAAA?[REDACTED_QUERY]>]"}
    ];
    return records.map(function(record, index) {
        return Object.assign({source: "mail", section: "now", state: "open", summary: "", note: "", pinned_rank: null, important: false, timed: false, has_body: true,
            bundle_id: "7b4c0b41-2b58-4fbd-a37a-72ed2831f5c8", bundle_title: "Techship Newsletter September 23, 2026", bundle_leader: index === 0, bundle_member_count: 3,
            account_name: "komzpa@gmail.com", sender_name: "Techship Newsletter", sender_address: "newsletter@news.techship.com"}, record);
    });
}
