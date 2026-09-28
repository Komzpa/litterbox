CREATE OR REPLACE FUNCTION notify_card_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM pg_notify('cards', OLD.tenant_id::text);
        RETURN OLD;
    END IF;
    PERFORM pg_notify('cards', NEW.tenant_id::text);
    IF TG_OP = 'UPDATE' AND OLD.tenant_id IS DISTINCT FROM NEW.tenant_id THEN
        PERFORM pg_notify('cards', OLD.tenant_id::text);
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER cards_notify_change
AFTER INSERT OR UPDATE OR DELETE ON cards
FOR EACH ROW EXECUTE FUNCTION notify_card_change();
