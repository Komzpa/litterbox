CREATE TABLE journal_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX journal_entries_tenant_created_idx ON journal_entries (tenant_id, created_at DESC);
ALTER TABLE journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY journal_entries_tenant_policy ON journal_entries
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

CREATE TABLE mcp_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    scopes text[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
CREATE INDEX mcp_tokens_tenant_idx ON mcp_tokens (tenant_id);
