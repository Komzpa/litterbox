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

-- The server-side sync worker must discover connected accounts without a
-- request tenant. Ordinary app queries remain subject to tenant RLS.
CREATE FUNCTION litterbox_gmail_sync_accounts()
RETURNS TABLE (tenant_id uuid, id uuid, refresh_token bytea, history_id text)
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
    SELECT a.tenant_id, a.id, a.refresh_token, COALESCE(a.history_id, '')
    FROM public.accounts AS a WHERE a.status = 'active'
    ORDER BY a.tenant_id, a.id;
$$;
REVOKE ALL ON FUNCTION litterbox_gmail_sync_accounts() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION litterbox_gmail_sync_accounts() TO litterbox_app;
