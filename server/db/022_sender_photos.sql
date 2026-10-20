-- R67 real Google sender photos: tenant/account-scoped photo bytes plus
-- the incremental-consent gate. Photos come only from the documented People
-- API contact surfaces (connections.list + otherContacts.list); Gmail.modify
-- alone carries no sender-photo field.
CREATE TABLE IF NOT EXISTS sender_photos (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    account_id uuid NOT NULL,
    sender_address text NOT NULL,
    photo bytea NOT NULL,
    content_type text NOT NULL DEFAULT 'image/jpeg',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, account_id, sender_address),
    FOREIGN KEY (tenant_id, account_id)
        REFERENCES accounts (tenant_id, id) ON DELETE CASCADE,
    CHECK (char_length(sender_address) > 0 AND char_length(sender_address) <= 320),
    CHECK (octet_length(photo) > 0 AND octet_length(photo) <= 524288)
);
CREATE INDEX IF NOT EXISTS sender_photos_account_idx
    ON sender_photos (tenant_id, account_id);
-- Cards carry a normalized addr-spec for the sender-photo join so the list
-- query needs no fragile SQL string surgery on the raw From header.
ALTER TABLE cards ADD COLUMN IF NOT EXISTS sender_address text NOT NULL DEFAULT '';
-- Granted OAuth scopes per account (space-separated, as returned by Google).
-- Empty means Gmail-only: sender photos stay in authorization_required
-- until the account completes the incremental photo consent.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS granted_photo_scopes text NOT NULL DEFAULT '';
