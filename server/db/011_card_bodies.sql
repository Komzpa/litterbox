CREATE TABLE card_bodies (
    tenant_id uuid NOT NULL,
    card_id uuid NOT NULL,
    html text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, card_id),
    FOREIGN KEY (tenant_id, card_id) REFERENCES cards (tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE card_bodies ENABLE ROW LEVEL SECURITY;
ALTER TABLE card_bodies FORCE ROW LEVEL SECURITY;
CREATE POLICY card_bodies_tenant_policy ON card_bodies
    USING (tenant_id = current_setting('litterbox.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('litterbox.tenant_id', true)::uuid);
