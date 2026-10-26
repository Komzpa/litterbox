-- App-owned, sanitized crash evidence. No public read or update endpoint.
CREATE TABLE crash_reports (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id uuid NOT NULL,
    report_id text NOT NULL CHECK (report_id ~ '^[a-z0-9-]{1,96}$'),
    kind text NOT NULL CHECK (kind IN ('java', 'exit')),
    app_version text NOT NULL,
    uploader_build text NOT NULL,
    body text NOT NULL CHECK (octet_length(body) BETWEEN 1 AND 65536),
    received_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, device_id, report_id),
    FOREIGN KEY (tenant_id, device_id) REFERENCES devices(tenant_id, id)
);

ALTER TABLE crash_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE crash_reports FORCE ROW LEVEL SECURITY;
CREATE POLICY crash_reports_device_isolation ON crash_reports
    USING (
        tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid
        AND device_id = NULLIF(current_setting('litterbox.device_id', true), '')::uuid
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid
        AND device_id = NULLIF(current_setting('litterbox.device_id', true), '')::uuid
    );
GRANT SELECT, INSERT ON crash_reports TO litterbox_app;
