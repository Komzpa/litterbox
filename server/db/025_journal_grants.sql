-- R2/R10: journal inbox creation and MCP access run as the non-owner app role.
-- 012_journal.sql enabled RLS but omitted the underlying table privileges;
-- owner-connected tests therefore missed runtime SQLSTATE 42501 / HTTP 422.
-- GRANT is idempotent and repairs existing installations as well as fresh ones.
GRANT SELECT, INSERT, UPDATE, DELETE ON journal_entries, mcp_tokens TO litterbox_app;
