ALTER TABLE cards ADD COLUMN note_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE journal_entries ADD COLUMN revision bigint NOT NULL DEFAULT 0;
