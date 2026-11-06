-- A failed raw-message fetch must not block other messages in the same card.
ALTER TABLE messages ADD COLUMN content_repair_attempted_at timestamptz;
-- A provider 404 is terminal for repair, while readable local content stays.
-- Ordinary thread synchronization still owns removal/reappearance of messages.
ALTER TABLE messages ADD COLUMN content_provider_deleted boolean NOT NULL DEFAULT false;
