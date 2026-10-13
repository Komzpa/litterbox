-- a2a conversations: a tenant-scoped projection of the canonical a2a journal,
-- one thread per peer. Rows are keyed by journal event_key so re-ingest is
-- idempotent. The a2a bridge authenticates with a source_tokens row whose
-- source is 'a2a' (hash only; provisioned at deploy), so no schema is needed
-- for that token.

CREATE TABLE a2a_threads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    peer_id text NOT NULL CHECK (peer_id <> ''),
    display_name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('token_agent', 'omp_session')),
    availability text NOT NULL DEFAULT 'unknown'
        CHECK (availability IN ('online', 'offline', 'busy', 'unknown')),
    last_text text,
    last_at timestamptz,
    last_seq bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- One peer is one thread.
    UNIQUE (tenant_id, peer_id),
    -- Target for the tenant-carrying message foreign key, so a message can
    -- never reference another tenant's thread.
    UNIQUE (tenant_id, id)
);

-- seq orders every insert and every delivery-state change; clients page with
-- ?after=<seq>.
CREATE SEQUENCE a2a_message_seq;

CREATE TABLE a2a_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    thread_id uuid NOT NULL,
    event_key text NOT NULL CHECK (event_key <> ''),
    seq bigint NOT NULL DEFAULT 0,
    direction text NOT NULL CHECK (direction IN ('in', 'out', 'observed')),
    sender text NOT NULL,
    recipient text NOT NULL,
    text text NOT NULL,
    delivery_state text NOT NULL DEFAULT 'pending'
        CHECK (delivery_state IN ('pending', 'sent', 'delivered', 'not_delivered', 'replied')),
    reason_code text,
    client_msg_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, thread_id) REFERENCES a2a_threads (tenant_id, id) ON DELETE CASCADE,
    -- An observed a2a event lands in both participants' threads, so the
    -- journal event_key is unique per thread.
    UNIQUE (tenant_id, thread_id, event_key)
);
CREATE UNIQUE INDEX a2a_messages_client_msg_idx
    ON a2a_messages (tenant_id, thread_id, client_msg_id) WHERE client_msg_id IS NOT NULL;
CREATE INDEX a2a_messages_thread_seq_idx ON a2a_messages (tenant_id, thread_id, seq);
CREATE INDEX a2a_messages_tenant_seq_idx ON a2a_messages (tenant_id, seq);

-- Assign seq on insert and whenever delivery state, reason or text changes;
-- a no-op upsert leaves seq alone.
CREATE FUNCTION a2a_messages_set_seq() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT'
       OR NEW.delivery_state IS DISTINCT FROM OLD.delivery_state
       OR NEW.reason_code IS DISTINCT FROM OLD.reason_code
       OR NEW.text IS DISTINCT FROM OLD.text THEN
        NEW.seq := nextval('a2a_message_seq');
        NEW.updated_at := now();
    ELSE
        NEW.seq := OLD.seq;
    END IF;
    RETURN NEW;
END;
$$;

-- Keep the thread summary current and wake SSE listeners. The payload is the
-- tenant id only, never message text.
CREATE FUNCTION a2a_messages_after_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' OR NEW.seq IS DISTINCT FROM OLD.seq THEN
        UPDATE a2a_threads
        SET last_text = NEW.text, last_at = NEW.updated_at, last_seq = NEW.seq, updated_at = now()
        WHERE tenant_id = NEW.tenant_id AND id = NEW.thread_id AND last_seq <= NEW.seq;
        PERFORM pg_notify('a2a', NEW.tenant_id::text);
    END IF;
    RETURN NULL;
END;
$$;

CREATE FUNCTION a2a_threads_after_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('a2a', NEW.tenant_id::text);
    RETURN NULL;
END;
$$;

CREATE TRIGGER a2a_messages_seq BEFORE INSERT OR UPDATE ON a2a_messages
FOR EACH ROW EXECUTE FUNCTION a2a_messages_set_seq();
CREATE TRIGGER a2a_messages_notify AFTER INSERT OR UPDATE ON a2a_messages
FOR EACH ROW EXECUTE FUNCTION a2a_messages_after_change();
CREATE TRIGGER a2a_threads_notify AFTER INSERT OR UPDATE ON a2a_threads
FOR EACH ROW EXECUTE FUNCTION a2a_threads_after_change();

ALTER TABLE a2a_threads ENABLE ROW LEVEL SECURITY;
ALTER TABLE a2a_threads FORCE ROW LEVEL SECURITY;
CREATE POLICY a2a_threads_tenant_policy ON a2a_threads
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

ALTER TABLE a2a_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE a2a_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY a2a_messages_tenant_policy ON a2a_messages
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON a2a_threads, a2a_messages TO litterbox_app;
GRANT USAGE, SELECT ON SEQUENCE a2a_message_seq TO litterbox_app;
