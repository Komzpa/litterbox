-- Authentication hardening. Values stored in these columns are SHA-256 hashes;
-- raw enrollment invites and device tokens are returned once and never stored.
ALTER TABLE devices ADD COLUMN device_name text NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN platform text NOT NULL DEFAULT '';
ALTER TABLE invites ADD COLUMN expires_at timestamptz;

CREATE UNIQUE INDEX devices_token_hash_global ON devices(token_hash);
CREATE UNIQUE INDEX invites_code_hash_global ON invites(code_hash);

DROP FUNCTION litterbox_redeem_invite(bytea, uuid, bytea);

-- RLS is evaluated for the application role. These public functions are the
-- only pre-tenant lookup paths and expose only the enrollment/auth result.
CREATE OR REPLACE FUNCTION litterbox_redeem_invite(
    p_code_hash bytea,
    p_device_id uuid,
    p_device_name text,
    p_platform text,
    p_token_hash bytea
) RETURNS uuid
    LANGUAGE sql
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $$
    WITH consumed AS (
        UPDATE public.invites
        SET consumed_at = now()
        WHERE code_hash = p_code_hash AND consumed_at IS NULL
          AND (expires_at IS NULL OR expires_at > now())
        RETURNING tenant_id
    ), enrolled AS (
        INSERT INTO public.devices (tenant_id, id, token_hash, device_name, platform)
        SELECT tenant_id, p_device_id, p_token_hash, p_device_name, p_platform FROM consumed
        RETURNING tenant_id
    )
    SELECT tenant_id FROM enrolled;
$$;

CREATE OR REPLACE FUNCTION litterbox_device_by_token(p_token_hash bytea)
RETURNS TABLE (tenant_id uuid, device_id uuid)
    LANGUAGE sql
    STABLE
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $$
    SELECT d.tenant_id, d.id FROM public.devices d
    WHERE d.token_hash = p_token_hash AND d.revoked_at IS NULL;
$$;

REVOKE ALL ON FUNCTION litterbox_redeem_invite(bytea, uuid, text, text, bytea) FROM PUBLIC;
REVOKE ALL ON FUNCTION litterbox_device_by_token(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION litterbox_redeem_invite(bytea, uuid, text, text, bytea) TO litterbox_app;
GRANT EXECUTE ON FUNCTION litterbox_device_by_token(bytea) TO litterbox_app;
