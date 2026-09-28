-- Gmail history polling cursor is per connected account; retain the last
-- successful sync time for operational visibility without resetting history.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS gmail_synced_at timestamptz;
CREATE INDEX IF NOT EXISTS cards_account_open_idx
    ON cards (tenant_id, account_id, state, sort_at DESC);

CREATE TABLE gmail_oauth_states (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    state_hash bytea NOT NULL,
    code_verifier text NOT NULL,
    redirect_uri text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, state_hash)
);
ALTER TABLE gmail_oauth_states ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON gmail_oauth_states
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON gmail_oauth_states TO litterbox_app;
