-- Row-level security and pre-tenant token bootstrap (architecture: "Storage
-- and ownership"). The runtime role connects as non-owner without BYPASSRLS;
-- every request sets transaction-local litterbox.tenant_id from
-- authentication. Only the two SECURITY DEFINER functions below run before a
-- tenant is known.

CREATE ROLE litterbox_app
    LOGIN
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOBYPASSRLS;
-- Password is set by deployment tooling, never stored in the repository (R17).

GRANT SELECT, INSERT, UPDATE, DELETE ON
    tenants, devices, invites, accounts, bundles, cards, messages,
    exclusions, ops, gmail_effects, changes
    TO litterbox_app;

-- Tenant isolation policies. An unset or empty litterbox.tenant_id compares
-- against NULL and therefore hides every row instead of erroring.
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenants
    USING (id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE devices ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON devices
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE invites ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON invites
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON accounts
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE bundles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bundles
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE cards ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON cards
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON messages
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE exclusions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON exclusions
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE ops ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ops
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE gmail_effects ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON gmail_effects
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE changes ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON changes
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

-- Invite redemption, before any tenant context exists. The conditional UPDATE
-- consumes the invite atomically, so a code is single use even under
-- concurrent redemption attempts. Only hashes cross the boundary (R17).
CREATE FUNCTION litterbox_redeem_invite(
    p_code_hash bytea,
    p_device_id uuid,
    p_token_hash bytea
) RETURNS uuid
    LANGUAGE sql
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $$
    WITH consumed AS (
        UPDATE public.invites
        SET consumed_at = now()
        WHERE code_hash = p_code_hash
          AND consumed_at IS NULL
        RETURNING tenant_id
    ), enrolled AS (
        INSERT INTO public.devices (tenant_id, id, token_hash)
        SELECT tenant_id, p_device_id, p_token_hash FROM consumed
    )
    SELECT tenant_id FROM consumed;
$$;

-- Device token lookup for request authentication. Revoked devices and
-- unknown hashes return no row, which the API maps to 401/no data.
CREATE FUNCTION litterbox_device_by_token(
    p_token_hash bytea
) RETURNS TABLE (tenant_id uuid, device_id uuid)
    LANGUAGE sql
    STABLE
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $$
    SELECT d.tenant_id, d.id
    FROM public.devices d
    WHERE d.token_hash = p_token_hash
      AND d.revoked_at IS NULL;
$$;

REVOKE ALL ON FUNCTION litterbox_redeem_invite(bytea, uuid, bytea) FROM PUBLIC;
REVOKE ALL ON FUNCTION litterbox_device_by_token(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION litterbox_redeem_invite(bytea, uuid, bytea) TO litterbox_app;
GRANT EXECUTE ON FUNCTION litterbox_device_by_token(bytea) TO litterbox_app;
