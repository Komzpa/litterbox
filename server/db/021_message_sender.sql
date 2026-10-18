-- Preserve each original Gmail message's parsed From fields for thread reading.
ALTER TABLE messages ADD COLUMN sender_name text NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN sender_address text NOT NULL DEFAULT '';
