CREATE TABLE reminders (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    title text NOT NULL CHECK (length(btrim(title)) > 0),
    due_at timestamptz NOT NULL,
    recurrence text NOT NULL DEFAULT '' CHECK (recurrence IN ('', 'daily', 'weekly')),
    state text NOT NULL DEFAULT 'scheduled' CHECK (state IN ('scheduled', 'emitted', 'done')),
    card_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, card_id) REFERENCES cards(tenant_id, id)
);
ALTER TABLE reminders ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON reminders USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON reminders TO litterbox_app;
CREATE INDEX reminders_due_idx ON reminders(due_at) WHERE state = 'scheduled';
CREATE INDEX cards_meeting_end_idx ON cards(at) WHERE source = 'meeting' AND state = 'open';

-- Completing a reminder card advances a recurring series transactionally.
CREATE FUNCTION litterbox_repeat_reminder() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r reminders%ROWTYPE;
BEGIN
    IF OLD.state <> 'done' AND NEW.state = 'done' AND NEW.source = 'reminder' THEN
        SELECT * INTO r FROM reminders WHERE tenant_id=NEW.tenant_id AND card_id=NEW.id FOR UPDATE;
        IF FOUND AND r.state='emitted' THEN
            UPDATE reminders SET state='done' WHERE tenant_id=r.tenant_id AND id=r.id;
            IF r.recurrence <> '' THEN
                INSERT INTO reminders(tenant_id,title,due_at,recurrence)
                VALUES(r.tenant_id,r.title,r.due_at + CASE r.recurrence WHEN 'daily' THEN interval '1 day' ELSE interval '7 days' END,r.recurrence);
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER cards_repeat_reminder AFTER UPDATE OF state ON cards FOR EACH ROW EXECUTE FUNCTION litterbox_repeat_reminder();
