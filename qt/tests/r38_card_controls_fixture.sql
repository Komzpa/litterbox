-- R38 manual card creation and order fixture:
--   litterbox-qt --capture-card-controls <fixture.sqlite> <empty-output-directory>
-- Two pinned manual cards, two untimed manual cards, one time-anchored card,
-- and an empty outbox. Regenerate with:
--   sqlite3 r38-card-controls.sqlite < r38_card_controls_fixture.sql
CREATE TABLE cards (id TEXT PRIMARY KEY, position INTEGER NOT NULL, payload TEXT NOT NULL);
CREATE TABLE outbox (seq INTEGER PRIMARY KEY AUTOINCREMENT, op_id TEXT UNIQUE NOT NULL, payload TEXT NOT NULL);

INSERT INTO cards(id, position, payload) VALUES
 ('11111111-1111-4111-8111-111111111111', 0, '{"id":"11111111-1111-4111-8111-111111111111","title":"Pinned one","summary":"Fixture","source":"manual","state":"open","section":"now","pinned_rank":1}'),
 ('22222222-2222-4222-8222-222222222222', 1, '{"id":"22222222-2222-4222-8222-222222222222","title":"Pinned two","summary":"Fixture","source":"manual","state":"open","section":"now","pinned_rank":2}'),
 ('33333333-3333-4333-8333-333333333333', 2, '{"id":"33333333-3333-4333-8333-333333333333","title":"Manual A","summary":"Fixture","source":"manual","state":"open","section":"now"}'),
 ('44444444-4444-4444-8444-444444444444', 3, '{"id":"44444444-4444-4444-8444-444444444444","title":"Manual B","summary":"Fixture","source":"manual","state":"open","section":"now"}'),
 ('55555555-5555-4555-8555-555555555555', 4, '{"id":"55555555-5555-4555-8555-555555555555","title":"Later timed","summary":"Fixture","source":"todo","state":"open","section":"later","timed":true,"at":"2031-01-01T09:00:00Z"}');
