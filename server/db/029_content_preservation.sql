-- Preserve accepted deliverables through partial producer upserts.
ALTER TABLE card_bodies ADD COLUMN source_files jsonb;
ALTER TABLE card_bodies ADD COLUMN legacy_html text NOT NULL DEFAULT '';
ALTER TABLE card_bodies ADD COLUMN source_body text NOT NULL DEFAULT '';
