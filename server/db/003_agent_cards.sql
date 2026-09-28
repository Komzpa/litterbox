-- Shared card metadata for non-mail sources. Existing mail cards retain source=mail.
ALTER TABLE cards
    ALTER COLUMN account_id DROP NOT NULL,
    ALTER COLUMN gmail_thread_id DROP NOT NULL,
    ADD COLUMN source text NOT NULL DEFAULT 'mail',
    ADD COLUMN external_id text,
    ADD COLUMN title text NOT NULL DEFAULT '',
    ADD COLUMN summary text NOT NULL DEFAULT '';

ALTER TABLE cards ADD CONSTRAINT cards_source_identity_check CHECK (
    (source = 'mail' AND account_id IS NOT NULL AND gmail_thread_id IS NOT NULL AND external_id IS NULL)
    OR (source <> 'mail' AND account_id IS NULL AND gmail_thread_id IS NULL AND external_id IS NOT NULL)
);
CREATE UNIQUE INDEX cards_source_external_id_idx ON cards (tenant_id, source, external_id);
