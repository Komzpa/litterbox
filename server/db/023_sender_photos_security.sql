-- R67 follow-up: runtime grants and tenant isolation for sender_photos.
-- 022 created the table plus the cards/accounts columns but granted nothing
-- to the runtime role and enabled no RLS, so the photo read/upsert fails for
-- litterbox_app on an otherwise current schema. This file only adds the
-- missing grants/policy, so it applies cleanly to fresh installs (after 022
-- in lexical order) and to databases already migrated through 022.
-- Pattern follows 002_security.sql tenant_isolation: an unset or empty
-- litterbox.tenant_id compares against NULL and hides every row.
GRANT SELECT, INSERT, UPDATE, DELETE ON sender_photos TO litterbox_app;
ALTER TABLE sender_photos ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sender_photos;
CREATE POLICY tenant_isolation ON sender_photos
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
