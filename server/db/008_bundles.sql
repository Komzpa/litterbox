-- Store deterministic sender/domain/list-id bundle identity independently of
-- message importance. Existing bundles retain their user-visible UUIDs.
ALTER TABLE bundles ADD COLUMN IF NOT EXISTS bundle_key text;
CREATE UNIQUE INDEX IF NOT EXISTS bundles_tenant_key_idx
    ON bundles (tenant_id, bundle_key) WHERE bundle_key IS NOT NULL;
ALTER TABLE cards ADD COLUMN IF NOT EXISTS importance text NOT NULL DEFAULT 'normal';

CREATE TABLE IF NOT EXISTS bundle_exclusions (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sender_key text NOT NULL,
    bundle_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, sender_key, bundle_key)
);
ALTER TABLE bundle_exclusions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bundle_exclusions
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON bundle_exclusions TO litterbox_app;
