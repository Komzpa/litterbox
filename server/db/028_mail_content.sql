-- Keep the original and decoded metadata so rendering never discards the source.
ALTER TABLE messages ADD COLUMN raw_mime bytea NOT NULL DEFAULT ''::bytea;
ALTER TABLE messages ADD COLUMN decode_status text NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN to_recipients jsonb NOT NULL DEFAULT '[]';
ALTER TABLE messages ADD COLUMN cc_recipients jsonb NOT NULL DEFAULT '[]';
ALTER TABLE messages ADD COLUMN content_version integer NOT NULL DEFAULT 0;
ALTER TABLE cards ADD COLUMN mail_content_checked_at timestamptz;
CREATE TABLE message_parts (
 tenant_id uuid NOT NULL,
 message_id uuid NOT NULL,
 id uuid NOT NULL,
 part_index integer NOT NULL,
 name text NOT NULL,
 media_type text NOT NULL,
 content_id text NOT NULL DEFAULT '',
 data bytea NOT NULL,
 PRIMARY KEY (tenant_id,id),
 UNIQUE (tenant_id,message_id,part_index),
 FOREIGN KEY (tenant_id,message_id) REFERENCES messages(tenant_id,id) ON DELETE CASCADE
);
ALTER TABLE message_parts ENABLE ROW LEVEL SECURITY;
ALTER TABLE message_parts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON message_parts
 USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id',true),'')::uuid)
 WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id',true),'')::uuid);
GRANT SELECT,INSERT,UPDATE,DELETE ON message_parts TO litterbox_app;
