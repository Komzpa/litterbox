-- Offline sync support (R5, R9-R11, R28 offline). The card note column and
-- live pg_notify trigger are introduced by migrations 004 and 005 landing on
-- a parallel branch; this migration adds the durable change log those two
-- migrations do not populate. to_jsonb(NEW/OLD) avoids hardcoding a column
-- list, so it stays correct as the cards table gains fields from other
-- migrations without a second place needing the same edit.
CREATE OR REPLACE FUNCTION record_card_change() RETURNS trigger
    LANGUAGE plpgsql
AS $$
DECLARE
    v_tenant uuid := COALESCE(NEW.tenant_id, OLD.tenant_id);
    v_seq bigint;
BEGIN
    UPDATE tenants SET next_seq = next_seq + 1 WHERE id = v_tenant RETURNING next_seq INTO v_seq;
    IF v_seq IS NULL THEN
        RAISE EXCEPTION 'record_card_change: unknown tenant %', v_tenant;
    END IF;
    IF TG_OP = 'DELETE' THEN
        INSERT INTO changes (tenant_id, seq, entity, id, value) VALUES (v_tenant, v_seq, 'card', OLD.id, NULL);
        RETURN OLD;
    END IF;
    INSERT INTO changes (tenant_id, seq, entity, id, value) VALUES (v_tenant, v_seq, 'card', NEW.id, to_jsonb(NEW));
    RETURN NEW;
END;
$$;

CREATE TRIGGER cards_record_change
    AFTER INSERT OR UPDATE OR DELETE ON cards
    FOR EACH ROW EXECUTE FUNCTION record_card_change();

CREATE INDEX IF NOT EXISTS changes_tenant_seq_idx ON changes (tenant_id, seq);

-- Development deployments without device-auth middleware leave device attribution null.
ALTER TABLE ops ALTER COLUMN device_id DROP NOT NULL;
