-- R67 account-bound photo consent: the incremental photo-consent state is
-- bound to one intended account so the shared OAuth callback can verify
-- that the Google identity it receives is that account before storing
-- anything. beginPhotoConsent inserts flow='photo' plus the intended
-- account_id; beginConnect inserts neither, so both default to the plain
-- connect flow (flow='connect', account_id NULL). The callback consumes
-- both columns in one DELETE ... RETURNING. Without these columns every
-- OAuth callback fails closed (400) and photo consent cannot start (500).
-- Additive only: applies cleanly to fresh installs (after 006 in lexical
-- order) and to databases already migrated through 023.
ALTER TABLE gmail_oauth_states
    ADD COLUMN IF NOT EXISTS flow text NOT NULL DEFAULT 'connect',
    ADD COLUMN IF NOT EXISTS account_id uuid;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'gmail_oauth_states_account_fk'
    ) THEN
        ALTER TABLE gmail_oauth_states
            ADD CONSTRAINT gmail_oauth_states_account_fk
            FOREIGN KEY (tenant_id, account_id)
            REFERENCES accounts (tenant_id, id) ON DELETE CASCADE;
    END IF;
END
$$;
