-- Cache the exact normalized input sent to each embedding model. A changed
-- subject/body replaces only that card's vector; model switches cannot reuse it.
CREATE TABLE card_embeddings (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    card_id uuid NOT NULL,
    model text NOT NULL,
    input_hash bytea NOT NULL CHECK (octet_length(input_hash) = 32),
    vector double precision[] NOT NULL,
    PRIMARY KEY (tenant_id, card_id, model),
    FOREIGN KEY (tenant_id, card_id) REFERENCES cards (tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE card_embeddings ENABLE ROW LEVEL SECURITY;
ALTER TABLE card_embeddings FORCE ROW LEVEL SECURITY;
CREATE POLICY card_embeddings_tenant_policy ON card_embeddings
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON card_embeddings TO litterbox_app;
