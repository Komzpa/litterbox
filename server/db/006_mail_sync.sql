-- Gmail history polling cursor is per connected account; retain the last
-- successful sync time for operational visibility without resetting history.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS gmail_synced_at timestamptz;
CREATE INDEX IF NOT EXISTS cards_account_open_idx
    ON cards (tenant_id, account_id, state, sort_at DESC);
