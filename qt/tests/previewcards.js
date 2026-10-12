.pragma library

// Mail text read from the screenshot cards, production read-only, 2026-10-12.
function cards() {
    return [
        {id: "flying", title: "Выиграйте аренду автомобиля в Hertz", sender_name: "Flying Blue", snippet: "Flying Blue\r\n\r\n"},
        {id: "odm", title: "OpenDroneMap previews mesh quality leap via OpenMVS", sender_name: "LinkedIn", snippet: "", bundle_id: "odm", bundle_title: "OpenDroneMap previews mesh quality leap via OpenMVS", bundle_leader: true, bundle_member_count: 3},
        {id: "salt", title: "Финансовые новости от компании Salt BY", sender_name: "Salt BY", snippet: "", bundle_id: "salt", bundle_title: "Финансовые новости от компании Salt BY", bundle_leader: true, bundle_member_count: 9},
        {id: "jasmine", title: "I want to connect", sender_name: "Jasmine Wu.", snippet: "Jasmine, 销售经理 from Guangzhou Runfu Biotechnology Co., Ltd. is waiting for your response\r\n\r\n"},
        {id: "qgis", title: "[QGIS-Developer] QEP 429: Add exception to the AI policy for trusted contributors", sender_name: "Martin Dobias via QGIS-Developer", snippet: "Hi list\r\n\r\n"},
        {id: "kyivstar", title: "🔥 Топ-10 актуальних вакансій тут 📩", sender_name: "Київстар", snippet: "☁️ А раптом цей знак? :) І ще акції, бонуси, подарунки та інші чудові новини"},
        {id: "polis", title: "Ваш страховий полис готов - Polis.ge", sender_name: "Polis.ge Online Insurance", snippet: ""}
    ].map(function(card) { return Object.assign({source: "mail", section: "now", account_name: "komzpa@gmail.com", received_at: "2026-10-09T14:33:00Z", summary: "", note: "", bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: true}, card) })
}
