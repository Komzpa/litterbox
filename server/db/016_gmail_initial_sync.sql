ALTER TABLE accounts
    ADD COLUMN gmail_initial_sync boolean NOT NULL DEFAULT false,
    ADD COLUMN gmail_initial_page_token text,
    ADD COLUMN gmail_initial_history_id text;

CREATE TABLE gmail_initial_sync_threads (
    tenant_id uuid NOT NULL,
    account_id uuid NOT NULL,
    thread_id text NOT NULL,
    PRIMARY KEY (tenant_id, account_id, thread_id),
    FOREIGN KEY (tenant_id, account_id)
        REFERENCES accounts (tenant_id, id) ON DELETE CASCADE
);
ALTER TABLE gmail_initial_sync_threads ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON gmail_initial_sync_threads
    USING (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON gmail_initial_sync_threads TO litterbox_app;
