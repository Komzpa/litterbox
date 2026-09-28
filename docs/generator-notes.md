# Generator card notes

Task generators should fetch owner-provided card context from `GET /v1/cards/notes?since=<RFC3339 timestamp>`. The endpoint is tenant-scoped and returns `{"notes":[{"card_id":"…","title":"…","note":"…","updated_at":"…"}]}`. Omit `since` for the initial fetch; save the latest `updated_at` and pass it on subsequent polls. Notes are per-card instruction/context; do not apply one card’s note to another card. Empty notes are excluded, and updating a note replaces that card’s current context.
