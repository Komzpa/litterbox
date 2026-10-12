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

function additionalCards() {
    return [
        {id: "postgis6144", title: "[PostGIS] #6144: ST_CreateOverview function errors", sender_name: "PostGIS", snippet: "#6144: ST_CreateOverview function errors\r\n-------------------------+---------------------------\r\n Reporter:  amorevosgeo\tOwner:  pramsey\r\n     Type:  defect\tStatus:  new\r\n Priority"},
        {id: "postgis6143", title: "Re: [PostGIS] #6143: [topology] False edge crossing detected", sender_name: "Darafei \"Komяpa\" Praliaskouski", snippet: "⁰ж\r\n\r\n"},
        {id: "miles", title: "A New Login to Your Miles&Smiles Account", sender_name: "Turkish Airlines", snippet: "", bundle_id: "miles", bundle_title: "A New Login to Your Miles&Smiles Account", bundle_leader: true, bundle_member_count: 2},
        {id: "domains", title: "Upcoming Domain Expirations - 30-day reminder", sender_name: "support via Hosting", snippet: "Hello Arben,\r\n\r\n", bundle_id: "domains", bundle_title: "Upcoming Domain Expirations - 30-day reminder", bundle_leader: true, bundle_member_count: 4},
        {id: "sleep", title: "Welcome to SleepHQ", sender_name: "Uncle Nicko & Adsy", snippet: "Your Sleep Journey Starts Here! Let's start tracking\r\n\r\n"},
        {id: "confirmation", title: "Confirmation instructions", sender_name: "SleepHQ", snippet: "Confirm your email address͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏\n͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏ ͏\n͏ ͏ ͏ ͏ ͏ ͏ ͏ ‌\n"},
        {id: "maproulette", title: "MapRoulette security incident notice", sender_name: "MapRoulette", snippet: "Hi Komяpa,\r\n\r\n"},
        {id: "hosting", title: "New device authorization", sender_name: "support via Hosting", snippet: "Hi Arben Kane,\r\n\r\n", account_name: "darafei@maumap.com"}
    ].map(function(card) { return Object.assign({source: "mail", section: "now", account_name: "komzpa@gmail.com", received_at: "2026-10-09T09:39:00Z", summary: "", note: "", bundle_id: "", pinned_rank: null, important: false, timed: false, has_body: true}, card) })
}
