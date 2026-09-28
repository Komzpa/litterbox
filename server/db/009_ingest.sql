-- Source-scoped credentials and durable callbacks for non-mail card actions.
CREATE TABLE source_tokens (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source text NOT NULL,
    token_hash bytea NOT NULL,
    callback_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    PRIMARY KEY (tenant_id, source),
    UNIQUE (token_hash)
);

CREATE TABLE source_action_callbacks (
    id bigserial PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    card_id uuid NOT NULL,
    source text NOT NULL,
    callback_url text NOT NULL,
    action jsonb NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, card_id) REFERENCES cards(tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX source_action_callbacks_pending_idx
    ON source_action_callbacks(next_attempt_at, id) WHERE delivered_at IS NULL;
ALTER TABLE source_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON source_tokens
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
ALTER TABLE source_action_callbacks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON source_action_callbacks
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

-- Authentication must discover a tenant before tenant-scoped RLS can be set.
CREATE FUNCTION litterbox_source_by_token(p_hash bytea)
RETURNS TABLE(tenant_id uuid, source text, callback_url text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
    SELECT st.tenant_id, st.source, st.callback_url
    FROM source_tokens st WHERE st.token_hash=p_hash AND st.revoked_at IS NULL
$$;

REVOKE ALL ON FUNCTION litterbox_source_by_token(bytea) FROM PUBLIC;

ALTER TABLE cards ADD COLUMN source_kind text NOT NULL DEFAULT '';
ALTER TABLE cards ADD COLUMN source_actions jsonb NOT NULL DEFAULT '{}'::jsonb;
GRANT SELECT, INSERT, UPDATE, DELETE ON source_tokens, source_action_callbacks TO litterbox_app;
GRANT USAGE, SELECT ON SEQUENCE source_action_callbacks_id_seq TO litterbox_app;
GRANT EXECUTE ON FUNCTION litterbox_source_by_token(bytea) TO litterbox_app;

-- The callback dispatcher is cross-tenant by nature; expose only claim/ack
-- operations, never unrestricted reads of the callback queue.
CREATE FUNCTION litterbox_claim_source_callback()
RETURNS TABLE(callback_id bigint, callback_url text, action jsonb, attempts integer)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_temp AS $$
DECLARE picked source_action_callbacks%ROWTYPE;
BEGIN
    SELECT * INTO picked FROM source_action_callbacks
    WHERE delivered_at IS NULL AND next_attempt_at <= now()
    ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT 1;
    IF NOT FOUND THEN RETURN; END IF;
    UPDATE source_action_callbacks c SET attempts=c.attempts+1,
        next_attempt_at=now()+interval '2 minutes'
    WHERE c.id=picked.id
    RETURNING c.id,c.callback_url,c.action,c.attempts
    INTO callback_id,callback_url,action,attempts;
    RETURN NEXT;
END $$;

CREATE FUNCTION litterbox_complete_source_callback(p_id bigint)
RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = public, pg_temp AS $$
    UPDATE source_action_callbacks SET delivered_at=now(),last_error=NULL
    WHERE id=p_id AND delivered_at IS NULL
$$;

CREATE FUNCTION litterbox_fail_source_callback(p_id bigint,p_error text)
RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = public, pg_temp AS $$
    UPDATE source_action_callbacks SET next_attempt_at=now()+
        (LEAST(1 << LEAST(attempts,10),1024) * interval '1 second'),
        last_error=left(p_error,2000)
    WHERE id=p_id AND delivered_at IS NULL
$$;

REVOKE ALL ON FUNCTION litterbox_claim_source_callback() FROM PUBLIC;
REVOKE ALL ON FUNCTION litterbox_complete_source_callback(bigint) FROM PUBLIC;
REVOKE ALL ON FUNCTION litterbox_fail_source_callback(bigint,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION litterbox_claim_source_callback() TO litterbox_app;
GRANT EXECUTE ON FUNCTION litterbox_complete_source_callback(bigint) TO litterbox_app;
GRANT EXECUTE ON FUNCTION litterbox_fail_source_callback(bigint,text) TO litterbox_app;
